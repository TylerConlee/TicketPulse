// Package services provides core business logic for TicketPulse.
package services

import (
	"sync"

	"github.com/nukosuke/go-zendesk/zendesk"
)

// TicketContext holds pre-fetched data for a ticket to reduce API calls.
type TicketContext struct {
	Ticket           zendesk.Ticket
	SLAInfo          *SLAInfo
	RequesterName    string
	OrganizationName string
	TicketURL        string
}

// NewTicketContext creates a TicketContext from a ticket with optional enrichment data.
func NewTicketContext(ticket zendesk.Ticket, slaInfo *SLAInfo, subdomain string) TicketContext {
	return TicketContext{
		Ticket:           ticket,
		SLAInfo:          slaInfo,
		RequesterName:    "Unknown Requester",
		OrganizationName: "Unknown Organization",
		TicketURL:        buildTicketURL(subdomain, ticket.ID),
	}
}

// buildTicketURL constructs a Zendesk ticket URL.
func buildTicketURL(subdomain string, ticketID int64) string {
	if subdomain == "" {
		return ""
	}
	return "https://" + subdomain + ".zendesk.com/agent/tickets/" + formatInt64(ticketID)
}

// formatInt64 converts an int64 to a string without importing strconv in this context.
func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte(n%10) + '0'
		n /= 10
	}

	if negative {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}

// WithRequester sets the requester name on the context.
func (tc TicketContext) WithRequester(name string) TicketContext {
	tc.RequesterName = name
	return tc
}

// WithOrganization sets the organization name on the context.
func (tc TicketContext) WithOrganization(name string) TicketContext {
	tc.OrganizationName = name
	return tc
}

// BatchGetRequesters fetches multiple users by ID in parallel and returns a map.
func (zc *ZendeskClient) BatchGetRequesters(userIDs []int64) map[int64]*User {
	result := make(map[int64]*User)
	if len(userIDs) == 0 {
		return result
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrent requests
	semaphore := make(chan struct{}, 10)

	for _, userID := range userIDs {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			user, err := zc.GetRequesterByID(id)
			if err != nil {
				return
			}

			mu.Lock()
			result[id] = user
			mu.Unlock()
		}(userID)
	}

	wg.Wait()
	return result
}

// BatchGetOrganizations fetches multiple organizations by ID in parallel and returns a map.
func (zc *ZendeskClient) BatchGetOrganizations(orgIDs []int64) map[int64]*Organization {
	result := make(map[int64]*Organization)
	if len(orgIDs) == 0 {
		return result
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrent requests
	semaphore := make(chan struct{}, 10)

	for _, orgID := range orgIDs {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			org, err := zc.GetOrganizationByID(id)
			if err != nil {
				return
			}

			mu.Lock()
			result[id] = org
			mu.Unlock()
		}(orgID)
	}

	wg.Wait()
	return result
}

// CollectUniqueRequesterIDs extracts unique requester IDs from a slice of tickets.
func CollectUniqueRequesterIDs(tickets []zendesk.Ticket) []int64 {
	seen := make(map[int64]bool)
	var result []int64

	for _, ticket := range tickets {
		if ticket.RequesterID > 0 && !seen[ticket.RequesterID] {
			seen[ticket.RequesterID] = true
			result = append(result, ticket.RequesterID)
		}
	}

	return result
}

// CollectUniqueOrganizationIDs extracts unique organization IDs from a slice of tickets.
func CollectUniqueOrganizationIDs(tickets []zendesk.Ticket) []int64 {
	seen := make(map[int64]bool)
	var result []int64

	for _, ticket := range tickets {
		if ticket.OrganizationID > 0 && !seen[ticket.OrganizationID] {
			seen[ticket.OrganizationID] = true
			result = append(result, ticket.OrganizationID)
		}
	}

	return result
}

// EnrichTicketsWithContext creates TicketContexts for a slice of tickets,
// pre-fetching requester and organization information in batch.
func (zc *ZendeskClient) EnrichTicketsWithContext(tickets []zendesk.Ticket, slaData map[int64]SLAInfo, subdomain string) []TicketContext {
	// Collect unique IDs
	requesterIDs := CollectUniqueRequesterIDs(tickets)
	orgIDs := CollectUniqueOrganizationIDs(tickets)

	// Batch fetch
	requesters := zc.BatchGetRequesters(requesterIDs)
	orgs := zc.BatchGetOrganizations(orgIDs)

	// Build contexts
	contexts := make([]TicketContext, 0, len(tickets))
	for _, ticket := range tickets {
		var slaInfo *SLAInfo
		if info, ok := slaData[ticket.ID]; ok {
			slaInfo = &info
		}

		ctx := NewTicketContext(ticket, slaInfo, subdomain)

		// Enrich with requester
		if requester, ok := requesters[ticket.RequesterID]; ok {
			ctx = ctx.WithRequester(requester.Name)
		}

		// Enrich with organization
		if org, ok := orgs[ticket.OrganizationID]; ok {
			ctx = ctx.WithOrganization(org.Name)
		}

		contexts = append(contexts, ctx)
	}

	return contexts
}

// TicketContextMap provides fast lookup of contexts by ticket ID.
type TicketContextMap map[int64]TicketContext

// BuildTicketContextMap creates a map of ticket ID to TicketContext.
func BuildTicketContextMap(contexts []TicketContext) TicketContextMap {
	result := make(TicketContextMap, len(contexts))
	for _, ctx := range contexts {
		result[ctx.Ticket.ID] = ctx
	}
	return result
}
