// Package mocks provides mock implementations for testing.
package mocks

import (
	"time"

	"github.com/TylerConlee/TicketPulse/services"
	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/stretchr/testify/mock"
)

// MockZendeskClient is a mock implementation of ZendeskClientInterface.
type MockZendeskClient struct {
	mock.Mock
}

// SearchTicketsWithActiveSLA mocks the SearchTicketsWithActiveSLA method.
func (m *MockZendeskClient) SearchTicketsWithActiveSLA() ([]zendesk.Ticket, map[int64]services.SLAInfo, error) {
	args := m.Called()
	tickets := args.Get(0)
	slaInfo := args.Get(1)
	if tickets == nil {
		return nil, nil, args.Error(2)
	}
	if slaInfo == nil {
		return tickets.([]zendesk.Ticket), nil, args.Error(2)
	}
	return tickets.([]zendesk.Ticket), slaInfo.(map[int64]services.SLAInfo), args.Error(2)
}

// SearchNewOrUpdatedTickets mocks the SearchNewOrUpdatedTickets method.
func (m *MockZendeskClient) SearchNewOrUpdatedTickets(since time.Time) ([]zendesk.Ticket, error) {
	args := m.Called(since)
	tickets := args.Get(0)
	if tickets == nil {
		return nil, args.Error(1)
	}
	return tickets.([]zendesk.Ticket), args.Error(1)
}

// GetTicketsAssignedToUser mocks the GetTicketsAssignedToUser method.
func (m *MockZendeskClient) GetTicketsAssignedToUser(userID int64, since time.Time) ([]zendesk.Ticket, error) {
	args := m.Called(userID, since)
	tickets := args.Get(0)
	if tickets == nil {
		return nil, args.Error(1)
	}
	return tickets.([]zendesk.Ticket), args.Error(1)
}

// GetUserByEmail mocks the GetUserByEmail method.
func (m *MockZendeskClient) GetUserByEmail(email string) (*zendesk.User, error) {
	args := m.Called(email)
	user := args.Get(0)
	if user == nil {
		return nil, args.Error(1)
	}
	return user.(*zendesk.User), args.Error(1)
}

// GetCSATRatingsForUser mocks the GetCSATRatingsForUser method.
func (m *MockZendeskClient) GetCSATRatingsForUser(userID int64, since time.Time) ([]services.SatisfactionRating, error) {
	args := m.Called(userID, since)
	ratings := args.Get(0)
	if ratings == nil {
		return nil, args.Error(1)
	}
	return ratings.([]services.SatisfactionRating), args.Error(1)
}

// GetRequesterByID mocks the GetRequesterByID method.
func (m *MockZendeskClient) GetRequesterByID(userID int64) (*services.User, error) {
	args := m.Called(userID)
	user := args.Get(0)
	if user == nil {
		return nil, args.Error(1)
	}
	return user.(*services.User), args.Error(1)
}

// GetOrganizationByID mocks the GetOrganizationByID method.
func (m *MockZendeskClient) GetOrganizationByID(organizationID int64) (*services.Organization, error) {
	args := m.Called(organizationID)
	org := args.Get(0)
	if org == nil {
		return nil, args.Error(1)
	}
	return org.(*services.Organization), args.Error(1)
}

// GetLastPublicCommentTime mocks the GetLastPublicCommentTime method.
func (m *MockZendeskClient) GetLastPublicCommentTime(ticketID int64) (time.Time, bool, error) {
	args := m.Called(ticketID)
	return args.Get(0).(time.Time), args.Bool(1), args.Error(2)
}

// GenerateDailySummary mocks the GenerateDailySummary method.
func (m *MockZendeskClient) GenerateDailySummary(userEmail string, slackService *services.SlackService, workDayStart, workDayEnd time.Time, timezone string, tagFilterMode, ticketFilterMode string, userTags []string) (string, error) {
	args := m.Called(userEmail, slackService, workDayStart, workDayEnd, timezone, tagFilterMode, ticketFilterMode, userTags)
	return args.String(0), args.Error(1)
}

// Ensure MockZendeskClient implements ZendeskClientInterface.
var _ services.ZendeskClientInterface = (*MockZendeskClient)(nil)
