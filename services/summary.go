package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nukosuke/go-zendesk/zendesk"
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

		// Create a custom HTTP client and execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
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

		// Create a custom HTTP client and execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
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

		// Create a custom HTTP client and execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
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

	// Create a custom HTTP client and execute the request
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

	// Create a custom HTTP client and execute the request
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
		Results []SatisfactionRating `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
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

// compileSummaryMessage generates a summary message from provided ticket data.
func compileSummaryMessage(userName string, unreadTickets, openTicketsWithSLA []zendesk.Ticket, csatRatings []SatisfactionRating) string {
	message := fmt.Sprintf("Hello %s! Here’s your Daily Summary for %s:\n\n", userName, time.Now().Format("January 2, 2006"))

	// Unread Tickets
	if len(unreadTickets) > 0 {
		message += fmt.Sprintf("You have %d unread tickets that were updated since yesterday:\n", len(unreadTickets))
		for _, ticket := range unreadTickets {
			message += fmt.Sprintf("- %s (ID: %d)\n", ticket.Subject, ticket.ID)
		}
	} else {
		message += "No unread tickets from the last 24 hours.\n"
	}

	// Open Tickets with Active SLAs
	if len(openTicketsWithSLA) > 0 {
		message += fmt.Sprintf("\nYou have %d open tickets with active SLAs:\n", len(openTicketsWithSLA))
		for _, ticket := range openTicketsWithSLA {
			message += fmt.Sprintf("- %s (ID: %d)\n", ticket.Subject, ticket.ID)
		}
	} else {
		message += "\nNo open tickets with active SLAs.\n"
	}

	// CSAT Ratings
	if len(csatRatings) > 0 {
		message += fmt.Sprintf("\nYou received %d new CSAT reviews:\n", len(csatRatings))
		for _, rating := range csatRatings {
			message += fmt.Sprintf("- Rating: %s (Ticket ID: %d)\n", rating.Score, rating.TicketID)
		}
	} else {
		message += "\nNo new CSAT reviews from the last 24 hours.\n"
	}

	return message
}
