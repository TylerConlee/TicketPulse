package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/logging"
	"github.com/TylerConlee/TicketPulse/middlewares"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type SlackService struct {
	client        *slack.Client
	socketMode    *socketmode.Client
	ready         bool
	sseServer     *middlewares.SSEServer
	DB            db.Database
	channels      []slack.Channel
	ZendeskClient *ZendeskClient
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

func NewSlackService(db db.Database, sseServer *middlewares.SSEServer) (*SlackService, error) {
	broadcastStatusUpdates(sseServer, "slack", "polling", "Connecting to Slack...")
	botToken, err := models.GetConfiguration(db, "slack_bot_token")
	if err != nil || botToken == "" {
		broadcastStatusUpdates(sseServer, "slack", "error", "Bot token not yet configured")
		return nil, fmt.Errorf("slack bot token not configured")
	}

	appToken, err := models.GetConfiguration(db, "slack_app_token")
	if err != nil || appToken == "" {
		broadcastStatusUpdates(sseServer, "slack", "error", "App token not yet configured")
		return nil, fmt.Errorf("slack app token not configured")
	}

	slackDebug := os.Getenv("SLACK_DEBUG") == "true"
	client := slack.New(
		botToken,
		slack.OptionDebug(slackDebug),
		slack.OptionAppLevelToken(appToken),
	)
	broadcastStatusUpdates(sseServer, "slack", "connected", "")
	socketMode := socketmode.New(client, socketmode.OptionDebug(slackDebug))

	params := &slack.GetConversationsParameters{
		ExcludeArchived: true,
		Types:           []string{"public_channel", "private_channel"},
		Limit:           100,
	}

	var allChannels []slack.Channel
	for {
		channels, nextCursor, err := client.GetConversations(params)
		if err != nil {
			return nil, fmt.Errorf("failed to get Slack conversations: %w", err)
		}

		allChannels = append(allChannels, channels...)

		if nextCursor == "" {
			break
		}

		params.Cursor = nextCursor
		time.Sleep(1 * time.Second)
	}
	return &SlackService{
		client:     client,
		socketMode: socketMode,
		ready:      true,
		sseServer:  sseServer,
		DB:         db,
		channels:   allChannels,
	}, nil
}

// IsReady checks if the Slack service is ready
func (s *SlackService) IsReady() bool {
	return s.ready
}

func (s *SlackService) GetConversations() ([]slack.Channel, error) {
	return s.channels, nil
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

func (s *SlackService) StartSocketMode(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in socket mode event handler: %v", r)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				log.Println("Slack socket mode event handler stopped (context cancelled)")
				return
			case evt, ok := <-s.socketMode.Events:
				if !ok {
					return
				}
				log.Printf("Socket mode event: type=%s dataType=%T", evt.Type, evt.Data)
				switch evt.Type {
				case socketmode.EventTypeInteractive:
					log.Printf("Socket mode: received interactive event")
					callback, ok := evt.Data.(slack.InteractionCallback)
					if !ok {
						log.Printf("Socket mode: interactive event data is not InteractionCallback, acking anyway")
						s.socketMode.Ack(*evt.Request)
						continue
					}
					log.Printf("Socket mode: acknowledging request %s", evt.Request.EnvelopeID)
					s.socketMode.Ack(*evt.Request)

					if callback.Type == slack.InteractionTypeBlockActions {
						if len(callback.ActionCallback.BlockActions) > 0 {
							action := callback.ActionCallback.BlockActions[0]
							log.Printf("Socket mode: block action received: actionID=%s value=%s", action.ActionID, action.Value)
							if action.ActionID == "acknowledge" {
								go s.HandleAcknowledge(callback)
							}
						}
					}
				default:
					log.Printf("Socket mode: received event type: %s", evt.Type)
				}
			}
		}
	}()

	go func() {
		<-ctx.Done()
		log.Println("Shutting down Slack socket mode...")
	}()

	log.Printf("Socket mode: calling Run() - will block until disconnected")
	if err := s.socketMode.RunContext(ctx); err != nil {
		log.Printf("Socket mode: RunContext exited with error: %v", err)
	} else {
		log.Printf("Socket mode: RunContext exited cleanly")
	}
}

