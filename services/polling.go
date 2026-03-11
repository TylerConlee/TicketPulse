// Package services provides core business logic for TicketPulse.
package services

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/nukosuke/go-zendesk/zendesk"
	"golang.org/x/sync/errgroup"
)

// PollingService encapsulates the Zendesk polling logic for testability.
type PollingService struct {
	db           db.Database
	sseServer    *middlewares.SSEServer
	slackService *SlackService
	clock        Clock
	pollingState *PollingState
	configCache  *ConfigCache
}

// PollingResult represents the result of a single polling iteration.
type PollingResult struct {
	SLATicketsCount      int
	UpdatedTicketsCount  int
	ProcessedTicketsCount int
	Error                error
}

// NewPollingService creates a new PollingService.
func NewPollingService(db db.Database, sseServer *middlewares.SSEServer, slackService *SlackService, configCache *ConfigCache) *PollingService {
	return &PollingService{
		db:           db,
		sseServer:    sseServer,
		slackService: slackService,
		clock:        RealClock{},
		pollingState: NewPollingState(),
		configCache:  configCache,
	}
}

// NewPollingServiceWithClock creates a new PollingService with a custom clock for testing.
func NewPollingServiceWithClock(db db.Database, sseServer *middlewares.SSEServer, slackService *SlackService, clock Clock, configCache *ConfigCache) *PollingService {
	return &PollingService{
		db:           db,
		sseServer:    sseServer,
		slackService: slackService,
		clock:        clock,
		pollingState: NewPollingStateWithClock(clock),
		configCache:  configCache,
	}
}

// NewPollingStateWithClock creates a new polling state with a custom clock.
func NewPollingStateWithClock(clock Clock) *PollingState {
	return &PollingState{
		lastPollTime: clock.Now().Add(-5 * time.Minute),
	}
}

// PollOnce performs a single polling iteration.
// This is the testable core of the polling logic.
func (ps *PollingService) PollOnce(ctx context.Context) PollingResult {
	result := PollingResult{}

	var zendeskClient *ZendeskClient
	var err error
	if ps.configCache != nil {
		zendeskClient, err = NewZendeskClientWithConfig(ps.configCache, &http.Client{})
	} else {
		zendeskClient, err = NewZendeskClient(ps.db)
	}
	if err != nil {
		if ps.sseServer != nil {
			middlewares.AddGlobalNotification(ps.sseServer, "Zendesk Configuration Error", fmt.Sprintf("Error fetching Zendesk configuration: %v", err), "danger")
			broadcastStatusUpdates(ps.sseServer, "zendesk", "error", "Error fetching Zendesk configuration")
		}
		result.Error = fmt.Errorf("failed to create Zendesk client: %w", err)
		return result
	}

	if ps.sseServer != nil {
		middlewares.AddGlobalNotification(ps.sseServer, "Refreshing Zendesk tickets", "Requesting tickets from Zendesk", "info")
	}
	log.Println("Requesting tickets from Zendesk...")

	// Fetch SLA tickets and new/updated tickets in parallel
	var slaTickets []zendesk.Ticket
	var slaData map[int64]SLAInfo
	var newUpdatedTickets []zendesk.Ticket

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var slaErr error
		slaTickets, slaData, slaErr = zendeskClient.SearchTicketsWithActiveSLA()
		if slaErr != nil {
			return fmt.Errorf("failed to search SLA tickets: %w", slaErr)
		}
		return nil
	})

	g.Go(func() error {
		_ = gCtx // captured for cancellation awareness
		var updErr error
		newUpdatedTickets, updErr = zendeskClient.SearchNewOrUpdatedTickets(ps.pollingState.lastPollTime)
		if updErr != nil {
			return fmt.Errorf("failed to search new/updated tickets: %w", updErr)
		}
		return nil
	})

	if err = g.Wait(); err != nil {
		if ps.sseServer != nil {
			middlewares.AddGlobalNotification(ps.sseServer, "Zendesk Connectivity Error", fmt.Sprintf("Error fetching tickets: %v", err), "warning")
			broadcastStatusUpdates(ps.sseServer, "zendesk", "error", "Error fetching tickets")
		}
		result.Error = err
		return result
	}

	result.SLATicketsCount = len(slaTickets)
	result.UpdatedTicketsCount = len(newUpdatedTickets)
	log.Printf("Fetched %d SLA tickets and %d new/updated tickets", len(slaTickets), len(newUpdatedTickets))

	// Deduplicate tickets by ID (SLA tickets may overlap with new/updated tickets)
	allTickets := deduplicateTickets(slaTickets, newUpdatedTickets)
	if len(allTickets) == 0 {
		log.Println("No tickets to process")
	} else {
		processTickets(ctx, ps.db, allTickets, slaData, ps.sseServer, ps.slackService, zendeskClient, ps.pollingState)
		result.ProcessedTicketsCount = len(allTickets)
	}

	// Clear expired SLA cache entries
	if err := models.ClearExpiredSLAAlertCache(ctx, ps.db); err != nil {
		log.Printf("Failed to clear expired SLA cache: %v", err)
	}

	// Update polling state
	ps.pollingState.lastPollTime = ps.clock.Now()

	return result
}

// StartPolling runs the polling loop. This is the entry point for production use.
func (ps *PollingService) StartPolling(ctx context.Context) {
	if ps.sseServer != nil {
		broadcastStatusUpdates(ps.sseServer, "zendesk", "connected", "")
	}

	pollInterval := 5 * time.Minute

	for {
		result := ps.PollOnce(ctx)
		if result.Error != nil {
			log.Printf("Polling error: %v", result.Error)
		}

		select {
		case <-ctx.Done():
			log.Println("Polling stopped (context cancelled)")
			return
		case <-time.After(pollInterval):
		}
	}
}

// GetLastPollTime returns the last poll time for testing.
func (ps *PollingService) GetLastPollTime() time.Time {
	return ps.pollingState.lastPollTime
}

// SetLastPollTime sets the last poll time for testing.
func (ps *PollingService) SetLastPollTime(t time.Time) {
	ps.pollingState.lastPollTime = t
}

// deduplicateTickets merges multiple ticket slices and removes duplicates by ID,
// keeping the first occurrence.
func deduplicateTickets(slices ...[]zendesk.Ticket) []zendesk.Ticket {
	seen := make(map[int64]bool)
	var result []zendesk.Ticket
	for _, tickets := range slices {
		for _, t := range tickets {
			if !seen[t.ID] {
				seen[t.ID] = true
				result = append(result, t)
			}
		}
	}
	return result
}
