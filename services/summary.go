package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/nukosuke/go-zendesk/zendesk"
)

const (
	TagFilterAllTags        = "all_tags"
	TagFilterConfiguredTags = "configured_tags"
	TicketFilterAllTickets  = "all_tickets"
	TicketFilterAssigned    = "assigned_tickets"
)

// SearchTicketsWithActiveSLA retrieves tickets with active SLA metrics.
func (zc *ZendeskClient) SearchTicketsWithActiveSLA() ([]zendesk.Ticket, map[int64]SLAInfo, error) {
	var allTickets []zendesk.Ticket
	slaData := make(map[int64]SLAInfo)

	query := "type:ticket status<pending"
	params := url.Values{}
	params.Set("query", query)
	params.Set("include", "tickets(slas)")

	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/search.json?%s", zc.Subdomain, params.Encode())

	for endpoint != "" {
		req, err := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.SetBasicAuth(zc.Email+"/token", zc.APIToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := zc.httpClient.Do(req)
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

		for _, ticketResult := range result.Results {
			allTickets = append(allTickets, ticketResult.Ticket)
			if len(ticketResult.SLAMetrics.PolicyMetrics) > 0 {
				slaData[ticketResult.Ticket.ID] = SLAInfo{
					PolicyMetrics: ticketResult.SLAMetrics.PolicyMetrics,
				}
			}
		}

		endpoint = result.NextPage
	}

	return allTickets, slaData, nil
}

// SearchNewOrUpdatedTickets searches for tickets that were updated since the given time.
func (zc *ZendeskClient) SearchNewOrUpdatedTickets(since time.Time) ([]zendesk.Ticket, error) {
	var allTickets []zendesk.Ticket

	query := fmt.Sprintf("type:ticket updated>%s", since.Format(time.RFC3339))
	params := url.Values{}
	params.Set("query", query)

	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/search.json?%s", zc.Subdomain, params.Encode())

	for endpoint != "" {
		req, err := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.SetBasicAuth(zc.Email+"/token", zc.APIToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := zc.httpClient.Do(req)
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

// GetTicketsAssignedToUser retrieves tickets assigned to a specific user.
func (zc *ZendeskClient) GetTicketsAssignedToUser(userID int64, since time.Time) ([]zendesk.Ticket, error) {
	var allTickets []zendesk.Ticket

	query := fmt.Sprintf("type:ticket assignee_id:%d updated>%s", userID, since.Format(time.RFC3339))
	params := url.Values{}
	params.Set("query", query)

	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/search.json?%s", zc.Subdomain, params.Encode())

	for endpoint != "" {
		req, err := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.SetBasicAuth(zc.Email+"/token", zc.APIToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := zc.httpClient.Do(req)
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

// GetUserByEmail retrieves a user from Zendesk based on their email address.
func (zc *ZendeskClient) GetUserByEmail(email string) (*zendesk.User, error) {
	query := fmt.Sprintf("type:user email:%s", email)
	params := url.Values{}
	params.Set("query", query)

	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/search.json?%s", zc.Subdomain, params.Encode())

	req, err := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
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
		Results []zendesk.User `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Results) == 0 {
		return nil, fmt.Errorf("no user found with email: %s", email)
	}

	return &result.Results[0], nil
}

// GetCSATRatingsForUser fetches CSAT ratings for the user's tickets since the specified time.
func (zc *ZendeskClient) GetCSATRatingsForUser(userID int64, since time.Time) ([]SatisfactionRating, error) {
	query := fmt.Sprintf("type:ticket assignee_id:%d satisfaction_rating:offered updated>%s", userID, since.Format(time.RFC3339))
	params := url.Values{}
	params.Set("query", query)

	endpoint := fmt.Sprintf("https://%s.zendesk.com/api/v2/search.json?%s", zc.Subdomain, params.Encode())

	req, err := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
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
		Results []SatisfactionRating `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

// GenerateDailySummary generates a daily summary and sends it to the user via Slack.
func (zc *ZendeskClient) GenerateDailySummary(userEmail string, slackService *SlackService, workDayStart, workDayEnd time.Time, timezone string, tagFilterMode, ticketFilterMode string, userTags []string) (string, error) {
	// Step 1: Get the Zendesk user ID from the email
	zendeskUser, err := zc.GetUserByEmail(userEmail)
	if err != nil {
		return "", fmt.Errorf("failed to get Zendesk user by email: %v", err)
	}

	// Step 2: Calculate the end of previous work day
	since, err := calculatePreviousWorkDayEnd(workDayStart, workDayEnd, timezone)
	if err != nil {
		return "", fmt.Errorf("failed to calculate previous work day end: %v", err)
	}

	// Step 3: Retrieve tickets based on filter mode
	var tickets []zendesk.Ticket
	if ticketFilterMode == TicketFilterAssigned {
		// Get only assigned tickets
		tickets, err = zc.GetTicketsAssignedToUser(zendeskUser.ID, since)
		if err != nil {
			return "", fmt.Errorf("failed to get tickets assigned to user: %v", err)
		}
	} else {
		// Get all tickets updated since the end of previous work day
		tickets, err = zc.SearchNewOrUpdatedTickets(since)
		if err != nil {
			return "", fmt.Errorf("failed to get updated tickets: %v", err)
		}
	}

	// Step 4: Filter tickets by tags if needed
	if tagFilterMode == TagFilterConfiguredTags && len(userTags) > 0 {
		tickets = filterTicketsByTags(tickets, userTags)
	}

	// Step 5: Retrieve SLA data for the tickets
	_, slaData, err := zc.SearchTicketsWithActiveSLA()
	if err != nil {
		return "", fmt.Errorf("failed to get SLA data: %v", err)
	}

	// Step 6: Retrieve CSAT ratings for the user since the end of previous work day
	csatRatings, err := zc.GetCSATRatingsForUser(zendeskUser.ID, since)
	if err != nil {
		return "", fmt.Errorf("failed to get CSAT ratings for user: %v", err)
	}

	// Step 7: Identify unread tickets (tickets updated but not by the user)
	unreadTickets := filterUnreadTickets(tickets, zendeskUser.Email)

	// Step 8: Identify open tickets with active SLAs
	openTicketsWithSLA := filterTicketsWithActiveSLA(tickets, slaData)

	// Step 9: Compile the summary message
	summaryMessage := compileSummaryMessage(zendeskUser.Name, unreadTickets, openTicketsWithSLA, csatRatings, since)

	// Step 10: Fetch the Slack user ID
	slackUserID, err := models.GetUserByEmail(zc.DB, userEmail)
	if err != nil {
		return summaryMessage, fmt.Errorf("failed to get Slack user ID: %v", err)
	}
	if !slackUserID.SlackUserID.Valid {
		return summaryMessage, fmt.Errorf("slack user ID is not set for user: %s", userEmail)
	}
	sUID := slackUserID.SlackUserID.String

	// Step 11: Send the Slack message as a DM using block formatting
	err = sendSlackDM(slackService, sUID, unreadTickets, openTicketsWithSLA, csatRatings, slaData)
	if err != nil {
		log.Printf("failed to send Slack DM: %v", err)
	}

	return summaryMessage, nil
}

// calculatePreviousWorkDayEnd calculates the end time of the previous work day
func calculatePreviousWorkDayEnd(workDayStart, workDayEnd time.Time, timezone string) (time.Time, error) {
	// Load timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone: %v", err)
	}

	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	// Extract hour and minute from work day start/end times
	startHour := workDayStart.Hour()
	startMinute := workDayStart.Minute()
	endHour := workDayEnd.Hour()
	endMinute := workDayEnd.Minute()

	// Calculate today's work day start and end
	todayWorkDayStart := time.Date(now.Year(), now.Month(), now.Day(), startHour, startMinute, 0, 0, loc)
	todayWorkDayEnd := time.Date(now.Year(), now.Month(), now.Day(), endHour, endMinute, 0, 0, loc)

	// If we're before today's work day start, previous work day ended yesterday
	// If we're after today's work day start but before work day end, previous work day ended at start of today
	// If we're after today's work day end, previous work day ended at end of today
	if now.Before(todayWorkDayStart) {
		// Previous work day ended yesterday at work day end time
		yesterday := today.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), endHour, endMinute, 0, 0, loc), nil
	} else if now.Before(todayWorkDayEnd) {
		// Previous work day ended at the start of today
		return todayWorkDayStart, nil
	} else {
		// Previous work day ended at the end of today
		return todayWorkDayEnd, nil
	}
}

// filterTicketsByTags filters tickets to include only those with matching tags
func filterTicketsByTags(tickets []zendesk.Ticket, userTags []string) []zendesk.Ticket {
	var filteredTickets []zendesk.Ticket
	tagMap := make(map[string]bool)
	for _, tag := range userTags {
		tagMap[tag] = true
	}

	for _, ticket := range tickets {
		for _, ticketTag := range ticket.Tags {
			if tagMap[ticketTag] {
				filteredTickets = append(filteredTickets, ticket)
				break
			}
		}
	}

	return filteredTickets
}

// filterUnreadTickets filters unread tickets (those updated but not by the user).
func filterUnreadTickets(tickets []zendesk.Ticket, userEmail string) []zendesk.Ticket {
	var unreadTickets []zendesk.Ticket
	for _, ticket := range tickets {
		email, _ := ticket.Via.Source.From["address"].(string)
		if email != userEmail {
			unreadTickets = append(unreadTickets, ticket)
		}
	}
	return unreadTickets
}

// filterTicketsWithActiveSLA filters tickets to include only those with active SLA policies.
func filterTicketsWithActiveSLA(tickets []zendesk.Ticket, slaData map[int64]SLAInfo) []zendesk.Ticket {
	var activeSLATickets []zendesk.Ticket

	for _, ticket := range tickets {
		if slaInfo, exists := slaData[ticket.ID]; exists {
			// Check if any of the SLA metrics for the ticket are active
			for _, metric := range slaInfo.PolicyMetrics {
				if metric.Stage == "active" {
					activeSLATickets = append(activeSLATickets, ticket)
					break // No need to check further if we found an active SLA
				}
			}
		}
	}

	return activeSLATickets
}

func compileSummaryMessage(userName string, unreadTickets, openTicketsWithSLA []zendesk.Ticket, csatRatings []SatisfactionRating, since time.Time) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Hello %s! Here's your Daily Summary for %s:\n\n", userName, time.Now().Format("January 2, 2006")))

	// Unread Tickets
	if len(unreadTickets) > 0 {
		sb.WriteString(fmt.Sprintf("You have %d unread tickets that were updated since %s:\n", len(unreadTickets), since.Format("January 2, 2006 15:04")))
		for _, ticket := range unreadTickets {
			sb.WriteString(fmt.Sprintf("- <%s|%s> (ID: %d)\n  %s\n", ticket.URL, ticket.Subject, ticket.ID, truncateDescription(ticket.Description, 30)))
		}
	} else {
		sb.WriteString("No unread tickets since the end of your previous work day.\n")
	}

	// Open Tickets with Active SLAs
	if len(openTicketsWithSLA) > 0 {
		sb.WriteString(fmt.Sprintf("\nYou have %d open tickets with active SLAs:\n", len(openTicketsWithSLA)))
		for _, ticket := range openTicketsWithSLA {
			sb.WriteString(fmt.Sprintf("- <%s|%s> (ID: %d)\n  %s\n", ticket.URL, ticket.Subject, ticket.ID, truncateDescription(ticket.Description, 30)))
		}
	} else {
		sb.WriteString("\nNo open tickets with active SLAs.\n")
	}

	// CSAT Ratings
	if len(csatRatings) > 0 {
		sb.WriteString(fmt.Sprintf("\nYou received %d new CSAT reviews:\n", len(csatRatings)))
		for _, rating := range csatRatings {
			sb.WriteString(fmt.Sprintf("- %s\n", rating.Comment))
		}
	} else {
		sb.WriteString("\nNo new CSAT reviews since the end of your previous work day.\n")
	}

	return sb.String()
}

func truncateDescription(desc string, wordCount int) string {
	words := strings.Fields(desc)
	if len(words) > wordCount {
		return strings.Join(words[:wordCount], " ") + "..."
	}
	return desc
}