func (s *SlackService) HandleAcknowledge(callback slack.InteractionCallback) {
	now := time.Now()
	slackUserID := callback.User.ID

	// Parse the ticket ID from the button value (format: "acknowledge_{ticketID}")
	var ticketID int64
	if len(callback.ActionCallback.BlockActions) > 0 {
		value := callback.ActionCallback.BlockActions[0].Value
		if parts := strings.SplitN(value, "_", 2); len(parts) == 2 {
			if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				ticketID = id
			}
		}
	}

	// Build the updated Slack message: keep all blocks except the action block
	var newBlocks []slack.Block
	for _, block := range callback.Message.Blocks.BlockSet {
		if _, ok := block.(*slack.ActionBlock); !ok {
			newBlocks = append(newBlocks, block)
		}
	}

	ackText := fmt.Sprintf("Ticket acknowledged by <@%s> at %s", slackUserID, now.Format(time.RFC1123))
	newBlocks = append(newBlocks, slack.NewContextBlock("acknowledged-footer",
		slack.NewTextBlockObject("mrkdwn", ackText, false, false),
	))

	// Attempt Zendesk operations if we have the required dependencies
	var zendeskErrors []string
	if ticketID > 0 && s.ZendeskClient != nil {
		userInfo, err := s.GetSlackUserInfo(slackUserID)
		if err != nil {
			log.Printf("HandleAcknowledge: failed to look up Slack user %s: %v", slackUserID, err)
			zendeskErrors = append(zendeskErrors, "could not resolve Slack user email")
		} else {
			// Look up the Zendesk user by email to get their Zendesk user ID
			zdUser, err := s.ZendeskClient.GetUserByEmail(userInfo.Email)
			if err != nil {
				log.Printf("HandleAcknowledge: failed to find Zendesk user for %s: %v", userInfo.Email, err)
				zendeskErrors = append(zendeskErrors, "Zendesk user not found")
			} else {
				if err := s.ZendeskClient.AssignTicket(ticketID, zdUser.ID); err != nil {
					log.Printf("HandleAcknowledge: failed to assign ticket %d: %v", ticketID, err)
					zendeskErrors = append(zendeskErrors, "ticket assignment failed")
				}
			}

			// Look up alert type from the database for the internal note
			alertType := "alert"
			if alertLog, err := models.GetMostRecentAlertLogByTicketID(s.DB, ticketID); err == nil {
				alertType = alertLog.AlertType
			}

			noteBody := fmt.Sprintf("%s has acknowledged a TicketPulse alert for %s at %s",
				userInfo.RealName, alertType, now.Format(time.RFC1123))
			if err := s.ZendeskClient.AddInternalNote(ticketID, noteBody); err != nil {
				log.Printf("HandleAcknowledge: failed to add internal note to ticket %d: %v", ticketID, err)
				zendeskErrors = append(zendeskErrors, "internal note failed")
			}
		}
	}

	// If any Zendesk operations failed, add a warning block
	if len(zendeskErrors) > 0 {
		warningText := fmt.Sprintf(":warning: Zendesk update incomplete: %s", strings.Join(zendeskErrors, ", "))
		newBlocks = append(newBlocks, slack.NewContextBlock("zendesk-warning",
			slack.NewTextBlockObject("mrkdwn", warningText, false, false),
		))
	}

	_, _, _, updateErr := s.client.UpdateMessage(callback.Channel.ID, callback.Message.Timestamp, slack.MsgOptionBlocks(newBlocks...))
	if updateErr != nil {
		log.Printf("Failed to update message in channel %s at %s: %v", callback.Channel.ID, callback.Message.Timestamp, updateErr)
	}

	// Record acknowledgment in the database
	if ticketID > 0 && s.DB != nil {
		ackEntry := models.AcknowledgmentLog{
			TicketID:      ticketID,
			SlackUserID:   slackUserID,
			SlackUserName: callback.User.Name,
		}
		if alertLog, err := models.GetMostRecentAlertLogByTicketID(s.DB, ticketID); err == nil {
			ackEntry.AlertLogID = &alertLog.ID
		}
		if err := models.CreateAcknowledgmentLog(context.Background(), s.DB, ackEntry); err != nil {
			log.Printf("HandleAcknowledge: failed to record acknowledgment for ticket %d: %v", ticketID, err)
		}
	}
}

// SendSlackMessage creates a temporary ZendeskClient for backward compatibility.
// Prefer SendSlackMessageWithClient when a ZendeskClient is already available.
func (s *SlackService) SendSlackMessage(channelID, alertType, slaLabel string, ticket zendesk.Ticket, slaInfo *SLAInfo, alertTag string, color string) error {
	zc, err := NewZendeskClient(s.DB)
	if err != nil {
		return fmt.Errorf("failed to create Zendesk client: %v", err)
	}
	return s.SendSlackMessageWithClient(channelID, alertType, slaLabel, ticket, slaInfo, alertTag, color, zc)
}

