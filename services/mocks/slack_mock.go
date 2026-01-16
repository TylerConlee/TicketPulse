// Package mocks provides mock implementations for testing.
package mocks

import (
	"github.com/TylerConlee/TicketPulse/services"
	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/stretchr/testify/mock"
)

// MockSlackService is a mock implementation of SlackServiceInterface.
type MockSlackService struct {
	mock.Mock
}

// IsReady mocks the IsReady method.
func (m *MockSlackService) IsReady() bool {
	args := m.Called()
	return args.Bool(0)
}

// GetConversations mocks the GetConversations method.
func (m *MockSlackService) GetConversations() ([]interface{}, error) {
	args := m.Called()
	conversations := args.Get(0)
	if conversations == nil {
		return nil, args.Error(1)
	}
	return conversations.([]interface{}), args.Error(1)
}

// SendAlert mocks the SendAlert method.
func (m *MockSlackService) SendAlert(channelID, message string) error {
	args := m.Called(channelID, message)
	return args.Error(0)
}

// SendSlackMessage mocks the SendSlackMessage method.
func (m *MockSlackService) SendSlackMessage(channelID, alertType, slaLabel string, ticket zendesk.Ticket, slaInfo *services.SLAInfo, alertTag string, color string) error {
	args := m.Called(channelID, alertType, slaLabel, ticket, slaInfo, alertTag, color)
	return args.Error(0)
}

// GetUserIDByEmail mocks the GetUserIDByEmail method.
func (m *MockSlackService) GetUserIDByEmail(email string) (string, error) {
	args := m.Called(email)
	return args.String(0), args.Error(1)
}

// StartSocketMode mocks the StartSocketMode method.
func (m *MockSlackService) StartSocketMode() {
	m.Called()
}

// Ensure MockSlackService implements SlackServiceInterface.
var _ services.SlackServiceInterface = (*MockSlackService)(nil)
