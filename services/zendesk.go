package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/nukosuke/go-zendesk/zendesk"
)

type ZendeskClient struct {
	Subdomain string
	Email     string
	APIToken  string
	client    *http.Client
}

type SLAPolicyMetric struct {
	BreachAt time.Time `json:"breach_at"`
	Stage    string    `json:"stage"`
	Metric   string    `json:"metric"`
	Hours    int       `json:"hours"`
	Minutes  int       `json:"minutes"`
	Days     int       `json:"days"`
}

type SLAInfo struct {
	PolicyMetrics []SLAPolicyMetric `json:"policy_metrics"`
}

type SLAResponse struct {
	SLAPolicies []struct {
		TicketID      int64             `json:"ticket_id"`
		PolicyMetrics []SLAPolicyMetric `json:"policy_metrics"`
	} `json:"slas"`
}

func NewZendeskClient(subdomain, email, apiToken string) *ZendeskClient {
	return &ZendeskClient{
		Subdomain: subdomain,
		Email:     email,
		APIToken:  apiToken,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}
func (zc *ZendeskClient) SearchTicketsWithActiveSLA() ([]zendesk.Ticket, map[int64]SLAInfo, error) {
	var allTickets []zendesk.Ticket
	slaData := make(map[int64]SLAInfo)

	query := "type:ticket status<pending"
	params := url.Values{}
	params.Set("query", query)
	params.Set("include", "tickets(slas)") // Correct sideloading parameter

	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/search.json?%s", zc.Subdomain, params.Encode())

	for endpoint != "" {
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.SetBasicAuth(zc.Email+"/token", zc.APIToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := zc.client.Do(req.WithContext(context.Background()))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to perform request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, nil, fmt.Errorf("failed to search tickets: received status %s", resp.Status)
		}

		var result struct {
			Results []struct {
				zendesk.Ticket
				SLAMetrics struct {
					PolicyMetrics []SLAPolicyMetric `json:"policy_metrics"`
				} `json:"slas"`
			} `json:"results"`
			NextPage string `json:"next_page"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, nil, fmt.Errorf("failed to parse ticket search response: %w", err)
		}

		// Process tickets and extract SLA data
		for _, ticketResult := range result.Results {
			allTickets = append(allTickets, ticketResult.Ticket)

			// Only store metrics with active SLAs
			var relevantMetrics []SLAPolicyMetric
			for _, metric := range ticketResult.SLAMetrics.PolicyMetrics {
				if metric.Stage == "active" {
					relevantMetrics = append(relevantMetrics, metric)
				}
			}
			if len(relevantMetrics) > 0 {
				slaData[ticketResult.Ticket.ID] = SLAInfo{
					PolicyMetrics: relevantMetrics,
				}
			}
		}

		endpoint = result.NextPage
	}

	return allTickets, slaData, nil
}

func (zc *ZendeskClient) SearchNewOrUpdatedTickets(since time.Time) ([]zendesk.Ticket, error) {
	var allTickets []zendesk.Ticket

	query := fmt.Sprintf("type:ticket updated>%s", since.Format(time.RFC3339))
	params := url.Values{}
	params.Set("query", query)

	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/search.json?%s", zc.Subdomain, params.Encode())

	for endpoint != "" {

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.SetBasicAuth(zc.Email+"/token", zc.APIToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := zc.client.Do(req.WithContext(context.Background()))
		if err != nil {
			return nil, fmt.Errorf("failed to perform request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to search tickets: received status %s", resp.Status)
		}

		var result struct {
			Results  []zendesk.Ticket `json:"results"`
			NextPage string           `json:"next_page"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to parse ticket search response: %w", err)
		}

		allTickets = append(allTickets, result.Results...)

		endpoint = result.NextPage
	}

	return allTickets, nil
}

func StartZendeskPolling(sseServer *middlewares.SSEServer) {
	var lastPollTime = time.Now().Add(-5 * time.Minute) // Start 5 minutes before now

	for {
		subdomain, email, apiKey, err := getZendeskConfig()
		if err != nil {
			middlewares.AddGlobalNotification(sseServer, "Zendesk Configuration Error", fmt.Sprintf("Error fetching Zendesk configuration: %v", err), "danger")
			time.Sleep(5 * time.Minute)
			continue
		}

		zendeskClient := NewZendeskClient(subdomain, email, apiKey)
		middlewares.AddGlobalNotification(sseServer, "Refreshing Zendesk tickets", "Requesting tickets from Zendesk", "info")
		log.Println("Requesting tickets from Zendesk...")
		slaTickets, slaData, err := zendeskClient.SearchTicketsWithActiveSLA()
		if err != nil {
			middlewares.AddGlobalNotification(sseServer, "Zendesk Connectivity Error", fmt.Sprintf("Error searching SLA tickets: %v", err), "warning")
			time.Sleep(5 * time.Minute)
			continue
		}
		log.Println("Fetched", len(slaTickets), "SLA tickets")
		newUpdatedTickets, err := zendeskClient.SearchNewOrUpdatedTickets(lastPollTime)
		if err != nil {
			middlewares.AddGlobalNotification(sseServer, "Zendesk Connectivity Error", fmt.Sprintf("Error searching new/updated tickets: %v", err), "warning")
			time.Sleep(5 * time.Minute)
			continue
		}
		log.Println("Fetched", len(newUpdatedTickets), "new/updated tickets")
		allTickets := append(slaTickets, newUpdatedTickets...)
		if len(allTickets) == 0 {
			log.Println("No tickets to process")
		} else {
			processTickets(allTickets, slaData, sseServer)
		}

		lastPollTime = time.Now()

		time.Sleep(5 * time.Minute)
	}
}

func getZendeskConfig() (string, string, string, error) {
	apiKey, err := models.GetConfiguration("zendesk_api_key")
	if err != nil || apiKey == "" {
		return "", "", "", fmt.Errorf("zendesk API key not configured")
	}

	email, err := models.GetConfiguration("zendesk_email")
	if err != nil || email == "" {
		return "", "", "", fmt.Errorf("zendesk email not configured")
	}

	subdomain, err := models.GetConfiguration("zendesk_subdomain")
	if err != nil || subdomain == "" {
		return "", "", "", fmt.Errorf("zendesk subdomain not configured")
	}

	return subdomain, email, apiKey, nil
}
func processTickets(tickets []zendesk.Ticket, slaData map[int64]SLAInfo, sseServer *middlewares.SSEServer) {

	for _, ticket := range tickets {
		userAlerts, err := models.GetAllTagAlerts()
		if err != nil {
			fmt.Println("Error fetching user alerts:", err)
			continue
		}

		for _, alert := range userAlerts {
			if tagMatches(alert.Tag, ticket.Tags) {
				switch alert.AlertType {
				case "new_ticket":
					if isNewTicket(ticket) {
						logAlert(alert, ticket, "New Ticket Alert")
					}
				case "ticket_update":
					if isUpdatedTicket(ticket) {
						logAlert(alert, ticket, "Ticket Update Alert")
					}
				case "sla_deadline":
					// Fetch SLA metrics from the ticket
					if slaInfo, ok := slaData[ticket.ID]; ok {
						if label, matches := slaConditionMatches(slaInfo.PolicyMetrics); matches {
							logAlert(alert, ticket, label)
						}
					}
				}
			}
		}
	}
	middlewares.AddGlobalNotification(sseServer, "Ticket processing complete", fmt.Sprintf("Processed %v tickets...", len(tickets)), "success")
}

func tagMatches(alertTag string, ticketTags []string) bool {
	for _, tag := range ticketTags {
		if tag == alertTag {
			return true
		}
	}
	return false
}

// Determine if a ticket is new since the last poll
func isNewTicket(ticket zendesk.Ticket) bool {
	// Replace with actual last poll time
	lastPollTime := time.Now().Add(-5 * time.Minute)
	return ticket.CreatedAt.After(lastPollTime)
}

// Determine if a ticket was updated since the last poll
func isUpdatedTicket(ticket zendesk.Ticket) bool {
	// Replace with actual last poll time
	lastPollTime := time.Now().Add(-5 * time.Minute)
	return ticket.UpdatedAt.After(lastPollTime)
}

func slaConditionMatches(slaMetrics []SLAPolicyMetric) (string, bool) {

	for _, metric := range slaMetrics {
		// Check if the SLA metric is still active
		if metric.Stage == "active" {
			// Calculate the time remaining until the SLA breach
			timeRemaining := time.Until(metric.BreachAt)
			// Check if the SLA has been breached (negative time remaining)
			if timeRemaining < 0 {
				return "SLA Breached", true
			}

			// Check if the SLA is within the alert threshold
			switch {
			case timeRemaining <= 3*time.Hour && timeRemaining > 2*time.Hour:
				return "Less than 3 hours remaining", true
			case timeRemaining <= 2*time.Hour && timeRemaining > 1*time.Hour:
				return "Less than 2 hours remaining", true
			case timeRemaining <= 1*time.Hour && timeRemaining > 30*time.Minute:
				return "Less than 1 hour remaining", true
			case timeRemaining <= 30*time.Minute && timeRemaining > 15*time.Minute:
				return "Less than 30 minutes remaining", true
			case timeRemaining <= 15*time.Minute:
				return "Less than 15 minutes remaining", true
			}
		}
	}

	return "", false
}

// Log the alert for now
func logAlert(alert models.TagAlert, ticket zendesk.Ticket, alertType string) {
	log.Printf("ALERT: [%s] Ticket #%d (Title: '%s') triggered an alert for tag '%s'\n",
		alertType, ticket.ID, ticket.Subject, alert.Tag)
}

type SLAAlertTracker struct {
	Thresholds map[int64]map[string]bool // Map of ticket ID to threshold alerts
}

func NewSLAAlertTracker() *SLAAlertTracker {
	return &SLAAlertTracker{
		Thresholds: make(map[int64]map[string]bool),
	}
}

func (tracker *SLAAlertTracker) HasAlerted(ticketID int64, threshold string) bool {
	if tracker.Thresholds[ticketID] == nil {
		tracker.Thresholds[ticketID] = make(map[string]bool)
	}
	return tracker.Thresholds[ticketID][threshold]
}

func (tracker *SLAAlertTracker) SetAlerted(ticketID int64, threshold string) {
	tracker.Thresholds[ticketID][threshold] = true
}
