package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/models"
	"github.com/slack-go/slack"
)

var skipReasonLabels = map[string]string{
	models.SkipReasonSLADuplicate:           "SLA Alerts Already Received (Deduplicated)",
	models.SkipReasonNewTicketOutsideWindow:  "New Ticket Alerts Skipped (Outside Window)",
	models.SkipReasonUpdateNoEndUserComment:  "Update Alerts Skipped (No End-User Comment)",
	models.SkipReasonUpdateNotModified:       "Update Alerts Skipped (Not Modified)",
	models.SkipReasonSLAMetricMismatch:       "SLA Alerts Skipped (Metric Mismatch)",
	models.SkipReasonSLAStageInactive:        "SLA Alerts Skipped (Inactive Stage)",
}

func sendDailyAlertLog(ctx context.Context, database db.Database, slackService *SlackService, configCache *ConfigCache) error {
	channelID, err := configCache.Get("daily_alert_log_channel_id")
	if err != nil || channelID == "" {
		return fmt.Errorf("daily alert log channel not configured")
	}

	tz, err := configCache.Get("daily_alert_log_timezone")
	if err != nil || tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	yesterday := now.AddDate(0, 0, -1)
	dateStr := yesterday.Format("2006-01-02")

	existing, _ := models.GetDailyAlertLogSent(ctx, database, dateStr)
	if existing != nil {
		log.Printf("Daily alert log already sent for %s, skipping", dateStr)
		return nil
	}

	zendeskSubdomain, _ := configCache.Get("zendesk_subdomain")

	summary, err := models.GetDailyAlertSummary(ctx, database, dateStr)
	if err != nil {
		return fmt.Errorf("failed to get daily alert summary: %w", err)
	}

	blocks := buildDailyAlertLogBlocks(summary, dateStr, zendeskSubdomain)

	if err := slackService.PostBlockMessage(channelID, blocks...); err != nil {
		return fmt.Errorf("failed to post daily alert log: %w", err)
	}

	if err := models.CreateDailyAlertLogSent(ctx, database, dateStr); err != nil {
		log.Printf("Failed to record daily alert log sent for %s: %v", dateStr, err)
	}

	log.Printf("Daily alert log posted for %s", dateStr)
	return nil
}

func buildDailyAlertLogBlocks(summary *models.DailyAlertSummary, dateStr, zendeskSubdomain string) []slack.Block {
	var blocks []slack.Block

	displayDate := dateStr
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		displayDate = t.Format("January 2, 2006")
	}

	// Header
	blocks = append(blocks, slack.NewHeaderBlock(
		slack.NewTextBlockObject("plain_text", fmt.Sprintf("Daily Alert Log - %s", displayDate), false, false),
	))

	// Summary line
	totalSkipped := 0
	for _, sg := range summary.Skipped {
		totalSkipped += sg.Count
	}

	summaryText := fmt.Sprintf(
		"*Total Alerts:* %d  |  *Acknowledged:* %d  |  *Skipped:* %d",
		summary.TotalAlerts, summary.Acknowledgments.TotalAcknowledged, totalSkipped,
	)

	if summary.PreviousDay > 0 {
		diff := summary.TotalAlerts - summary.PreviousDay
		pct := float64(diff) / float64(summary.PreviousDay) * 100
		sign := "+"
		if pct < 0 {
			sign = ""
		}
		summaryText += fmt.Sprintf("\n*Trend:* %s%.0f%% vs previous day (%d alerts)", sign, pct, summary.PreviousDay)
	} else if summary.PreviousDay == 0 && summary.TotalAlerts > 0 {
		summaryText += "\n*Trend:* No alerts on previous day"
	}

	blocks = append(blocks, slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", summaryText, false, false), nil, nil,
	))

	blocks = append(blocks, slack.NewDividerBlock())

	// By Type
	if len(summary.ByType) > 0 {
		var lines []string
		for _, tc := range summary.ByType {
			lines = append(lines, fmt.Sprintf("  %s: %d", tc.AlertType, tc.Count))
		}
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*Alerts by Type*\n"+strings.Join(lines, "\n"), false, false), nil, nil,
		))
	}

	// By Channel
	if len(summary.ByChannel) > 0 {
		var lines []string
		for _, cc := range summary.ByChannel {
			lines = append(lines, fmt.Sprintf("  #%s: %d", cc.ChannelName, cc.Count))
		}
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*Alerts by Channel*\n"+strings.Join(lines, "\n"), false, false), nil, nil,
		))
	}

	// By Tag
	if len(summary.ByTag) > 0 {
		var lines []string
		for _, tc := range summary.ByTag {
			lines = append(lines, fmt.Sprintf("  %s: %d", tc.Tag, tc.Count))
		}
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*Alerts by Tag*\n"+strings.Join(lines, "\n"), false, false), nil, nil,
		))
	}

	blocks = append(blocks, slack.NewDividerBlock())

	// Top tickets
	if len(summary.TopTickets) > 0 {
		var lines []string
		for _, tt := range summary.TopTickets {
			link := ticketLink(zendeskSubdomain, tt.TicketID)
			lines = append(lines, fmt.Sprintf("  %s: %d alerts", link, tt.Count))
		}
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*Top Tickets*\n"+strings.Join(lines, "\n"), false, false), nil, nil,
		))
	}

	blocks = append(blocks, slack.NewDividerBlock())

	// Acknowledgment stats
	ackPct := 0.0
	if summary.TotalAlerts > 0 {
		ackPct = float64(summary.Acknowledgments.TotalAcknowledged) / float64(summary.TotalAlerts) * 100
	}
	avgAckStr := formatDuration(summary.Acknowledgments.AvgAckSeconds)
	ackText := fmt.Sprintf(
		"*Acknowledgment Stats*\n  Acknowledged: %d / %d (%.0f%%)\n  Avg time to acknowledge: %s",
		summary.Acknowledgments.TotalAcknowledged, summary.TotalAlerts, ackPct, avgAckStr,
	)
	blocks = append(blocks, slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", ackText, false, false), nil, nil,
	))

	blocks = append(blocks, slack.NewDividerBlock())

	// Skipped alerts by reason
	for _, sg := range summary.Skipped {
		heading := skipReasonLabels[sg.SkipReason]
		if heading == "" {
			heading = sg.SkipReason
		}

		var ticketParts []string
		for i, tid := range sg.TicketIDs {
			link := ticketLink(zendeskSubdomain, tid)
			if i < len(sg.Labels) && sg.Labels[i] != "" {
				link += " - " + sg.Labels[i]
			}
			ticketParts = append(ticketParts, link)
		}

		detail := strings.Join(ticketParts, ", ")
		if len(detail) > 2900 {
			detail = detail[:2900] + "..."
		}

		text := fmt.Sprintf("*%s*\n%d skipped: %s", heading, sg.Count, detail)
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil,
		))
	}

	if len(summary.Skipped) == 0 {
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*Skipped Alerts*\nNone", false, false), nil, nil,
		))
	}

	return blocks
}

func ticketLink(subdomain string, ticketID int64) string {
	if subdomain == "" {
		return fmt.Sprintf("#%d", ticketID)
	}
	return fmt.Sprintf("<%s|#%d>",
		fmt.Sprintf("https://%s.zendesk.com/agent/tickets/%d", subdomain, ticketID),
		ticketID,
	)
}

func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return "N/A"
	}
	totalSec := int(math.Round(seconds))
	if totalSec < 60 {
		return fmt.Sprintf("%ds", totalSec)
	}
	minutes := totalSec / 60
	secs := totalSec % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	hours := minutes / 60
	mins := minutes % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}
