package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type SlackService struct {
	client     *slack.Client
	socketMode *socketmode.Client
	ready      bool
	sseServer  *middlewares.SSEServer
}

// SlackMessage represents a Slack Block Kit message payload.
type SlackMessage struct {
	Channel string  `json:"channel"`
	Blocks  []Block `json:"blocks"`
}

// Block represents a single block in the Block Kit message.
type Block struct {
	Type   string  `json:"type"`
	Text   *Text   `json:"text,omitempty"`
	Fields []Text  `json:"fields,omitempty"`
	Action *Action `json:"accessory,omitempty"`
}

// Text represents text in a Block Kit block.
type Text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Action represents an interactive element in a Block Kit block.
type Action struct {
	Type  string `json:"type"`
	Text  Text   `json:"text"`
	Style string `json:"style"`
	Value string `json:"value"`
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
	broadcastStatusUpdates(sseServer, "slack", "connected", "")
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
	// Create a new footer block with the acknowledgment text
	acknowledgmentBlock := slack.NewContextBlock(
		"acknowledged-footer",
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("Ticket acknowledged by <@%s> at %s", callback.User.ID, time.Now().Format(time.RFC1123)), false, false),
	)

	// Initialize a new slice to store the blocks
	var newBlocks []slack.Block

	// Iterate over the existing blocks to keep only the first section block
	for _, block := range callback.Message.Blocks.BlockSet {
		// If it's a section block and newBlocks is empty, keep it
		if sectionBlock, ok := block.(*slack.SectionBlock); ok {
			if len(newBlocks) == 0 {
				newBlocks = append(newBlocks, sectionBlock)
			}
		}
	}

	// Append the new acknowledgment block
	newBlocks = append(newBlocks, acknowledgmentBlock)

	// Update the message with the modified blocks
	_, _, _, err := s.client.UpdateMessage(callback.Channel.ID, callback.Message.Timestamp, slack.MsgOptionBlocks(newBlocks...))
	if err != nil {
		log.Printf("Failed to update message in channel %s at %s: %v", callback.Channel.ID, callback.Message.Timestamp, err)
	}
}

