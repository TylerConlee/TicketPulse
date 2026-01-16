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
	AlertTypeNewTicket     = "new_ticket"
	AlertTypeTicketUpdate  = "ticket_update"
	AlertTypeSLABreach     = "sla_deadline" // Deprecated, kept for backward compatibility
	AlertTypeSLAReply      = "sla_reply"
	AlertTypeSLAResolution = "sla_resolution"
)

const (
	MetricTypeReply      = "reply_time"
	MetricTypeResolution = "resolution_time"
)

type ZendeskClient struct {
	client     *zendesk.Client
	httpClient *http.Client // Injectable HTTP client for testing
	Subdomain  string
	Email      string
	APIToken   string
	DB         db.Database
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
	Role string `json:"role"`
}

// Organization represents a Zendesk organization.
type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// PollingState tracks the state of Zendesk polling
type PollingState struct {
	lastPollTime time.Time
}

// NewPollingState creates a new polling state
func NewPollingState() *PollingState {
	return &PollingState{
		lastPollTime: time.Now().Add(-5 * time.Minute), // Start 5 minutes before now
	}
}

// NewZendeskClient initializes a new ZendeskClient using configuration from the database.
func NewZendeskClient(db db.Database) (*ZendeskClient, error) {
	return NewZendeskClientWithHTTP(db, &http.Client{})
}

// NewZendeskClientWithHTTP initializes a new ZendeskClient with a custom HTTP client.
// This is useful for testing with httptest.Server.
func NewZendeskClientWithHTTP(db db.Database, httpClient *http.Client) (*ZendeskClient, error) {
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
		client:     client,
		httpClient: httpClient,
		Subdomain:  subdomain,
		Email:      email,
		APIToken:   apiToken,
		DB:         db,
	}, nil
}

// NewZendeskClientForTesting creates a ZendeskClient for testing without database configuration.
// It accepts the credentials directly and uses the provided HTTP client.
func NewZendeskClientForTesting(subdomain, email, apiToken string, httpClient *http.Client) *ZendeskClient {
	return &ZendeskClient{
		client:     nil, // Not used in custom API calls
		httpClient: httpClient,
		Subdomain:  subdomain,
		Email:      email,
		APIToken:   apiToken,
		DB:         nil,
	}
}

// StartZendeskPolling handles periodic polling of tickets from Zendesk.
func StartZendeskPolling(ctx context.Context, db db.Database, sseServer *middlewares.SSEServer, slackService *SlackService) {
	pollingState := NewPollingState()

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

		newUpdatedTickets, err := zendeskClient.SearchNewOrUpdatedTickets(pollingState.lastPollTime)
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
			processTickets(ctx, db, allTickets, slaData, sseServer, slackService, zendeskClient, pollingState)
		}

		// Clear expired SLA cache entries (breached SLAs)
		if err := models.ClearExpiredSLAAlertCache(ctx, db); err != nil {
			log.Printf("Failed to clear expired SLA cache: %v", err)
		}

		pollingState.lastPollTime = time.Now()
		time.Sleep(5 * time.Minute)
	}
}

