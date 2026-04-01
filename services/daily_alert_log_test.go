package services

import (
	"testing"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/stretchr/testify/assert"
)

func TestBuildDailyAlertLogBlocks_Empty(t *testing.T) {
	summary := &models.DailyAlertSummary{}
	blocks := buildDailyAlertLogBlocks(summary, "2025-06-15", "mycompany")
	assert.NotEmpty(t, blocks, "should produce at least a header block")
}

func TestBuildDailyAlertLogBlocks_WithData(t *testing.T) {
	summary := &models.DailyAlertSummary{
		TotalAlerts: 42,
		PreviousDay: 37,
		ByType: []models.DailyAlertTypeCount{
			{AlertType: "new_ticket", Count: 12},
			{AlertType: "sla_reply", Count: 15},
			{AlertType: "sla_resolution", Count: 8},
			{AlertType: "ticket_update", Count: 7},
		},
		ByChannel: []models.DailyAlertChannelCount{
			{ChannelName: "support-escalations", Count: 22},
			{ChannelName: "billing-alerts", Count: 12},
		},
		ByTag: []models.DailyAlertTagCount{
			{Tag: "vip", Count: 18},
			{Tag: "billing", Count: 12},
		},
		TopTickets: []models.DailyTopTicket{
			{TicketID: 12345, Count: 4},
			{TicketID: 12346, Count: 3},
		},
		Skipped: []models.DailySkippedGroup{
			{SkipReason: models.SkipReasonSLADuplicate, Count: 3, TicketIDs: []int64{123, 456, 789}, Labels: []string{"reply_time: 1h", "reply_time: 1h", "resolution_time: 4h"}},
			{SkipReason: models.SkipReasonNewTicketOutsideWindow, Count: 5, TicketIDs: []int64{101, 102, 103, 104, 105}},
		},
		Acknowledgments: models.DailyAckStats{
			TotalAcknowledged: 18,
			AvgAckSeconds:     754,
		},
	}

	blocks := buildDailyAlertLogBlocks(summary, "2025-06-15", "mycompany")
	assert.True(t, len(blocks) > 5, "should produce multiple blocks for a full summary")
}

func TestBuildDailyAlertLogBlocks_NoSubdomain(t *testing.T) {
	summary := &models.DailyAlertSummary{
		TotalAlerts: 1,
		TopTickets:  []models.DailyTopTicket{{TicketID: 123, Count: 1}},
	}
	blocks := buildDailyAlertLogBlocks(summary, "2025-06-15", "")
	assert.NotEmpty(t, blocks)
}

func TestTicketLink_WithSubdomain(t *testing.T) {
	link := ticketLink("mycompany", 12345)
	assert.Contains(t, link, "mycompany.zendesk.com")
	assert.Contains(t, link, "#12345")
}

func TestTicketLink_NoSubdomain(t *testing.T) {
	link := ticketLink("", 12345)
	assert.Equal(t, "#12345", link)
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0, "N/A"},
		{-5, "N/A"},
		{30, "30s"},
		{90, "1m 30s"},
		{754, "12m 34s"},
		{3661, "1h 1m"},
	}
	for _, tc := range tests {
		result := formatDuration(tc.seconds)
		assert.Equal(t, tc.expected, result, "formatDuration(%f)", tc.seconds)
	}
}

func TestBuildDailyAlertLogBlocks_TrendNoPreviousDay(t *testing.T) {
	summary := &models.DailyAlertSummary{
		TotalAlerts: 10,
		PreviousDay: 0,
	}
	blocks := buildDailyAlertLogBlocks(summary, "2025-06-15", "mycompany")
	assert.NotEmpty(t, blocks)
}

func TestBuildDailyAlertLogBlocks_NoSkipped(t *testing.T) {
	summary := &models.DailyAlertSummary{
		TotalAlerts: 5,
		Skipped:     []models.DailySkippedGroup{},
	}
	blocks := buildDailyAlertLogBlocks(summary, "2025-06-15", "mycompany")
	assert.NotEmpty(t, blocks)
}