func (s *SlackService) SendSlackMessage(channelID, alertType, slaLabel string, ticket zendesk.Ticket, slaInfo *SLAInfo, alertTag string) error {
	// Fetch Zendesk subdomain for ticket URL
	zendeskSubdomain, err := models.GetConfiguration("zendesk_subdomain")
	if err != nil || zendeskSubdomain == "" {
		return fmt.Errorf("failed to retrieve Zendesk subdomain")
	}
	ticketURL := fmt.Sprintf("https://%s.zendesk.com/agent/tickets/%d", zendeskSubdomain, ticket.ID)

	// Retrieve Zendesk configuration from the database
	subdomain, _ := models.GetConfiguration("zendesk_subdomain")
	email, _ := models.GetConfiguration("zendesk_email")
	apiToken, _ := models.GetConfiguration("zendesk_api_key")

	// Create a new Zendesk client
	zc := NewZendeskClient(subdomain, email, apiToken)

	// Get requester information
	requesterName := "Unknown Requester"
	requester, err := zc.GetRequesterByID(ticket.RequesterID)
	if err != nil {
		log.Printf("Failed to retrieve requester name for Ticket #%d: %v", ticket.ID, err)
	} else {
		requesterName = requester.Name
	}

	// Get organization information
	organizationName := "Unknown Organization"
	if ticket.OrganizationID > 0 {
		org, err := zc.GetOrganizationByID(ticket.OrganizationID)
		if err != nil {
			log.Printf("Failed to retrieve organization name for Ticket #%d: %v", ticket.ID, err)
		} else {
			organizationName = org.Name
		}
	}

	// Determine the SLA expiration time if present
	slaExpiration := ""
	if slaInfo != nil && len(slaInfo.PolicyMetrics) > 0 {
		slaExpiration = slaInfo.PolicyMetrics[0].BreachAt.Format("2006-01-02 15:04")
	}

	// Determine the message content based on the alert type
	var alertHeader, alertDescription string
	switch alertType {
	case "new_ticket":
		alertHeader = "*New Ticket Alert*"
		alertDescription = fmt.Sprintf("A new ticket has been created: *%s*", ticket.Subject)
	case "ticket_update":
		alertHeader = "*Ticket Update Alert*"
		alertDescription = fmt.Sprintf("An update has been made to the ticket: *%s*", ticket.Subject)
	case "sla_deadline":
		alertHeader = "*SLA Breach Warning*"
		alertDescription = fmt.Sprintf("%s for SLA on the ticket: %d", slaLabel, ticket.ID)
	default:
		alertHeader = "*Ticket Alert*"
		alertDescription = fmt.Sprintf("Action required for ticket: *%s*", ticket.Subject)
	}

	// Construct the message blocks using Slack Block Kit
	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s\n%s", alertHeader, alertDescription), false, false), nil, nil),
		slack.NewSectionBlock(nil, []*slack.TextBlockObject{
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Ticket ID:*\n<%s|#%d>", ticketURL, ticket.ID), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Subject:*\n%s", ticket.Subject), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Requester:*\n%s", requesterName), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Organization:*\n%s", organizationName), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Tag:*\n%s", alertTag), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*SLA Expiration:*\n%s", slaExpiration), false, false),
		}, nil),
		slack.NewActionBlock("", slack.NewButtonBlockElement("acknowledge", fmt.Sprintf("acknowledge_%d", ticket.ID), slack.NewTextBlockObject("plain_text", "Acknowledge", false, false)).WithStyle(slack.StylePrimary)),
	}

	// Create and send the message using the Slack client
	channelID, timestamp, err := s.client.PostMessage(channelID, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %v", err)
	}

	log.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
	return nil
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

func sendSlackDM(slackService *SlackService, slackUserID string, summaryMessage string, unreadTickets []zendesk.Ticket, openTicketsWithSLA []zendesk.Ticket, csatRatings []SatisfactionRating, slaData map[int64]SLAInfo) error {
	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", "*Your Daily Summary*", false, false), nil, nil),
		slack.NewDividerBlock(),
	}

	// Add Unread Tickets Block
	if len(unreadTickets) > 0 {
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", "*Unread Tickets*:", false, false), nil, nil))
		for _, ticket := range unreadTickets {
			ticketInfo := fmt.Sprintf("• *%s* (ID: %d)\n", ticket.Subject, ticket.ID)
			blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", ticketInfo, false, false), nil, nil))
		}
		blocks = append(blocks, slack.NewDividerBlock())
	}

	// Add Open Tickets with SLA Block
	if len(openTicketsWithSLA) > 0 {
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", "*Open Tickets with Active SLAs*:", false, false), nil, nil))
		for _, ticket := range openTicketsWithSLA {
			slaLabel := getSLALabel(ticket, slaData)
			ticketInfo := fmt.Sprintf("• *%s* (ID: %d) - SLA: %s\n", ticket.Subject, ticket.ID, slaLabel)
			blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", ticketInfo, false, false), nil, nil))
		}
		blocks = append(blocks, slack.NewDividerBlock())
	}

	// Add CSAT Ratings Block
	if len(csatRatings) > 0 {
		blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", "*CSAT Ratings*:", false, false), nil, nil))
		for _, csat := range csatRatings {
			ratingInfo := fmt.Sprintf("• *%s/5* - %s\n", csat.Score, csat.Comment)
			blocks = append(blocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", ratingInfo, false, false), nil, nil))
		}
		blocks = append(blocks, slack.NewDividerBlock())
	}

	// Send the message
	_, _, err := slackService.client.PostMessage(slackUserID, slack.MsgOptionBlocks(blocks...))
	return err
}
