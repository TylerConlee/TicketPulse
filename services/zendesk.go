package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/nukosuke/go-zendesk/zendesk"
)

const (
	AlertTypeNewTicket    = "new_ticket"
	AlertTypeTicketUpdate = "ticket_update"
	AlertTypeSLABreach    = "sla_deadline"
)

type ZendeskClient struct {
	client    *zendesk.Client
	Subdomain string
	Email     string
	APIToken  string
	DB        db.Database
}

// SLAPolicyMetric represents SLA metrics for a ticket.
type SLAPolicyMetric struct {
	BreachAt time.Time `json:"breach_at"`
	Stage    string    `json:"stage"`
	Metric   string    `json:"metric"`
	Hours    int       `json:"hours"`
	Minutes  int       `json:"minutes"`
	Days     int       `json:"days"`
}

// SLAInfo holds SLA metrics for a ticket.
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

// User represents a Zendesk user.
type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Organization represents a Zendesk organization.
type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

var lastPollTime = time.Now().Add(-5 * time.Minute) // Start 5 minutes before now

// NewZendeskClient initializes a new ZendeskClient using configuration from the database.
func NewZendeskClient(db db.Database) (*ZendeskClient, error) {
	subdomain, err := models.GetConfiguration(db, "zendesk_subdomain")
	if err != nil || subdomain == "" {
		return nil, fmt.Errorf("failed to retrieve Zendesk subdomain")
	}
	email, err := models.GetConfiguration(db, "zendesk_email")
	if err != nil || email == "" {
		return nil, fmt.Errorf("failed to retrieve Zendesk email")
	}
	apiToken, err := models.GetConfiguration(db, "zendesk_api_key")
	if err != nil || apiToken == "" {
		return nil, fmt.Errorf("failed to retrieve Zendesk API token")
	}

	client, err := zendesk.NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Zendesk client: %v", err)
	}
	client.SetSubdomain(subdomain)
	client.SetCredential(zendesk.NewAPITokenCredential(email, apiToken))

	return &ZendeskClient{
		client:    client,
		Subdomain: subdomain,
		Email:     email,
		APIToken:  apiToken,
		DB:        db,
	}, nil
}

// StartZendeskPolling handles periodic polling of tickets from Zendesk.
func StartZendeskPolling(ctx context.Context, db db.Database, sseServer *middlewares.SSEServer, slackService *SlackService) {

	broadcastStatusUpdates(sseServer, "zendesk", "connected", "")

	for {
		zendeskClient, err := NewZendeskClient(db)
		if err != nil {
			middlewares.AddGlobalNotification(sseServer, "Zendesk Configuration Error", fmt.Sprintf("Error fetching Zendesk configuration: %v", err), "danger")
			broadcastStatusUpdates(sseServer, "zendesk", "error", "Error fetching Zendesk configuration")
			time.Sleep(5 * time.Minute)
			continue
		}

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
			processTickets(ctx, db, allTickets, slaData, sseServer, slackService, zendeskClient)
		}

		lastPollTime = time.Now()
		time.Sleep(5 * time.Minute)
	}
}