func processTickets(ctx context.Context, db db.Database, tickets []zendesk.Ticket, slaData map[int64]SLAInfo, sseServer *middlewares.SSEServer, slackService *SlackService, zc *ZendeskClient, pollingState *PollingState) {

	userAlerts, err := models.GetAllTagAlerts(db)
	if err != nil {
		log.Printf("Error fetching user alerts: %v", err)
		return
	}
	log.Printf("Processing %d tickets...\n", len(tickets))
	processedTickets := make(map[int64]bool)

	// Pre-build tag index for faster lookups
	tagIndex := make(map[string][]models.TagAlert)
	for _, alert := range userAlerts {
		tagIndex[alert.Tag] = append(tagIndex[alert.Tag], alert)
	}

	for _, ticket := range tickets {
		if processedTickets[ticket.ID] {
			continue
		}

		// Find matching alerts using tag index
		matchedAlerts := make([]models.TagAlert, 0)
		for _, ticketTag := range ticket.Tags {
			if alerts, exists := tagIndex[ticketTag]; exists {
				matchedAlerts = append(matchedAlerts, alerts...)
			}
		}

		for _, alert := range matchedAlerts {
			var sendAlert bool
			var slaLabel string
			var msgColor string      // Slack color for SLA alerts
			msgColor = "#3498DB"     // Default color for SLA alerts
			slaAlertHandled := false // Flag to indicate SLA alerts were handled separately

			switch alert.AlertType {
			case AlertTypeNewTicket:
				sendAlert = isNewTicket(ticket)
			case AlertTypeTicketUpdate:
				if isUpdatedTicket(ticket, pollingState.lastPollTime) {
					lastPublicCommentTime, isEndUser, err := zc.GetLastPublicCommentTime(ticket.ID)
					if err != nil {
						log.Printf("Skipping ticket %d: %v", ticket.ID, err)
						continue
					}

					// Only send alert if the last comment was after lastPollTime AND made by an end user
					if lastPublicCommentTime.After(pollingState.lastPollTime) && isEndUser {
						sendAlert = true
					} else {
						log.Printf("Skipping ticket update for #%d - no new comment from end user", ticket.ID)
					}
				}

			case AlertTypeSLABreach, AlertTypeSLAReply, AlertTypeSLAResolution:
				slaAlertHandled = true // Mark that we're handling SLA alerts separately
				if slaInfo, ok := slaData[ticket.ID]; ok {
					// Process each metric separately
					for _, metric := range slaInfo.PolicyMetrics {
						label, color, metricType, matches := SlaConditionMatches(metric)
						if !matches {
							continue
						}

						// Determine the alert type based on metric type
						var alertType string
						if metricType == MetricTypeReply {
							alertType = AlertTypeSLAReply
						} else if metricType == MetricTypeResolution {
							alertType = AlertTypeSLAResolution
						} else {
							// Skip unknown metric types
							continue
						}

						// For backward compatibility, also check if user has old AlertTypeSLABreach configured
						// In that case, we'll send alerts for both types
						useOldAlertType := (alert.AlertType == AlertTypeSLABreach)
						if !useOldAlertType && alert.AlertType != alertType {
							// User has specific alert type configured, only send if it matches
							continue
						}

						msgColor = color

						// Retrieve existing alert from cache (using the specific alert type)
						cacheAlertType := alertType
						if useOldAlertType {
							cacheAlertType = alertType // Use specific type even if user has old config
						}
						existingAlert, err := models.GetSLAAlertCache(ctx, db, int(alert.User.ID), int(ticket.ID), cacheAlertType, metricType)

						// If an alert exists, check if the label is different
						if err == nil {
							if existingAlert.Label != label {
								// The label has changed (e.g., from "Less than 3 hours" to "Less than 2 hours"),
								// so clear the existing alert and allow a new alert to be sent
								models.ClearSLAAlertCache(ctx, db, existingAlert.ID)
							} else {
								// If the label hasn't changed, skip the alert to avoid duplicates
								continue
							}
						}

						// Prepare the alert label
						slaLabel = fmt.Sprintf("%s: %s", metricType, label)

						logEntry := models.SLAAlertCache{
							UserID:     int64(alert.User.ID),
							TicketID:   int64(ticket.ID),
							AlertType:  cacheAlertType,
							MetricType: metricType,
							BreachAt:   metric.BreachAt,
							Label:      label,
						}
						if err := models.CreateSLAAlertCache(ctx, db, logEntry); err != nil {
							fmt.Printf("Failed to log SLA alert for Ticket #%d: %v\n", ticket.ID, err)
						}

						// Log the alert
						logAlert(alert, ticket, cacheAlertType)
						timestamp := time.Now().Format("2006-01-02 15:04:05")
						alertLog := models.AlertLog{
							UserID:    int64(alert.User.ID),
							TicketID:  int64(ticket.ID),
							Tag:       alert.Tag,
							AlertType: cacheAlertType,
							Timestamp: timestamp,
						}
						models.CreateAlertLog(ctx, db, alertLog)

						// Create a filtered SLAInfo with just this metric for the message
						filteredSLAInfo := SLAInfo{
							PolicyMetrics: []SLAPolicyMetric{metric},
						}

						// Send Slack message
						err = slackService.SendSlackMessage(alert.SlackChannelID, cacheAlertType, slaLabel, ticket, &filteredSLAInfo, alert.Tag, msgColor)
						if err != nil {
							fmt.Printf("Failed to send Slack message for Ticket #%d: %v\n", ticket.ID, err)
						}
					}
				}
			}

			// Only send generic alert if it's not an SLA alert (SLA alerts are handled above)
			if sendAlert && !slaAlertHandled {
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
		processedTickets[ticket.ID] = true
	}
	middlewares.AddGlobalNotification(sseServer, "Ticket processing complete", fmt.Sprintf("Processed %v tickets...", len(tickets)), "success")
	log.Printf("Ticket processing complete. Processed %d tickets...\n", len(tickets))
}

// MaxBreachAge is the maximum age of a breached SLA before we stop alerting on it.
// This prevents a flood of notifications when the app first starts.
const MaxBreachAge = 3 * time.Hour

// SlaConditionMatches checks if the SLA condition matches the threshold for sending alerts.
// Returns: label, color, metricType, shouldAlert
func SlaConditionMatches(metric SLAPolicyMetric) (string, string, string, bool) {
	if metric.Stage != "active" {
		return "", "", "", false
	}

	// Determine metric type from the metric name
	metricType := ""
	if metric.Metric == "reply_time" || metric.Metric == "first_reply_time" {
		metricType = MetricTypeReply
	} else if metric.Metric == "resolution_time" || metric.Metric == "full_resolution_time" {
		metricType = MetricTypeResolution
	} else {
		// Unknown metric type, skip it
		return "", "", "", false
	}

	timeRemaining := time.Until(metric.BreachAt)
	if timeRemaining < 0 {
		// SLA is breached - check how long ago
		timeSinceBreach := -timeRemaining
		if timeSinceBreach > MaxBreachAge {
			// Don't alert on SLAs that were breached more than 3 hours ago
			return "", "", "", false
		}
		return "BREACHED", "#FF0000", metricType, true // Red for breached SLA
	}
	switch {
	case timeRemaining <= 3*time.Hour && timeRemaining > 2*time.Hour:
		return "Less than 3 hours remaining", "#3498DB", metricType, true // Blue
	case timeRemaining <= 2*time.Hour && timeRemaining > 1*time.Hour:
		return "Less than 2 hours remaining", "#F1C40F", metricType, true // Yellow
	case timeRemaining <= 1*time.Hour && timeRemaining > 30*time.Minute:
		return "Less than 1 hour remaining", "#FFA500", metricType, true // Orange
	case timeRemaining <= 30*time.Minute && timeRemaining > 15*time.Minute:
		return "Less than 30 minutes remaining", "#FF8C00", metricType, true // Darker Orange
	case timeRemaining <= 15*time.Minute:
		return "Less than 15 minutes remaining", "#FF0000", metricType, true // Red
	}
	return "", "", "", false
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
	// Check if ticket was created in the last 5 minutes
	threshold := time.Now().Add(-5 * time.Minute)
	return ticket.CreatedAt.After(threshold)
}

// Helper function to determine if a ticket is updated.
func isUpdatedTicket(ticket zendesk.Ticket, lastPollTime time.Time) bool {
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

	resp, err := zc.httpClient.Do(req)
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

	resp, err := zc.httpClient.Do(req)
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

// GetLastPublicCommentTime retrieves the timestamp of the last public comment on a ticket
// and determines if the comment was made by an end user.
func (zc *ZendeskClient) GetLastPublicCommentTime(ticketID int64) (time.Time, bool, error) {
	var lastPublicCommentTime time.Time
	var isEndUser bool // Tracks if the last comment was from an end user

	ops := zendesk.NewPaginationOptions()
	ops.PageSize = 10
	ops.Id = ticketID
	it := zc.client.GetTicketCommentsIterator(context.Background(), ops)

	for it.HasMore() {
		comments, err := it.GetNext()
		if err != nil {
			return time.Time{}, false, fmt.Errorf("failed to retrieve comments for ticket %d: %v", ticketID, err)
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

				// Determine if the commenter is an end user
				requester, err := zc.GetRequesterByID(lastComment.AuthorID)
				if err != nil {
					return time.Time{}, false, fmt.Errorf("failed to retrieve author details for ticket %d: %v", ticketID, err)

				}

				isEndUser = (requester.Role == "end-user")

			}
		}
	}

	if lastPublicCommentTime.IsZero() {
		return time.Time{}, false, fmt.Errorf("no public comments found for ticket %d", ticketID)
	}

	return lastPublicCommentTime, isEndUser, nil
}

// GetSLALabel returns a formatted SLA label for a ticket
func GetSLALabel(ticket zendesk.Ticket, slaData map[int64]SLAInfo) string {
	slaInfo, exists := slaData[ticket.ID]
	if !exists || len(slaInfo.PolicyMetrics) == 0 {
		return "No SLA"
	}

	metric := slaInfo.PolicyMetrics[0]
	return fmt.Sprintf("%s - %d hours %d minutes remaining", metric.Metric, metric.Hours, metric.Minutes)
}
