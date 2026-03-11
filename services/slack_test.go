package services

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
)

func TestSlackMessage_JSON(t *testing.T) {
	msg := SlackMessage{
		Channel: "C12345",
		Blocks: []Block{
			{
				Type: "section",
				Text: &Text{
					Type: "mrkdwn",
					Text: "Hello, World!",
				},
			},
		},
	}

	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	var decoded SlackMessage
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, "C12345", decoded.Channel)
	assert.Len(t, decoded.Blocks, 1)
	assert.Equal(t, "section", decoded.Blocks[0].Type)
	assert.Equal(t, "Hello, World!", decoded.Blocks[0].Text.Text)
}

func TestBlock_WithFields(t *testing.T) {
	block := Block{
		Type: "section",
		Fields: []Text{
			{Type: "mrkdwn", Text: "*Field 1:*\nValue 1"},
			{Type: "mrkdwn", Text: "*Field 2:*\nValue 2"},
		},
	}

	assert.Equal(t, "section", block.Type)
	assert.Len(t, block.Fields, 2)
	assert.Equal(t, "*Field 1:*\nValue 1", block.Fields[0].Text)
	assert.Equal(t, "*Field 2:*\nValue 2", block.Fields[1].Text)
}

func TestBlock_WithAction(t *testing.T) {
	block := Block{
		Type: "section",
		Text: &Text{Type: "mrkdwn", Text: "Action block"},
		Action: &Action{
			Type:  "button",
			Text:  Text{Type: "plain_text", Text: "Click Me"},
			Style: "primary",
			Value: "button_clicked",
		},
	}

	assert.NotNil(t, block.Action)
	assert.Equal(t, "button", block.Action.Type)
	assert.Equal(t, "Click Me", block.Action.Text.Text)
	assert.Equal(t, "primary", block.Action.Style)
	assert.Equal(t, "button_clicked", block.Action.Value)
}

func TestText_Types(t *testing.T) {
	tests := []struct {
		name     string
		textType string
		content  string
	}{
		{
			name:     "Markdown text",
			textType: "mrkdwn",
			content:  "*Bold* and _italic_",
		},
		{
			name:     "Plain text",
			textType: "plain_text",
			content:  "Simple text without formatting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := Text{
				Type: tt.textType,
				Text: tt.content,
			}
			assert.Equal(t, tt.textType, text.Type)
			assert.Equal(t, tt.content, text.Text)
		})
	}
}

func TestSlackService_IsReady(t *testing.T) {
	tests := []struct {
		name     string
		ready    bool
		expected bool
	}{
		{
			name:     "Service is ready",
			ready:    true,
			expected: true,
		},
		{
			name:     "Service is not ready",
			ready:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &SlackService{
				ready: tt.ready,
			}
			assert.Equal(t, tt.expected, service.IsReady())
		})
	}
}

func TestSlackMessage_EmptyBlocks(t *testing.T) {
	msg := SlackMessage{
		Channel: "C12345",
		Blocks:  []Block{},
	}

	assert.Empty(t, msg.Blocks)
	assert.Equal(t, "C12345", msg.Channel)
}

func TestSlackMessage_MultipleBlocks(t *testing.T) {
	msg := SlackMessage{
		Channel: "C12345",
		Blocks: []Block{
			{
				Type: "header",
				Text: &Text{Type: "plain_text", Text: "Header"},
			},
			{
				Type: "section",
				Text: &Text{Type: "mrkdwn", Text: "Body"},
			},
			{
				Type: "divider",
			},
			{
				Type: "section",
				Fields: []Text{
					{Type: "mrkdwn", Text: "Field 1"},
					{Type: "mrkdwn", Text: "Field 2"},
				},
			},
		},
	}

	assert.Len(t, msg.Blocks, 4)
	assert.Equal(t, "header", msg.Blocks[0].Type)
	assert.Equal(t, "section", msg.Blocks[1].Type)
	assert.Equal(t, "divider", msg.Blocks[2].Type)
	assert.Equal(t, "section", msg.Blocks[3].Type)
	assert.Len(t, msg.Blocks[3].Fields, 2)
}

func TestAction_Struct(t *testing.T) {
	action := Action{
		Type:  "button",
		Text:  Text{Type: "plain_text", Text: "Acknowledge"},
		Style: "danger",
		Value: "ack_123",
	}

	data, err := json.Marshal(action)
	assert.NoError(t, err)

	var decoded Action
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, "button", decoded.Type)
	assert.Equal(t, "Acknowledge", decoded.Text.Text)
	assert.Equal(t, "danger", decoded.Style)
	assert.Equal(t, "ack_123", decoded.Value)
}

func TestSlackService_GetConversations(t *testing.T) {
	channels := []slack.Channel{
		{GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: "C111"}, Name: "general"}},
		{GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: "C222"}, Name: "random"}},
	}
	service := &SlackService{channels: channels}

	result, err := service.GetConversations()
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "C111", result[0].ID)
	assert.Equal(t, "general", result[0].Name)
}

func TestSlackService_GetConversations_Empty(t *testing.T) {
	service := &SlackService{channels: nil}

	result, err := service.GetConversations()
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestParseTicketIDFromButtonValue(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		expectedID int64
	}{
		{"valid acknowledge value", "acknowledge_12345", 12345},
		{"zero ticket", "acknowledge_0", 0},
		{"invalid format - no underscore", "acknowledge", 0},
		{"invalid format - not a number", "acknowledge_abc", 0},
		{"empty value", "", 0},
		{"multiple underscores", "acknowledge_123_extra", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ticketID int64
			if parts := strings.SplitN(tt.value, "_", 2); len(parts) == 2 {
				if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					ticketID = id
				}
			}
			assert.Equal(t, tt.expectedID, ticketID)
		})
	}
}

func TestSlackUserInfo_Struct(t *testing.T) {
	info := SlackUserInfo{
		Email:    "user@example.com",
		RealName: "Test User",
	}
	assert.Equal(t, "user@example.com", info.Email)
	assert.Equal(t, "Test User", info.RealName)
}

func TestSlackService_ZendeskClientField(t *testing.T) {
	service := &SlackService{ready: true}
	assert.Nil(t, service.ZendeskClient)

	zc := &ZendeskClient{Subdomain: "test"}
	service.ZendeskClient = zc
	assert.NotNil(t, service.ZendeskClient)
	assert.Equal(t, "test", service.ZendeskClient.Subdomain)
}
