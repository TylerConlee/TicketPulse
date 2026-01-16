package models

import (
	"testing"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupConfigTestDB(t *testing.T) *db.SQLDatabase {
	database := db.InitDB(":memory:")
	require.NotNil(t, database, "Failed to initialize test database")
	return database
}

func TestSetConfiguration(t *testing.T) {
	database := setupConfigTestDB(t)
	defer database.Close()

	err := SetConfiguration(database, "zendesk_subdomain", "mycompany")
	assert.NoError(t, err)

	value, err := GetConfiguration(database, "zendesk_subdomain")
	assert.NoError(t, err)
	assert.Equal(t, "mycompany", value)
}

func TestSetConfiguration_Update(t *testing.T) {
	database := setupConfigTestDB(t)
	defer database.Close()

	// Set initial value
	err := SetConfiguration(database, "zendesk_subdomain", "oldcompany")
	require.NoError(t, err)

	// Update value
	err = SetConfiguration(database, "zendesk_subdomain", "newcompany")
	assert.NoError(t, err)

	value, err := GetConfiguration(database, "zendesk_subdomain")
	assert.NoError(t, err)
	assert.Equal(t, "newcompany", value)
}

func TestGetConfiguration(t *testing.T) {
	database := setupConfigTestDB(t)
	defer database.Close()

	err := SetConfiguration(database, "slack_bot_token", "xoxb-test-token")
	require.NoError(t, err)

	value, err := GetConfiguration(database, "slack_bot_token")
	assert.NoError(t, err)
	assert.Equal(t, "xoxb-test-token", value)
}

func TestGetConfiguration_NotFound(t *testing.T) {
	database := setupConfigTestDB(t)
	defer database.Close()

	value, err := GetConfiguration(database, "nonexistent_key")
	assert.NoError(t, err)
	assert.Empty(t, value)
}

func TestGetAllConfigurations(t *testing.T) {
	database := setupConfigTestDB(t)
	defer database.Close()

	// Set multiple configurations
	err := SetConfiguration(database, "zendesk_subdomain", "mycompany")
	require.NoError(t, err)
	err = SetConfiguration(database, "slack_bot_token", "xoxb-test-token")
	require.NoError(t, err)
	err = SetConfiguration(database, "polling_interval", "300")
	require.NoError(t, err)

	configs, err := GetAllConfigurations(database)
	assert.NoError(t, err)
	assert.Len(t, configs, 3)
	assert.Equal(t, "mycompany", configs["zendesk_subdomain"])
	assert.Equal(t, "xoxb-test-token", configs["slack_bot_token"])
	assert.Equal(t, "300", configs["polling_interval"])
}

func TestGetAllConfigurations_Empty(t *testing.T) {
	database := setupConfigTestDB(t)
	defer database.Close()

	configs, err := GetAllConfigurations(database)
	assert.NoError(t, err)
	assert.Empty(t, configs)
}

func TestSetConfiguration_EmptyValue(t *testing.T) {
	database := setupConfigTestDB(t)
	defer database.Close()

	err := SetConfiguration(database, "optional_field", "")
	assert.NoError(t, err)

	value, err := GetConfiguration(database, "optional_field")
	assert.NoError(t, err)
	assert.Empty(t, value)
}

func TestSetConfiguration_SpecialCharacters(t *testing.T) {
	database := setupConfigTestDB(t)
	defer database.Close()

	// Test with special characters in value
	err := SetConfiguration(database, "webhook_url", "https://hooks.slack.com/services/T00/B00/XXXX")
	assert.NoError(t, err)

	value, err := GetConfiguration(database, "webhook_url")
	assert.NoError(t, err)
	assert.Equal(t, "https://hooks.slack.com/services/T00/B00/XXXX", value)
}

func TestSetConfiguration_LongValue(t *testing.T) {
	database := setupConfigTestDB(t)
	defer database.Close()

	// Test with a long value (like an API key or token)
	longValue := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	err := SetConfiguration(database, "api_token", longValue)
	assert.NoError(t, err)

	value, err := GetConfiguration(database, "api_token")
	assert.NoError(t, err)
	assert.Equal(t, longValue, value)
}