func processTickets(ctx context.Context, db db.Database, tickets []zendesk.Ticket, slaData map[int64]SLAInfo, sseServer *middlewares.SSEServer, slackService *SlackService, zc *ZendeskClient) {

	userAlerts, err := models.GetAllTagAlerts(db)
	log.Printf("Processing %d tickets...\n", len(tickets))
	for _, ticket := range tickets {

		if err != nil {
			fmt.Println("Error fetching user alerts:", err)
			continue
		}

		for _, alert := range userAlerts {
			if tagMatches(alert.Tag, ticket.Tags) {
				var sendAlert bool
				var slaLabel string
				var msgColor string  // Slack color for SLA alerts
				msgColor = "#3498DB" // Default color for SLA alerts

				switch alert.AlertType {
				case AlertTypeNewTicket:
					sendAlert = isNewTicket(ticket)
				case AlertTypeTicketUpdate:
					if isUpdatedTicket(ticket) {
						lastPublicCommentTime, err := zc.GetLastPublicCommentTime(ticket.ID)
						if err != nil {
							log.Printf("Skipping ticket %d: %v", ticket.ID, err)
							continue
						}

						if lastPublicCommentTime.After(lastPollTime) {
							sendAlert = true
						}
					}

				case AlertTypeSLABreach:

					if slaInfo, ok := slaData[ticket.ID]; ok {
						if label, color, matches := slaConditionMatches(slaInfo.PolicyMetrics); matches {
							msgColor = color
							// Correct the argument types and pass *sql.DB
							existingAlert, err := models.GetSLAAlertCache(ctx, db, int(alert.User.ID), int(ticket.ID), alert.AlertType)
							if err == nil && existingAlert.BreachAt != slaInfo.PolicyMetrics[0].BreachAt && existingAlert.Label != label {
								models.ClearSLAAlertCache(ctx, db, existingAlert.ID)
							} else if err == nil {
								continue
							}

							sendAlert = true
							slaLabel = label

							// Log the SLA alert
							logEntry := models.SLAAlertCache{
								UserID:    int64(alert.User.ID), // Use int type
								TicketID:  int64(ticket.ID),     // Use int type
								AlertType: alert.AlertType,
								BreachAt:  slaInfo.PolicyMetrics[0].BreachAt,
								Label:     slaLabel,
							}
							if err := models.CreateSLAAlertCache(ctx, db, logEntry); err != nil {
								fmt.Printf("Failed to log SLA alert for Ticket #%d: %v\n", ticket.ID, err)
							}
						}
					}
				}

				if sendAlert {
					logAlert(alert, ticket, alert.AlertType)
					timestamp := time.Now().Format("2006-01-02 15:04:05")
					alertLog := models.AlertLog{
						UserID:    int64(alert.User.ID), // Use int type
						TicketID:  int64(ticket.ID),     // Use int type
						Tag:       alert.Tag,
						AlertType: alert.AlertType,
						Timestamp: timestamp,
					}
					models.CreateAlertLog(ctx, db, alertLog)
					slaInfo := slaData[ticket.ID]
					err := slackService.SendSlackMessage(alert.SlackChannelID, alert.AlertType, slaLabel, ticket, &slaInfo, alert.Tag, msgColor)
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
func slaConditionMatches(slaMetrics []SLAPolicyMetric) (string, string, bool) {
	for _, metric := range slaMetrics {
		if metric.Stage == "active" {
			timeRemaining := time.Until(metric.BreachAt)
			if timeRemaining < 0 {
				return "SLA Breached", "#FF0000", true // Red for breached SLA
			}
			switch {
			case timeRemaining <= 3*time.Hour && timeRemaining > 2*time.Hour:
				return "Less than 3 hours remaining", "#3498DB", true // Blue
			case timeRemaining <= 2*time.Hour && timeRemaining > 1*time.Hour:
				return "Less than 2 hours remaining", "#F1C40F", true // Yellow
			case timeRemaining <= 1*time.Hour && timeRemaining > 30*time.Minute:
				return "Less than 1 hour remaining", "#FFA500", true // Orange
			case timeRemaining <= 30*time.Minute && timeRemaining > 15*time.Minute:
				return "Less than 30 minutes remaining", "#FF8C00", true // Darker Orange
			case timeRemaining <= 15*time.Minute:
				return "Less than 15 minutes remaining", "#FF0000", true // Red
			}
		}
	}
	return "", "", false
}

// Helper function to check if a tag matches.
func tagMatches(alertTag string, ticketTags []string) bool {
	for _, tag := range ticketTags {
		if tag == alertTag {
			return true
		}
	}
	return false
}

// Helper function to determine if a ticket is new.
func isNewTicket(ticket zendesk.Ticket) bool {
	lastPollTime := time.Now().Add(-5 * time.Minute)
	return ticket.CreatedAt.After(lastPollTime)
}

// Helper function to determine if a ticket is updated.
func isUpdatedTicket(ticket zendesk.Ticket) bool {
	lastPollTime := time.Now().Add(-5 * time.Minute)
	return ticket.UpdatedAt.After(lastPollTime)
}

// Log the alert.
func logAlert(alert models.TagAlert, ticket zendesk.Ticket, alertType string) {
	log.Printf("ALERT: [%s] Ticket #%d (Title: '%s') triggered an alert for tag '%s'\n",
		alertType, ticket.ID, ticket.Subject, alert.Tag)
}

// GetRequesterByID retrieves a user from Zendesk based on their ID.
func (zc *ZendeskClient) GetRequesterByID(userID int64) (*User, error) {
	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/users/%d.json", zc.Subdomain, userID)

	req, err := http.NewRequest("GET", endpoint, nil)
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

	req, err := http.NewRequest("GET", endpoint, nil)
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

// GetLastPublicCommentTime retrieves the timestamp of the last public comment on a ticket.
// GetLastPublicCommentTime retrieves the timestamp of the last public comment on a ticket.
func (zc *ZendeskClient) GetLastPublicCommentTime(ticketID int64) (time.Time, error) {
	var lastPublicCommentTime time.Time
	ops := zendesk.NewPaginationOptions()
	ops.PageSize = 10
	ops.Id = ticketID
	it := zc.client.GetTicketCommentsIterator(context.Background(), ops)

	for it.HasMore() {
		comments, err := it.GetNext()
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to retrieve comments for ticket %d: %v", ticketID, err)
		}

		// Ensure we have comments before accessing the last element
		if len(comments) == 0 {
			continue
		}

		lastComment := comments[len(comments)-1] // Get the last comment

		// Check if the comment is public by safely dereferencing the *bool
		if lastComment.Public != nil && *lastComment.Public {
			commentTime := lastComment.CreatedAt
			if commentTime.After(lastPublicCommentTime) {
				lastPublicCommentTime = commentTime
			}
		}
	}

	if lastPublicCommentTime.IsZero() {
		return time.Time{}, fmt.Errorf("no public comments found for ticket %d", ticketID)
	}

	return lastPublicCommentTime, nil
}

func getSLALabel(ticket zendesk.Ticket, slaData map[int64]SLAInfo) string {
	slaInfo, exists := slaData[ticket.ID]
	if !exists || len(slaInfo.PolicyMetrics) == 0 {
		return "No SLA"
	}

	metric := slaInfo.PolicyMetrics[0]
	return fmt.Sprintf("%s - %d hours %d minutes remaining", metric.Metric, metric.Hours, metric.Minutes)
}
