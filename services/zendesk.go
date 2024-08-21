package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/nukosuke/go-zendesk/zendesk"
)

const (
	AlertTypeNewTicket    = "new_ticket"
	AlertTypeTicketUpdate = "ticket_update"
	AlertTypeSLABreach    = "sla_breach"
)

type ZendeskClient struct {
	client    *zendesk.Client
	Subdomain string
	Email     string
	APIToken  string
}

// SLAPolicyMetric represents SLA metrics for a ticket
type SLAPolicyMetric struct {
	BreachAt time.Time `json:"breach_at"`
	Stage    string    `json:"stage"`
	Metric   string    `json:"metric"`
	Hours    int       `json:"hours"`
	Minutes  int       `json:"minutes"`
	Days     int       `json:"days"`
}

// SLAInfo holds SLA metrics for a ticket
type SLAInfo struct {
	PolicyMetrics []SLAPolicyMetric `json:"policy_metrics"`
}

// SatisfactionRating represents CSAT data from Zendesk.
type SatisfactionRating struct {
	ID        int64  `json:"id"`
	Score     string `json:"score"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
	TicketID  int64  `json:"ticket_id"`
}

// User represents a Zendesk user (expand with needed fields).
type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Organization represents a Zendesk organization (expand with needed fields).
type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// NewZendeskClient initializes a new ZendeskClient
func NewZendeskClient(subdomain, email, apiToken string) *ZendeskClient {
	client, err := zendesk.NewClient(nil)
	if err != nil {
		fmt.Printf("Failed to create Zendesk client: %v\n", err)
	}
	client.SetSubdomain(subdomain)
	client.SetCredential(zendesk.NewAPITokenCredential(email, apiToken))

	return &ZendeskClient{
		client:    client,
		Subdomain: subdomain,
		Email:     email,
		APIToken:  apiToken,
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

// StartZendeskPolling handles periodic polling of tickets from Zendesk.
func StartZendeskPolling(sseServer *middlewares.SSEServer, slackService *SlackService) {
	var lastPollTime = time.Now().Add(-5 * time.Minute) // Start 5 minutes before now
	broadcastStatusUpdates(sseServer, "zendesk", "connected", "")
	for {
		subdomain, email, apiKey, err := getZendeskConfig()
		if err != nil {
			middlewares.AddGlobalNotification(sseServer, "Zendesk Configuration Error", fmt.Sprintf("Error fetching Zendesk configuration: %v", err), "danger")
			broadcastStatusUpdates(sseServer, "zendesk", "error", "Error fetching Zendesk configuration")
			time.Sleep(5 * time.Minute)
			continue
		}

		zendeskClient := NewZendeskClient(subdomain, email, apiKey)
		middlewares.AddGlobalNotification(sseServer, "Refreshing Zendesk tickets", "Requesting tickets from Zendesk", "info")
		log.Println("Requesting tickets from Zendesk...")
		slaTickets, slaData, err := zendeskClient.SearchTicketsWithActiveSLA()
		if err != nil {
			middlewares.AddGlobalNotification(sseServer, "Zendesk Connectivity Error", fmt.Sprintf("Error searching SLA tickets: %v", err), "warning")
			broadcastStatusUpdates(sseServer, "zendesk", "error", "Error searching SLA tickets")
			time.Sleep(5 * time.Minute)
			continue
		}
		log.Println("Fetched", len(slaTickets), "SLA tickets")
		newUpdatedTickets, err := zendeskClient.SearchNewOrUpdatedTickets(lastPollTime)
		if err != nil {
			middlewares.AddGlobalNotification(sseServer, "Zendesk Connectivity Error", fmt.Sprintf("Error searching new/updated tickets: %v", err), "warning")
			broadcastStatusUpdates(sseServer, "zendesk", "error", "Error searching new/updated tickets")
			time.Sleep(5 * time.Minute)
			continue
		}
		log.Println("Fetched", len(newUpdatedTickets), "new/updated tickets")
		allTickets := append(slaTickets, newUpdatedTickets...)
		if len(allTickets) == 0 {
			log.Println("No tickets to process")
		} else {
			processTickets(allTickets, slaData, sseServer, slackService)
		}

		lastPollTime = time.Now()

		time.Sleep(5 * time.Minute)
	}
}

func processTickets(tickets []zendesk.Ticket, slaData map[int64]SLAInfo, sseServer *middlewares.SSEServer, slackService *SlackService) {
	for _, ticket := range tickets {
		userAlerts, err := models.GetAllTagAlerts()
		if err != nil {
			fmt.Println("Error fetching user alerts:", err)
			continue
		}

		for _, alert := range userAlerts {
			if tagMatches(alert.Tag, ticket.Tags) {
				var sendAlert bool
				var slaLabel string

				switch alert.AlertType {
				case "new_ticket":
					sendAlert = isNewTicket(ticket)
				case "ticket_update":
					sendAlert = isUpdatedTicket(ticket)
				case "sla_deadline":
					if slaInfo, ok := slaData[ticket.ID]; ok {
						if label, matches := slaConditionMatches(slaInfo.PolicyMetrics); matches {
							sendAlert = true
							slaLabel = label // Set the SLA label for the alert
						}
					}
				}

				if sendAlert {
					logAlert(alert, ticket, alert.AlertType)

					// Send the Slack message using the refactored function
					slaInfo, _ := slaData[ticket.ID]
					err := slackService.SendSlackMessage(alert.SlackChannelID, alert.AlertType, slaLabel, ticket, &slaInfo, alert.Tag)
					if err != nil {
						fmt.Printf("Failed to send Slack message for Ticket #%d: %v\n", ticket.ID, err)
					}
				}
			}
		}
	}
	middlewares.AddGlobalNotification(sseServer, "Ticket processing complete", fmt.Sprintf("Processed %v tickets...", len(tickets)), "success")
}

// slaConditionMatches checks if the SLA condition matches the threshold for sending alerts.
func slaConditionMatches(slaMetrics []SLAPolicyMetric) (string, bool) {
	for _, metric := range slaMetrics {
		if metric.Stage == "active" {
			timeRemaining := time.Until(metric.BreachAt)
			if timeRemaining < 0 {
				return "SLA Breached", true
			}
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

// Helper function to check if a tag matches
func tagMatches(alertTag string, ticketTags []string) bool {
	for _, tag := range ticketTags {
		if tag == alertTag {
			return true
		}
	}
	return false
}

// Helper function to determine if a ticket is new
func isNewTicket(ticket zendesk.Ticket) bool {
	lastPollTime := time.Now().Add(-5 * time.Minute)
	return ticket.CreatedAt.After(lastPollTime)
}

// Helper function to determine if a ticket is updated
func isUpdatedTicket(ticket zendesk.Ticket) bool {
	lastPollTime := time.Now().Add(-5 * time.Minute)
	return ticket.UpdatedAt.After(lastPollTime)
}

// Log the alert for now
func logAlert(alert models.TagAlert, ticket zendesk.Ticket, alertType string) {
	log.Printf("ALERT: [%s] Ticket #%d (Title: '%s') triggered an alert for tag '%s'\n",
		alertType, ticket.ID, ticket.Subject, alert.Tag)
}

// SLAAlertTracker keeps track of which alerts have already been sent.
type SLAAlertTracker struct {
	Thresholds map[int64]map[string]bool // Map of ticket ID to threshold alerts
}

// NewSLAAlertTracker initializes an SLAAlertTracker
func NewSLAAlertTracker() *SLAAlertTracker {
	return &SLAAlertTracker{
		Thresholds: make(map[int64]map[string]bool),
	}
}

// HasAlerted checks if a ticket has already triggered an alert at the given threshold
func (tracker *SLAAlertTracker) HasAlerted(ticketID int64, threshold string) bool {
	if tracker.Thresholds[ticketID] == nil {
		tracker.Thresholds[ticketID] = make(map[string]bool)
	}
	return tracker.Thresholds[ticketID][threshold]
}

// SetAlerted marks that a ticket has triggered an alert at the given threshold
func (tracker *SLAAlertTracker) SetAlerted(ticketID int64, threshold string) {
	tracker.Thresholds[ticketID][threshold] = true
}

// GetUserByID retrieves a user from Zendesk based on their ID.
func (zc *ZendeskClient) GetRequesterByID(userID int64) (*User, error) {
	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/users/%d.json", zc.Subdomain, userID)

	req, err := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(zc.Email+"/token", zc.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zendesk returned status %s", resp.Status)
	}

	var result struct {
		User User `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.User, nil
}

// GetOrganizationByID retrieves an organization from Zendesk based on its ID.
func (zc *ZendeskClient) GetOrganizationByID(organizationID int64) (*Organization, error) {
	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/organizations/%d.json", zc.Subdomain, organizationID)

	req, err := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(zc.Email+"/token", zc.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zendesk returned status %s", resp.Status)
	}

	var result struct {
		Organization Organization `json:"organization"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Organization, nil
}

func getSLALabel(ticket zendesk.Ticket, slaData map[int64]SLAInfo) string {
	slaInfo, exists := slaData[ticket.ID]
	if !exists || len(slaInfo.PolicyMetrics) == 0 {
		return "No SLA"
	}

	// Assuming the first SLA policy metric is relevant for simplicity.
	metric := slaInfo.PolicyMetrics[0]
	return fmt.Sprintf("%s - %d hours %d minutes remaining", metric.Metric, metric.Hours, metric.Minutes)
}
