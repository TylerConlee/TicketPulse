package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/logging"
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
	httpClient *http.Client
	Subdomain  string
	Email      string
	APIToken   string
	DB         db.Database

	requesterCache    map[int64]*User
	organizationCache map[int64]*Organization
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
		client:            client,
		httpClient:        httpClient,
		Subdomain:         subdomain,
		Email:             email,
		APIToken:          apiToken,
		DB:                db,
		requesterCache:    make(map[int64]*User),
		organizationCache: make(map[int64]*Organization),
	}, nil
}

// NewZendeskClientForTesting creates a ZendeskClient for testing without database configuration.
// It accepts the credentials directly and uses the provided HTTP client.
func NewZendeskClientForTesting(subdomain, email, apiToken string, httpClient *http.Client) *ZendeskClient {
	return &ZendeskClient{
		client:            nil,
		httpClient:        httpClient,
		Subdomain:         subdomain,
		Email:             email,
		APIToken:          apiToken,
		DB:                nil,
		requesterCache:    make(map[int64]*User),
		organizationCache: make(map[int64]*Organization),
	}
}

// NewZendeskClientWithConfig initializes a ZendeskClient using a ConfigCache instead of direct DB reads.
func NewZendeskClientWithConfig(configCache *ConfigCache, httpClient *http.Client) (*ZendeskClient, error) {
	subdomain, err := configCache.Get("zendesk_subdomain")
	if err != nil || subdomain == "" {
		return nil, fmt.Errorf("failed to retrieve Zendesk subdomain")
	}
	email, err := configCache.Get("zendesk_email")
	if err != nil || email == "" {
		return nil, fmt.Errorf("failed to retrieve Zendesk email")
	}
	apiToken, err := configCache.Get("zendesk_api_key")
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
		client:            client,
		httpClient:        httpClient,
		Subdomain:         subdomain,
		Email:             email,
		APIToken:          apiToken,
		DB:                configCache.db,
		requesterCache:    make(map[int64]*User),
		organizationCache: make(map[int64]*Organization),
	}, nil
}


func processTickets(ctx context.Context, db db.Database, tickets []zendesk.Ticket, slaData map[int64]SLAInfo, sseServer *middlewares.SSEServer, slackService *SlackService, zc *ZendeskClient, pollingState *PollingState) {
	userAlerts, err := models.GetAllTagAlerts(db)
	if err != nil {
		log.Printf("Error fetching user alerts: %v", err)
		return
	}
	log.Printf("Processing %d tickets...\n", len(tickets))
	logging.Debug(logging.AreaTags, "Loaded %d user tag alert configurations", len(userAlerts))

	tagIndex := buildTagIndex(userAlerts)
	processedTickets := make(map[int64]bool)

	for _, ticket := range tickets {
		if processedTickets[ticket.ID] {
			logging.Debug(logging.AreaPolling, "Ticket #%d: skipping (already processed this cycle)", ticket.ID)
			continue
		}

		matchedAlerts := matchAlertsForTicket(ticket, tagIndex)
		for _, alert := range matchedAlerts {
		switch alert.AlertType {
		case AlertTypeNewTicket:
			if isNewTicket(ticket) {
				logging.Debug(logging.AreaPolling, "Ticket #%d: new_ticket -> sending alert", ticket.ID)
				sendGenericAlert(ctx, db, alert, ticket, slaData, slackService, zc)
			}
		case AlertTypeTicketUpdate:
			if shouldSendUpdateAlert(ticket, pollingState, zc) {
				sendGenericAlert(ctx, db, alert, ticket, slaData, slackService, zc)
			}
		case AlertTypeSLABreach, AlertTypeSLAReply, AlertTypeSLAResolution:
			handleSLAAlerts(ctx, db, alert, ticket, slaData, slackService, zc)
		}
		}
		processedTickets[ticket.ID] = true
	}
	middlewares.AddGlobalNotification(sseServer, "Ticket processing complete", fmt.Sprintf("Processed %v tickets...", len(tickets)), "success")
	log.Printf("Ticket processing complete. Processed %d tickets...\n", len(tickets))
}

func buildTagIndex(userAlerts []models.TagAlert) map[string][]models.TagAlert {
	tagIndex := make(map[string][]models.TagAlert)
	for _, alert := range userAlerts {
		tagIndex[alert.Tag] = append(tagIndex[alert.Tag], alert)
		logging.Debug(logging.AreaTags, "  Tag index: tag=%q -> user=%d(%s) alertType=%s channel=%s",
			alert.Tag, alert.User.ID, alert.User.Email, alert.AlertType, alert.SlackChannelID)
	}
	logging.Debug(logging.AreaTags, "Tag index built with %d unique tags", len(tagIndex))
	return tagIndex
}

