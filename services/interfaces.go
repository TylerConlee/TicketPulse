package services

import (
	"context"
	"time"

	"github.com/nukosuke/go-zendesk/zendesk"
)

// ZendeskClientInterface defines the interface for Zendesk client operations
type ZendeskClientInterface interface {
	SearchTicketsWithActiveSLA() ([]zendesk.Ticket, map[int64]SLAInfo, error)
	SearchNewOrUpdatedTickets(since time.Time) ([]zendesk.Ticket, error)
	GetTicketsAssignedToUser(userID int64, since time.Time) ([]zendesk.Ticket, error)
	GetUserByEmail(email string) (*zendesk.User, error)
	GetCSATRatingsForUser(userID int64, since time.Time) ([]SatisfactionRating, error)
	GetRequesterByID(userID int64) (*User, error)
	GetOrganizationByID(organizationID int64) (*Organization, error)
	GetLastPublicCommentTime(ticketID int64) (time.Time, bool, error)
	GenerateDailySummary(userEmail string, slackService *SlackService, workDayStart, workDayEnd time.Time, timezone string, tagFilterMode, ticketFilterMode string, userTags []string) (string, error)
	AssignTicket(ticketID int64, assigneeID int64) error
	AddInternalNote(ticketID int64, body string) error
}

// SlackServiceInterface defines the interface for Slack operations
type SlackServiceInterface interface {
	IsReady() bool
	GetConversations() ([]interface{}, error) // Using interface{} to avoid importing slack types in interface
	SendAlert(channelID, message string) error
	SendSlackMessage(channelID, alertType, slaLabel string, ticket zendesk.Ticket, slaInfo *SLAInfo, alertTag string, color string) error
	SendSlackMessageWithClient(channelID, alertType, slaLabel string, ticket zendesk.Ticket, slaInfo *SLAInfo, alertTag string, color string, zc *ZendeskClient) error
	GetUserIDByEmail(email string) (string, error)
	StartSocketMode(ctx context.Context)
}

// SchedulerServiceInterface defines the interface for scheduler operations
type SchedulerServiceInterface interface {
	StartScheduler(ctx context.Context)
	checkAndSendSummaries(ctx context.Context) error
	sendDailySummary(ctx context.Context, user interface{}) error // Using interface{} to avoid importing models in interface
}
