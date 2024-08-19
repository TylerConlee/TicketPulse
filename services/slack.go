package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type SlackService struct {
	client     *slack.Client
	socketMode *socketmode.Client
	ready      bool
	sseServer  *middlewares.SSEServer
}

func NewSlackService(sseServer *middlewares.SSEServer) (*SlackService, error) {
	broadcastStatusUpdates(sseServer, "slack", "polling", "Connecting to Slack...")
	botToken, err := models.GetConfiguration("slack_bot_token")
	if err != nil || botToken == "" {
		broadcastStatusUpdates(sseServer, "slack", "error", "Bot token not yet configured")
		return nil, fmt.Errorf("slack bot token not configured")
	}

	appToken, err := models.GetConfiguration("slack_app_token")
	if err != nil || appToken == "" {
		broadcastStatusUpdates(sseServer, "slack", "error", "App token not yet configured")
		return nil, fmt.Errorf("slack app token not configured")
	}

	client := slack.New(
		botToken,
		slack.OptionDebug(true),
		slack.OptionAppLevelToken(appToken),
	)

	socketMode := socketmode.New(client)

	return &SlackService{
		client:     client,
		socketMode: socketMode,
		ready:      true,
		sseServer:  sseServer,
	}, nil
}

// IsReady checks if the Slack service is ready
func (s *SlackService) IsReady() bool {
	return s.ready
}

func (s *SlackService) GetConversations() ([]slack.Channel, error) {
	var allChannels []slack.Channel
	params := &slack.GetConversationsParameters{
		ExcludeArchived: true,
		Types:           []string{"public_channel", "private_channel"},
		Limit:           100,
	}

	for {
		channels, nextCursor, err := s.client.GetConversations(params)
		if err != nil {
			return nil, fmt.Errorf("failed to get Slack conversations: %w", err)
		}

		allChannels = append(allChannels, channels...)

		if nextCursor == "" {
			break
		}

		params.Cursor = nextCursor
	}

	return allChannels, nil
}

func (s *SlackService) SendAlert(channelID, message string) error {
	attachment := slack.Attachment{
		Text:       "This ticket needs attention",
		CallbackID: "ticket_alert",
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", message, false, false), nil, nil),
				slack.NewActionBlock("acknowledge_block", slack.NewButtonBlockElement("acknowledge", "acknowledge", slack.NewTextBlockObject("plain_text", "Acknowledge", false, false))),
			},
		},
	}

	_, _, err := s.client.PostMessage(
		channelID,
		slack.MsgOptionAttachments(attachment),
	)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}

func (s *SlackService) StartSocketMode() {
	broadcastStatusUpdates(s.sseServer, "slack", "connected", "")
	go func() {

		for evt := range s.socketMode.Events {

			switch evt.Type {
			case socketmode.EventTypeInteractive:
				callback, ok := evt.Data.(slack.InteractionCallback)
				if !ok {
					continue
				}

				if callback.Type == slack.InteractionTypeBlockActions {
					action := callback.ActionCallback.BlockActions[0]
					if action.ActionID == "acknowledge" {
						s.HandleAcknowledge(callback)
					}
				}

				s.socketMode.Ack(*evt.Request)
			}
		}
	}()
	s.socketMode.Run()

}

func (s *SlackService) HandleAcknowledge(callback slack.InteractionCallback) {
	response := slack.MsgOptionText(fmt.Sprintf("Ticket acknowledged by <@%s> at %s", callback.User.ID, callback.ActionTs), false)
	channelID, timestamp, _, err := s.client.UpdateMessage(callback.Channel.ID, callback.Message.Timestamp, response)
	if err != nil {
		log.Printf("Failed to update message in channel %s at %s: %v", channelID, timestamp, err)
	}
}

func (s *SlackService) SendMessage(channelID, message string) error {
	_, _, err := s.client.PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
	)
	return err
}

func (s *SlackService) GetUserIDByEmail(email string) (string, error) {
	user, err := s.client.GetUserByEmail(email)
	if err != nil {
		return "", err
	}

	if user.ID == "" {
		return "", errors.New("no Slack user ID found for the provided email")
	}

	return user.ID, nil
}

func broadcastStatusUpdates(sseServer *middlewares.SSEServer, service, status, errorMsg string) {
	// Create a map to hold the event data
	data := map[string]interface{}{
		"service": service,
		"status":  status,
		"error":   errorMsg,
	}

	// Create the event payload
	event := map[string]interface{}{
		"event": "connection-status",
		"data":  data,
	}

	// Marshal the event to JSON
	message, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal status update: %v", err)
		return
	}

	// Broadcast the JSON message to all connected clients
	sseServer.NotifyAll(string(message))
}