func matchAlertsForTicket(ticket zendesk.Ticket, tagIndex map[string][]models.TagAlert) []models.TagAlert {
	var matched []models.TagAlert
	for _, ticketTag := range ticket.Tags {
		if alerts, exists := tagIndex[ticketTag]; exists {
			matched = append(matched, alerts...)
		}
	}
	if len(matched) == 0 {
		logging.Debug(logging.AreaTags, "Ticket #%d: no tag matches (ticket tags: %v)", ticket.ID, ticket.Tags)
	} else {
		logging.Debug(logging.AreaTags, "Ticket #%d: matched %d alert configs via tags %v",
			ticket.ID, len(matched), ticket.Tags)
	}
	return matched
}

func shouldSendUpdateAlert(ticket zendesk.Ticket, pollingState *PollingState, zc *ZendeskClient) bool {
	if !isUpdatedTicket(ticket, pollingState.lastPollTime) {
		logging.Debug(logging.AreaPolling, "Ticket #%d: ticket_update -> not updated since lastPoll=%s",
			ticket.ID, pollingState.lastPollTime.Format(time.RFC3339))
		return false
	}

	lastPublicCommentTime, isEndUser, err := zc.GetLastPublicCommentTime(ticket.ID)
	if err != nil {
		log.Printf("Skipping ticket %d: %v", ticket.ID, err)
		return false
	}

	if lastPublicCommentTime.After(pollingState.lastPollTime) && isEndUser {
		logging.Debug(logging.AreaPolling, "Ticket #%d: ticket_update -> sendAlert=true (comment at %s by end-user)",
			ticket.ID, lastPublicCommentTime.Format(time.RFC3339))
		return true
	}

	log.Printf("Skipping ticket update for #%d - no new comment from end user", ticket.ID)
	return false
}

func sendGenericAlert(ctx context.Context, database db.Database, alert models.TagAlert, ticket zendesk.Ticket, slaData map[int64]SLAInfo, slackService *SlackService, zc *ZendeskClient) {
	logAlert(alert, ticket, alert.AlertType)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	alertLog := models.AlertLog{
		UserID:    int64(alert.User.ID),
		TicketID:  int64(ticket.ID),
		Tag:       alert.Tag,
		AlertType: alert.AlertType,
		Timestamp: timestamp,
	}
	models.CreateAlertLog(ctx, database, alertLog)

	slaInfo := slaData[ticket.ID]
	logging.Debug(logging.AreaSlack, "Ticket #%d: sending Slack alert type=%s to channel=%s",
		ticket.ID, alert.AlertType, alert.SlackChannelID)
	err := slackService.SendSlackMessageWithClient(alert.SlackChannelID, alert.AlertType, "", ticket, &slaInfo, alert.Tag, "#3498DB", zc)
	if err != nil {
		log.Printf("Failed to send Slack message for Ticket #%d: %v", ticket.ID, err)
	}
}

