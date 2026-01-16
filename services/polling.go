// Package services provides core business logic for TicketPulse.
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
)

// PollingService encapsulates the Zendesk polling logic for testability.
type PollingService struct {
	db           db.Database
	sseServer    *middlewares.SSEServer
	slackService *SlackService
	clock        Clock
	pollingState *PollingState
}

// PollingResult represents the result of a single polling iteration.
type PollingResult struct {
	SLATicketsCount      int
	UpdatedTicketsCount  int
	ProcessedTicketsCount int
	Error                error
}

// NewPollingService creates a new PollingService.
func NewPollingService(db db.Database, sseServer *middlewares.SSEServer, slackService *SlackService) *PollingService {
	return &PollingService{
		db:           db,
		sseServer:    sseServer,
		slackService: slackService,
		clock:        RealClock{},
		pollingState: NewPollingState(),
	}
}

// NewPollingServiceWithClock creates a new PollingService with a custom clock for testing.
func NewPollingServiceWithClock(db db.Database, sseServer *middlewares.SSEServer, slackService *SlackService, clock Clock) *PollingService {
	return &PollingService{
		db:           db,
		sseServer:    sseServer,
		slackService: slackService,
		clock:        clock,
		pollingState: NewPollingStateWithClock(clock),
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

	// Create Zendesk client
	zendeskClient, err := NewZendeskClient(ps.db)
	if err != nil {
		if ps.sseServer != nil {
			middlewares.AddGlobalNotification(ps.sseServer, "Zendesk Configuration Error", fmt.Sprintf("Error fetching Zendesk configuration: %v", err), "danger")
			broadcastStatusUpdates(ps.sseServer, "zendesk", "error", "Error fetching Zendesk configuration")
		}
		result.Error = fmt.Errorf("failed to create Zendesk client: %w", err)
		return result
	}

	// Fetch SLA tickets
	if ps.sseServer != nil {
		middlewares.AddGlobalNotification(ps.sseServer, "Refreshing Zendesk tickets", "Requesting tickets from Zendesk", "info")
	}
	log.Println("Requesting tickets from Zendesk...")

	slaTickets, slaData, err := zendeskClient.SearchTicketsWithActiveSLA()
	if err != nil {
		if ps.sseServer != nil {
			middlewares.AddGlobalNotification(ps.sseServer, "Zendesk Connectivity Error", fmt.Sprintf("Error searching SLA tickets: %v", err), "warning")
			broadcastStatusUpdates(ps.sseServer, "zendesk", "error", "Error searching SLA tickets")
		}
		result.Error = fmt.Errorf("failed to search SLA tickets: %w", err)
		return result
	}
	result.SLATicketsCount = len(slaTickets)
	log.Println("Fetched", len(slaTickets), "SLA tickets")

	// Fetch new/updated tickets
	newUpdatedTickets, err := zendeskClient.SearchNewOrUpdatedTickets(ps.pollingState.lastPollTime)
	if err != nil {
		if ps.sseServer != nil {
			middlewares.AddGlobalNotification(ps.sseServer, "Zendesk Connectivity Error", fmt.Sprintf("Error searching new/updated tickets: %v", err), "warning")
			broadcastStatusUpdates(ps.sseServer, "zendesk", "error", "Error searching new/updated tickets")
		}
		result.Error = fmt.Errorf("failed to search new/updated tickets: %w", err)
		return result
	}
	result.UpdatedTicketsCount = len(newUpdatedTickets)
	log.Println("Fetched", len(newUpdatedTickets), "new/updated tickets")

	// Process tickets
	allTickets := append(slaTickets, newUpdatedTickets...)
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

	for {
		select {
		case <-ctx.Done():
			log.Println("Polling stopped due to context cancellation")
			return
		default:
			result := ps.PollOnce(ctx)
			if result.Error != nil {
				log.Printf("Polling error: %v", result.Error)
			}
			time.Sleep(5 * time.Minute)
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