// SendSlackMessageWithClient sends a Slack alert, reusing the provided ZendeskClient
// for requester/organization lookups (which benefit from the client's built-in cache).
func (s *SlackService) SendSlackMessageWithClient(channelID, alertType, slaLabel string, ticket zendesk.Ticket, slaInfo *SLAInfo, alertTag string, color string, zc *ZendeskClient) error {
	logging.Debug(logging.AreaSlack, "SendSlackMessage: channelID=%s alertType=%s slaLabel=%q ticketID=%d tag=%s color=%s",
		channelID, alertType, slaLabel, ticket.ID, alertTag, color)

	ticketURL := fmt.Sprintf("https://%s.zendesk.com/agent/tickets/%d", zc.Subdomain, ticket.ID)

	requesterName := "Unknown Requester"
	requester, err := zc.GetRequesterByID(ticket.RequesterID)
	if err != nil {
		log.Printf("Failed to retrieve requester name for Ticket #%d: %v", ticket.ID, err)
	} else {
		requesterName = requester.Name
	}

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

	// Map ticket priority levels to image URLs for color representation
	var priorityColorImages = map[string]string{
		"low":    "https://singlecolorimage.com/get/3498DB/16x16", // Blue
		"normal": "https://singlecolorimage.com/get/F1C40F/16x16", // Yellow
		"high":   "https://singlecolorimage.com/get/FFA500/16x16", // Orange
		"urgent": "https://singlecolorimage.com/get/FF0000/16x16", // Red
	}

	// Map SLA alert levels to image URLs for color representation
	var slaColorImages = map[string]string{
		"#3498DB": "https://singlecolorimage.com/get/3498DB/16x16", // Blue (3 hours remaining)
		"#F1C40F": "https://singlecolorimage.com/get/F1C40F/16x16", // Yellow (2 hours remaining)
		"#FFA500": "https://singlecolorimage.com/get/FFA500/16x16", // Orange (1 hour remaining)
		"#FF8C00": "https://singlecolorimage.com/get/FF8C00/16x16", // Darker Orange (30 minutes remaining)
		"#FF0000": "https://singlecolorimage.com/get/FF0000/16x16", // Red (15 minutes remaining / breached)
	}

	// Determine the message content based on the alert type
	var alertHeader, alertDescription string
	switch alertType {
	case "new_ticket":
		alertHeader = ":admission_tickets: *New Ticket Alert*"
		alertDescription = fmt.Sprintf("*<%s|%s>* (#%d) was opened by %s from *%s*.", ticketURL, ticket.Subject, ticket.ID, requesterName, organizationName)
	case "ticket_update":
		alertHeader = ":memo: *Ticket Update Alert*"
		alertDescription = fmt.Sprintf("*<%s|%s>* (#%d) was updated by %s from *%s*.", ticketURL, ticket.Subject, ticket.ID, requesterName, organizationName)
	case "sla_deadline":
		// Generic SLA deadline - shouldn't typically be used now
		alertHeader = ":rotating_light: *SLA Breach Warning*"
		alertDescription = fmt.Sprintf("%s for SLA on *<%s|%s>* (#%d). ", slaLabel, ticketURL, ticket.Subject, ticket.ID)
	case "sla_reply":
		// Reply SLA - check if breached or warning
		if strings.Contains(slaLabel, "BREACHED") {
			alertHeader = ":rotating_light: *Reply SLA BREACHED*"
			alertDescription = fmt.Sprintf("*Reply SLA has been breached* on *<%s|%s>* (#%d). Immediate response required!", ticketURL, ticket.Subject, ticket.ID)
		} else {
			alertHeader = ":warning: *Reply SLA Warning*"
			alertDescription = fmt.Sprintf("*%s* for Reply SLA on *<%s|%s>* (#%d). ", slaLabel, ticketURL, ticket.Subject, ticket.ID)
		}
	case "sla_resolution":
		// Resolution SLA - check if breached or warning
		if strings.Contains(slaLabel, "BREACHED") {
			alertHeader = ":rotating_light: *Resolution SLA BREACHED*"
			alertDescription = fmt.Sprintf("*Resolution SLA has been breached* on *<%s|%s>* (#%d). Immediate action required!", ticketURL, ticket.Subject, ticket.ID)
		} else {
			alertHeader = ":warning: *Resolution SLA Warning*"
			alertDescription = fmt.Sprintf("*%s* for Resolution SLA on *<%s|%s>* (#%d). ", slaLabel, ticketURL, ticket.Subject, ticket.ID)
		}
	default:
		alertHeader = ":ticket: *Ticket Alert*"
		alertDescription = fmt.Sprintf("Action required for ticket: *%s*", ticket.Subject)
	}

	// Construct the message blocks using Slack Block Kit
	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s\n%s", alertHeader, alertDescription), false, false), nil, nil),
	}

	// Create a context block with priority and SLA expiration details
	contextElements := []slack.MixedElement{}

	priorityImageURL := priorityColorImages[strings.ToLower(ticket.Priority)]
	slaImageURL, slaImageExists := slaColorImages[color]

	// Add priority image if available
	if priorityImageURL != "" {
		contextElements = append(contextElements, slack.NewImageBlockElement(priorityImageURL, "Priority Level"))
	}

	// Add SLA image if applicable
	isSLAAlert := alertType == "sla_deadline" || alertType == "sla_reply" || alertType == "sla_resolution"
	if isSLAAlert && slaImageExists {
		contextElements = append(contextElements, slack.NewImageBlockElement(slaImageURL, "SLA Urgency Level"))
	}

	// Add text for priority and SLA expiration
	var contextText string
	if isSLAAlert && strings.Contains(slaLabel, "BREACHED") {
		contextText = fmt.Sprintf("*Ticket Priority:* %s | *SLA Breached at:* %s", ticket.Priority, slaExpiration)
	} else if isSLAAlert {
		contextText = fmt.Sprintf("*Ticket Priority:* %s | *SLA Expires at:* %s", ticket.Priority, slaExpiration)
	} else {
		contextText = fmt.Sprintf("*Ticket Priority:* %s", ticket.Priority)
	}
	contextElements = append(contextElements, slack.NewTextBlockObject("mrkdwn", contextText, false, false))

	// Add the context block
	blocks = append(blocks, slack.NewContextBlock("", contextElements...))

	// Add the acknowledgment button at the end
	blocks = append(blocks, slack.NewActionBlock("", slack.NewButtonBlockElement("acknowledge", fmt.Sprintf("acknowledge_%d", ticket.ID), slack.NewTextBlockObject("plain_text", "Acknowledge", false, false)).WithStyle(slack.StylePrimary)))

	// Create and send the message using the Slack client
	logging.Debug(logging.AreaSlack, "SendSlackMessage: posting to channel=%s with %d blocks", channelID, len(blocks))
	channelID, timestamp, err := s.client.PostMessage(channelID, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		logging.Debug(logging.AreaSlack, "SendSlackMessage: FAILED - Slack API error: %v", err)
		return fmt.Errorf("failed to send Slack message: %v", err)
	}

	log.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
	logging.Debug(logging.AreaSlack, "SendSlackMessage: SUCCESS - channel=%s timestamp=%s", channelID, timestamp)
	return nil
}