func handleSLAAlerts(ctx context.Context, database db.Database, alert models.TagAlert, ticket zendesk.Ticket, slaData map[int64]SLAInfo, slackService *SlackService, zc *ZendeskClient) {
	logging.Debug(logging.AreaSLA, "Ticket #%d: processing SLA alert (user alertType=%s, user=%d(%s))",
		ticket.ID, alert.AlertType, alert.User.ID, alert.User.Email)

	slaInfo, ok := slaData[ticket.ID]
	if !ok {
		logging.Debug(logging.AreaSLA, "Ticket #%d: NO SLA data found in slaData map", ticket.ID)
		return
	}

	logging.Debug(logging.AreaSLA, "Ticket #%d: has %d SLA metrics to evaluate", ticket.ID, len(slaInfo.PolicyMetrics))

	for i, metric := range slaInfo.PolicyMetrics {
		label, color, metricType, matches := SlaConditionMatches(metric)
		logging.Debug(logging.AreaSLA, "Ticket #%d: metric[%d] %q stage=%q -> matches=%v label=%q metricType=%q",
			ticket.ID, i, metric.Metric, metric.Stage, matches, label, metricType)

		if !matches {
			continue
		}

		alertType := metricTypeToAlertType(metricType)
		if alertType == "" {
			logging.Debug(logging.AreaSLA, "Ticket #%d: metric[%d] skipped: unknown metricType=%q", ticket.ID, i, metricType)
			continue
		}

		useOldAlertType := (alert.AlertType == AlertTypeSLABreach)
		if !useOldAlertType && alert.AlertType != alertType {
			logging.Debug(logging.AreaSLA, "Ticket #%d: metric[%d] skipped: user wants %q but metric maps to %q",
				ticket.ID, i, alert.AlertType, alertType)
			continue
		}

		if isDuplicateSLAAlert(ctx, database, alert, ticket, alertType, metricType, label) {
			continue
		}

		slaLabel := fmt.Sprintf("%s: %s", metricType, label)
		cacheSLAAlert(ctx, database, alert, ticket, alertType, metricType, metric.BreachAt, label)

		logAlert(alert, ticket, alertType)
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		models.CreateAlertLog(ctx, database, models.AlertLog{
			UserID:    int64(alert.User.ID),
			TicketID:  int64(ticket.ID),
			Tag:       alert.Tag,
			AlertType: alertType,
			Timestamp: timestamp,
		})

		filteredSLAInfo := SLAInfo{PolicyMetrics: []SLAPolicyMetric{metric}}
		logging.Debug(logging.AreaSlack, "Ticket #%d: sending Slack SLA alert to channel=%s type=%s label=%q",
			ticket.ID, alert.SlackChannelID, alertType, slaLabel)
		err := slackService.SendSlackMessageWithClient(alert.SlackChannelID, alertType, slaLabel, ticket, &filteredSLAInfo, alert.Tag, color, zc)
		if err != nil {
			log.Printf("Failed to send Slack message for Ticket #%d: %v", ticket.ID, err)
		}
	}
}

func metricTypeToAlertType(metricType string) string {
	switch metricType {
	case MetricTypeReply:
		return AlertTypeSLAReply
	case MetricTypeResolution:
		return AlertTypeSLAResolution
	default:
		return ""
	}
}

func isDuplicateSLAAlert(ctx context.Context, database db.Database, alert models.TagAlert, ticket zendesk.Ticket, alertType, metricType, label string) bool {
	existingAlert, err := models.GetSLAAlertCache(ctx, database, int(alert.User.ID), int(ticket.ID), alertType, metricType)
	if err != nil {
		logging.Debug(logging.AreaCache, "Ticket #%d: no cache entry for user=%d type=%s metric=%s -> will alert",
			ticket.ID, alert.User.ID, alertType, metricType)
		return false
	}

	if existingAlert.Label == label {
		logging.Debug(logging.AreaCache, "Ticket #%d: cache hit, same label=%q -> SKIPPING (dedup)",
			ticket.ID, label)
		return true
	}

	logging.Debug(logging.AreaCache, "Ticket #%d: cache hit, label changed %q -> %q, clearing old entry",
		ticket.ID, existingAlert.Label, label)
	models.ClearSLAAlertCache(ctx, database, existingAlert.ID)
	return false
}

func cacheSLAAlert(ctx context.Context, database db.Database, alert models.TagAlert, ticket zendesk.Ticket, alertType, metricType string, breachAt time.Time, label string) {
	logEntry := models.SLAAlertCache{
		UserID:     int64(alert.User.ID),
		TicketID:   int64(ticket.ID),
		AlertType:  alertType,
		MetricType: metricType,
		BreachAt:   breachAt,
		Label:      label,
	}
	if err := models.CreateSLAAlertCache(ctx, database, logEntry); err != nil {
		log.Printf("Failed to log SLA alert for Ticket #%d: %v", ticket.ID, err)
	} else {
		logging.Debug(logging.AreaCache, "Ticket #%d: created cache entry for user=%d type=%s metric=%s label=%q",
			ticket.ID, alert.User.ID, alertType, metricType, label)
	}
}

// MaxBreachAge is the maximum age of a breached SLA before we stop alerting on it.
// This prevents a flood of notifications when the app first starts or when
// tickets have long-standing breaches. Set to 24 hours to cover a full business day.
const MaxBreachAge = 24 * time.Hour

// SlaConditionMatches checks if the SLA condition matches the threshold for sending alerts.
// Returns: label, color, metricType, shouldAlert
func SlaConditionMatches(metric SLAPolicyMetric) (string, string, string, bool) {
	logging.Debug(logging.AreaSLA, "SlaConditionMatches: evaluating metric=%q stage=%q breach_at=%s",
		metric.Metric, metric.Stage, metric.BreachAt.Format(time.RFC3339))

	if metric.Stage != "active" {
		logging.Debug(logging.AreaSLA, "SlaConditionMatches: REJECTED - stage=%q is not active", metric.Stage)
		return "", "", "", false
	}

	// Determine metric type from the metric name.
	// Zendesk SLA metric names:
	//   Reply:      reply_time, first_reply_time, next_reply_time
	//   Resolution: resolution_time, full_resolution_time, total_resolution_time
	metricType := ""
	switch metric.Metric {
	case "reply_time", "first_reply_time", "next_reply_time":
		metricType = MetricTypeReply
	case "resolution_time", "full_resolution_time", "total_resolution_time":
		metricType = MetricTypeResolution
	default:
		logging.Debug(logging.AreaSLA, "SlaConditionMatches: REJECTED - unknown metric name %q", metric.Metric)
		return "", "", "", false
	}

	timeRemaining := time.Until(metric.BreachAt)
	logging.Debug(logging.AreaSLA, "SlaConditionMatches: metricType=%s timeRemaining=%s", metricType, timeRemaining.Round(time.Second))

	if timeRemaining < 0 {
		timeSinceBreach := -timeRemaining
		if timeSinceBreach > MaxBreachAge {
			logging.Debug(logging.AreaSLA, "SlaConditionMatches: REJECTED - breached %s ago, exceeds MaxBreachAge=%s",
				timeSinceBreach.Round(time.Second), MaxBreachAge)
			return "", "", "", false
		}
		logging.Debug(logging.AreaSLA, "SlaConditionMatches: MATCHED - BREACHED %s ago (within MaxBreachAge)", timeSinceBreach.Round(time.Second))
		return "BREACHED", "#FF0000", metricType, true
	}
	switch {
	case timeRemaining <= 3*time.Hour && timeRemaining > 2*time.Hour:
		logging.Debug(logging.AreaSLA, "SlaConditionMatches: MATCHED - less than 3 hours remaining (%s)", timeRemaining.Round(time.Second))
		return "Less than 3 hours remaining", "#3498DB", metricType, true
	case timeRemaining <= 2*time.Hour && timeRemaining > 1*time.Hour:
		logging.Debug(logging.AreaSLA, "SlaConditionMatches: MATCHED - less than 2 hours remaining (%s)", timeRemaining.Round(time.Second))
		return "Less than 2 hours remaining", "#F1C40F", metricType, true
	case timeRemaining <= 1*time.Hour && timeRemaining > 30*time.Minute:
		logging.Debug(logging.AreaSLA, "SlaConditionMatches: MATCHED - less than 1 hour remaining (%s)", timeRemaining.Round(time.Second))
		return "Less than 1 hour remaining", "#FFA500", metricType, true
	case timeRemaining <= 30*time.Minute && timeRemaining > 15*time.Minute:
		logging.Debug(logging.AreaSLA, "SlaConditionMatches: MATCHED - less than 30 minutes remaining (%s)", timeRemaining.Round(time.Second))
		return "Less than 30 minutes remaining", "#FF8C00", metricType, true
	case timeRemaining <= 15*time.Minute:
		logging.Debug(logging.AreaSLA, "SlaConditionMatches: MATCHED - less than 15 minutes remaining (%s)", timeRemaining.Round(time.Second))
		return "Less than 15 minutes remaining", "#FF0000", metricType, true
	}
	logging.Debug(logging.AreaSLA, "SlaConditionMatches: REJECTED - %s remaining, outside all alert thresholds", timeRemaining.Round(time.Second))
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
// Results are cached for the lifetime of the ZendeskClient to avoid redundant API calls
// within a single polling cycle.
func (zc *ZendeskClient) GetRequesterByID(userID int64) (*User, error) {
	if cached, ok := zc.requesterCache[userID]; ok {
		return cached, nil
	}

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

	zc.requesterCache[userID] = &result.User
	return &result.User, nil
}

// GetOrganizationByID retrieves an organization from Zendesk based on its ID.
// Results are cached for the lifetime of the ZendeskClient.
func (zc *ZendeskClient) GetOrganizationByID(organizationID int64) (*Organization, error) {
	if cached, ok := zc.organizationCache[organizationID]; ok {
		return cached, nil
	}

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

	zc.organizationCache[organizationID] = &result.Organization
	return &result.Organization, nil
}

// GetLastPublicCommentTime retrieves the timestamp of the last public comment on a ticket
// and determines if the comment was made by an end user.
func (zc *ZendeskClient) GetLastPublicCommentTime(ticketID int64) (time.Time, bool, error) {
	logging.Debug(logging.AreaZendesk, "GetLastPublicCommentTime: fetching comments for ticket=%d", ticketID)
	var lastPublicCommentTime time.Time
	var isEndUser bool

	ops := zendesk.NewPaginationOptions()
	ops.PageSize = 10
	ops.Id = ticketID
	it := zc.client.GetTicketCommentsIterator(context.Background(), ops)

	commentCount := 0
	for it.HasMore() {
		comments, err := it.GetNext()
		if err != nil {
			return time.Time{}, false, fmt.Errorf("failed to retrieve comments for ticket %d: %v", ticketID, err)
		}

		if len(comments) == 0 {
			continue
		}
		commentCount += len(comments)

		lastComment := comments[len(comments)-1]

		if lastComment.Public != nil && *lastComment.Public {
			commentTime := lastComment.CreatedAt
			if commentTime.After(lastPublicCommentTime) {
				lastPublicCommentTime = commentTime

				requester, err := zc.GetRequesterByID(lastComment.AuthorID)
				if err != nil {
					return time.Time{}, false, fmt.Errorf("failed to retrieve author details for ticket %d: %v", ticketID, err)
				}

				isEndUser = (requester.Role == "end-user")
				logging.Debug(logging.AreaZendesk, "GetLastPublicCommentTime: ticket=%d lastPublicComment at %s by authorID=%d role=%s isEndUser=%v",
					ticketID, commentTime.Format(time.RFC3339), lastComment.AuthorID, requester.Role, isEndUser)
			}
		}
	}

	if lastPublicCommentTime.IsZero() {
		logging.Debug(logging.AreaZendesk, "GetLastPublicCommentTime: ticket=%d no public comments found (%d total comments)", ticketID, commentCount)
		return time.Time{}, false, fmt.Errorf("no public comments found for ticket %d", ticketID)
	}

	logging.Debug(logging.AreaZendesk, "GetLastPublicCommentTime: ticket=%d result: time=%s isEndUser=%v (scanned %d comments)",
		ticketID, lastPublicCommentTime.Format(time.RFC3339), isEndUser, commentCount)
	return lastPublicCommentTime, isEndUser, nil
}

// ListAllTags fetches all tags from the Zendesk account using the Tags API.
// It paginates through all results and returns deduplicated tag names.
func (zc *ZendeskClient) ListAllTags() ([]string, error) {
	var allTags []string
	nextPage := fmt.Sprintf("https://%s.zendesk.com/api/v2/tags?page[size]=100", zc.Subdomain)

	for nextPage != "" {
		req, err := http.NewRequest("GET", nextPage, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.SetBasicAuth(zc.Email+"/token", zc.APIToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := zc.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tags: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("zendesk tags API returned status %s", resp.Status)
		}

		var result struct {
			Tags []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"tags"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode tags response: %w", err)
		}

		for _, tag := range result.Tags {
			allTags = append(allTags, tag.Name)
		}

		nextPage = result.Links.Next
	}

	logging.Debug(logging.AreaZendesk, "ListAllTags: fetched %d tags from Zendesk", len(allTags))
	return allTags, nil
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

// AssignTicket assigns a Zendesk ticket to the specified user.
func (zc *ZendeskClient) AssignTicket(ticketID int64, assigneeID int64) error {
	_, err := zc.client.UpdateTicket(context.Background(), ticketID, zendesk.Ticket{
		AssigneeID: assigneeID,
	})
	if err != nil {
		return fmt.Errorf("failed to assign ticket %d to user %d: %w", ticketID, assigneeID, err)
	}
	logging.Debug(logging.AreaZendesk, "AssignTicket: ticket=%d assigned to user=%d", ticketID, assigneeID)
	return nil
}

// AddInternalNote posts an internal (private) note on a Zendesk ticket.
func (zc *ZendeskClient) AddInternalNote(ticketID int64, body string) error {
	comment := zendesk.NewPrivateTicketComment(body, 0)
	_, err := zc.client.CreateTicketComment(context.Background(), ticketID, comment)
	if err != nil {
		return fmt.Errorf("failed to add internal note to ticket %d: %w", ticketID, err)
	}
	logging.Debug(logging.AreaZendesk, "AddInternalNote: added note to ticket=%d", ticketID)
	return nil
}