// PostBlockMessage posts a Block Kit message to the given channel.
func (s *SlackService) PostBlockMessage(channelID string, blocks ...slack.Block) error {
	_, _, err := s.client.PostMessage(channelID, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		return fmt.Errorf("failed to post block message: %w", err)
	}
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

// SlackUserInfo holds resolved Slack user details needed for acknowledge flows.
type SlackUserInfo struct {
	Email    string
	RealName string
}

// GetSlackUserInfo retrieves a Slack user's email and real name by their Slack user ID.
func (s *SlackService) GetSlackUserInfo(slackUserID string) (*SlackUserInfo, error) {
	user, err := s.client.GetUserInfo(slackUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Slack user info for %s: %w", slackUserID, err)
	}
	email := user.Profile.Email
	if email == "" {
		return nil, fmt.Errorf("no email found for Slack user %s", slackUserID)
	}
	realName := user.RealName
	if realName == "" {
		realName = user.Name
	}
	return &SlackUserInfo{Email: email, RealName: realName}, nil
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

func sendSlackDM(slackService *SlackService, slackUserID string, unreadTickets []zendesk.Ticket, openTicketsWithSLA []zendesk.Ticket, csatRatings []SatisfactionRating, slaData map[int64]SLAInfo) error {
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
			slaLabel := GetSLALabel(ticket, slaData)
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
