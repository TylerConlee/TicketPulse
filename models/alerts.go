package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/logging"
)

type SLAAlertCache struct {
	ID         int64     `db:"id"`
	UserID     int64     `db:"user_id"`
	TicketID   int64     `db:"ticket_id"`
	AlertType  string    `db:"alert_type"`
	MetricType string    `db:"metric_type"`
	BreachAt   time.Time `db:"breach_at"`
	CreatedAt  time.Time `db:"created_at"`
	Label      string    `db:"label"`
}

// CreateSLAAlertCache inserts a new entry into the sla_alert_cache table.
func CreateSLAAlertCache(ctx context.Context, db db.Database, cacheEntry SLAAlertCache) error {
	logging.Debug(logging.AreaCache, "CreateSLAAlertCache: inserting user=%d ticket=%d alertType=%s metricType=%s label=%q breach_at=%s",
		cacheEntry.UserID, cacheEntry.TicketID, cacheEntry.AlertType, cacheEntry.MetricType, cacheEntry.Label, cacheEntry.BreachAt.Format(time.RFC3339))

	query := `
        INSERT INTO sla_alert_cache (user_id, ticket_id, alert_type, metric_type, breach_at, label)
        VALUES (?, ?, ?, ?, ?, ?)
        RETURNING id
    `
	err := db.QueryRowContext(ctx, query, cacheEntry.UserID, cacheEntry.TicketID, cacheEntry.AlertType, cacheEntry.MetricType, cacheEntry.BreachAt, cacheEntry.Label).Scan(&cacheEntry.ID)
	if err != nil {
		logging.Debug(logging.AreaCache, "CreateSLAAlertCache: FAILED - %v", err)
		return fmt.Errorf("failed to create SLA alert cache entry: %w", err)
	}

	logging.Debug(logging.AreaCache, "CreateSLAAlertCache: SUCCESS - id=%d", cacheEntry.ID)
	return nil
}

// GetSLAAlertCache retrieves an SLA alert cache entry by user, ticket, alert type, and metric type.
func GetSLAAlertCache(ctx context.Context, db db.Database, userID, ticketID int, alertType, metricType string) (*SLAAlertCache, error) {
	logging.Debug(logging.AreaCache, "GetSLAAlertCache: looking up user=%d ticket=%d alertType=%s metricType=%s", userID, ticketID, alertType, metricType)

	var cacheEntry SLAAlertCache
	query := `SELECT id, user_id, ticket_id, alert_type, metric_type, breach_at, created_at, label FROM sla_alert_cache WHERE user_id = ? AND ticket_id = ? AND alert_type = ? AND metric_type = ?`
	err := db.QueryRowContext(ctx, query, userID, ticketID, alertType, metricType).Scan(&cacheEntry.ID, &cacheEntry.UserID, &cacheEntry.TicketID, &cacheEntry.AlertType, &cacheEntry.MetricType, &cacheEntry.BreachAt, &cacheEntry.CreatedAt, &cacheEntry.Label)
	if err != nil {
		logging.Debug(logging.AreaCache, "GetSLAAlertCache: no entry found (err=%v)", err)
		return nil, err
	}

	logging.Debug(logging.AreaCache, "GetSLAAlertCache: FOUND id=%d label=%q created_at=%s",
		cacheEntry.ID, cacheEntry.Label, cacheEntry.CreatedAt.Format(time.RFC3339))
	return &cacheEntry, nil
}

// ClearSLAAlertCache deletes an SLA alert cache entry by its ID.
func ClearSLAAlertCache(ctx context.Context, db db.Database, cacheID int64) error {
	logging.Debug(logging.AreaCache, "ClearSLAAlertCache: deleting cache entry id=%d", cacheID)
	query := `DELETE FROM sla_alert_cache WHERE id = ?`
	_, err := db.ExecContext(ctx, query, cacheID)
	if err != nil {
		logging.Debug(logging.AreaCache, "ClearSLAAlertCache: FAILED - %v", err)
		return fmt.Errorf("failed to clear SLA alert cache entry: %w", err)
	}
	logging.Debug(logging.AreaCache, "ClearSLAAlertCache: SUCCESS - deleted id=%d", cacheID)
	return nil
}

// ClearSLAAlertCacheByTicket clears all cache entries for a specific ticket (useful when SLA is resolved)
func ClearSLAAlertCacheByTicket(ctx context.Context, db db.Database, ticketID int64) error {
	logging.Debug(logging.AreaCache, "ClearSLAAlertCacheByTicket: clearing all entries for ticket=%d", ticketID)
	query := `DELETE FROM sla_alert_cache WHERE ticket_id = ?`
	_, err := db.ExecContext(ctx, query, ticketID)
	if err != nil {
		logging.Debug(logging.AreaCache, "ClearSLAAlertCacheByTicket: FAILED - %v", err)
		return fmt.Errorf("failed to clear SLA alert cache entries for ticket: %w", err)
	}
	logging.Debug(logging.AreaCache, "ClearSLAAlertCacheByTicket: SUCCESS")
	return nil
}

// DailySummaryLog represents a log entry for sent daily summaries
type DailySummaryLog struct {
	ID          int64     `db:"id"`
	UserID      int64     `db:"user_id"`
	SummaryDate time.Time `db:"summary_date"`
	CreatedAt   time.Time `db:"created_at"`
}

// CreateDailySummaryLog inserts a new daily summary log entry
func CreateDailySummaryLog(ctx context.Context, db db.Database, userID int, summaryDate time.Time) error {
	query := `
		INSERT INTO daily_summary_log (user_id, summary_date)
		VALUES (?, ?)
		RETURNING id
	`
	var id int64
	err := db.QueryRowContext(ctx, query, userID, summaryDate.Format("2006-01-02")).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to create daily summary log entry: %w", err)
	}
	return nil
}

// GetDailySummaryLog retrieves a daily summary log entry for a user and date
func GetDailySummaryLog(ctx context.Context, db db.Database, userID int, summaryDate time.Time) (*DailySummaryLog, error) {
	var logEntry DailySummaryLog
	query := `SELECT id, user_id, summary_date, created_at FROM daily_summary_log WHERE user_id = ? AND summary_date = ?`
	err := db.QueryRowContext(ctx, query, userID, summaryDate.Format("2006-01-02")).Scan(&logEntry.ID, &logEntry.UserID, &logEntry.SummaryDate, &logEntry.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &logEntry, nil
}

// ClearExpiredSLAAlertCache clears SLA alert cache entries that are old (created more than 24 hours ago).
// This keeps recent breached SLA entries to prevent duplicate alerts, while cleaning up stale data.
func ClearExpiredSLAAlertCache(ctx context.Context, db db.Database) error {
	logging.Debug(logging.AreaCache, "ClearExpiredSLAAlertCache: clearing entries older than 24 hours")
	query := `DELETE FROM sla_alert_cache WHERE created_at < datetime('now', '-24 hours')`
	result, err := db.ExecContext(ctx, query)
	if err != nil {
		logging.Debug(logging.AreaCache, "ClearExpiredSLAAlertCache: FAILED - %v", err)
		return fmt.Errorf("failed to clear expired SLA alert cache entries: %w", err)
	}
	if rowsAffected, raErr := result.RowsAffected(); raErr == nil {
		logging.Debug(logging.AreaCache, "ClearExpiredSLAAlertCache: deleted %d expired entries", rowsAffected)
	}
	return nil
}

type AlertLog struct {
	ID        int64  `db:"id"`
	UserID    int64  `db:"user_id"`
	TicketID  int64  `db:"ticket_id"`
	Tag       string `db:"tag"`
	AlertType string `db:"alert_type"`
	Timestamp string `db:"timestamp"`
}

// CreateAlertLog inserts a new alert log entry into the database.
func CreateAlertLog(ctx context.Context, db db.Database, logEntry AlertLog) error {
	logging.Debug(logging.AreaDB, "CreateAlertLog: user=%d ticket=%d tag=%s alertType=%s",
		logEntry.UserID, logEntry.TicketID, logEntry.Tag, logEntry.AlertType)
	query := `
		INSERT INTO alert_logs (user_id, ticket_id, tag, alert_type, timestamp)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`
	err := db.QueryRowContext(ctx, query, logEntry.UserID, logEntry.TicketID, logEntry.Tag, logEntry.AlertType, logEntry.Timestamp).Scan(&logEntry.ID)
	if err != nil {
		logging.Debug(logging.AreaDB, "CreateAlertLog: FAILED - %v", err)
		return fmt.Errorf("failed to create alert log: %w", err)
	}
	logging.Debug(logging.AreaDB, "CreateAlertLog: SUCCESS - id=%d", logEntry.ID)
	return nil
}

// GetMostRecentAlertLogByTicketID returns the most recent alert log entry for a given ticket.
func GetMostRecentAlertLogByTicketID(database db.Database, ticketID int64) (*AlertLog, error) {
	var l AlertLog
	query := `SELECT id, user_id, ticket_id, tag, alert_type, timestamp FROM alert_logs WHERE ticket_id = ? ORDER BY timestamp DESC LIMIT 1`
	err := database.QueryRow(query, ticketID).Scan(&l.ID, &l.UserID, &l.TicketID, &l.Tag, &l.AlertType, &l.Timestamp)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// ClearAndReplaceCachedTags deletes all cached tags and inserts the new set atomically.
func ClearAndReplaceCachedTags(db db.Database, tags []string) error {
	tx, err := db.GetDB().Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM zendesk_tag_cache"); err != nil {
		return fmt.Errorf("failed to clear tag cache: %w", err)
	}

	if len(tags) > 0 {
		valueStrings := make([]string, len(tags))
		valueArgs := make([]interface{}, len(tags))
		for i, tag := range tags {
			valueStrings[i] = "(?)"
			valueArgs[i] = tag
		}
		query := "INSERT INTO zendesk_tag_cache (tag) VALUES " + strings.Join(valueStrings, ",")
		if _, err := tx.Exec(query, valueArgs...); err != nil {
			return fmt.Errorf("failed to insert cached tags: %w", err)
		}
	}

	return tx.Commit()
}

// GetCachedTags returns all cached Zendesk tags, ordered alphabetically.
func GetCachedTags(db db.Database) ([]string, error) {
	rows, err := db.Query("SELECT tag FROM zendesk_tag_cache ORDER BY tag")
	if err != nil {
		return nil, fmt.Errorf("failed to query cached tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// UpdateTagAlert updates an existing tag alert's fields.
func UpdateTagAlert(db db.Database, alertID int, tag, slackChannelID, slackChannelName, alertType string) error {
	_, err := db.Exec(
		"UPDATE user_tag_alerts SET tag = ?, slack_channel_id = ?, slack_channel_name = ?, alert_type = ? WHERE id = ?",
		tag, slackChannelID, slackChannelName, alertType, alertID,
	)
	return err
}

// DeleteTagAlertsByIDs deletes multiple tag alerts by their IDs, scoped to a user.
func DeleteTagAlertsByIDs(db db.Database, userID int, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, userID)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := "DELETE FROM user_tag_alerts WHERE user_id = ? AND id IN (" + strings.Join(placeholders, ",") + ")"
	_, err := db.Exec(query, args...)
	return err
}

// GetTagAlertCountByUser returns the number of tag alerts for a user.
func GetTagAlertCountByUser(db db.Database, userID int) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM user_tag_alerts WHERE user_id = ?", userID).Scan(&count)
	return count, err
}

// AlertHistoryStats holds aggregate statistics for alert history.
type AlertHistoryStats struct {
	TotalAlerts  int
	AlertsToday  int
	TopTags      []TagCount
	TopTypes     []TypeCount
}

type TagCount struct {
	Tag   string
	Count int
}

type TypeCount struct {
	AlertType string
	Count     int
}

// GetAlertHistoryStats returns aggregate statistics for a user's alert history.
func GetAlertHistoryStats(db db.Database, userID int) (AlertHistoryStats, error) {
	var stats AlertHistoryStats

	db.QueryRow("SELECT COUNT(*) FROM alert_logs WHERE user_id = ?", userID).Scan(&stats.TotalAlerts)

	today := time.Now().Format("2006-01-02")
	db.QueryRow("SELECT COUNT(*) FROM alert_logs WHERE user_id = ? AND timestamp LIKE ?", userID, today+"%").Scan(&stats.AlertsToday)

	tagRows, err := db.Query("SELECT tag, COUNT(*) as cnt FROM alert_logs WHERE user_id = ? GROUP BY tag ORDER BY cnt DESC LIMIT 5", userID)
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var tc TagCount
			tagRows.Scan(&tc.Tag, &tc.Count)
			stats.TopTags = append(stats.TopTags, tc)
		}
	}

	typeRows, err := db.Query("SELECT alert_type, COUNT(*) as cnt FROM alert_logs WHERE user_id = ? GROUP BY alert_type ORDER BY cnt DESC LIMIT 5", userID)
	if err == nil {
		defer typeRows.Close()
		for typeRows.Next() {
			var tc TypeCount
			typeRows.Scan(&tc.AlertType, &tc.Count)
			stats.TopTypes = append(stats.TopTypes, tc)
		}
	}

	return stats, nil
}

// GetAlertHistoryPaginated returns paginated alert log entries for a user.
func GetAlertHistoryPaginated(db db.Database, userID, limit, offset int) ([]AlertLog, int, error) {
	var total int
	db.QueryRow("SELECT COUNT(*) FROM alert_logs WHERE user_id = ?", userID).Scan(&total)

	rows, err := db.Query(
		"SELECT id, user_id, ticket_id, tag, alert_type, timestamp FROM alert_logs WHERE user_id = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?",
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AlertLog
	for rows.Next() {
		var l AlertLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.TicketID, &l.Tag, &l.AlertType, &l.Timestamp); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, nil
}

// --- Skipped Alerts ---

const (
	SkipReasonSLADuplicate          = "sla_duplicate"
	SkipReasonNewTicketOutsideWindow = "new_ticket_outside_window"
	SkipReasonUpdateNoEndUserComment = "update_no_end_user_comment"
	SkipReasonUpdateNotModified      = "update_not_modified"
	SkipReasonSLAMetricMismatch      = "sla_metric_mismatch"
	SkipReasonSLAStageInactive       = "sla_stage_inactive"
)

type SkippedAlert struct {
	ID         int64  `db:"id"`
	TicketID   int64  `db:"ticket_id"`
	UserID     int64  `db:"user_id"`
	Tag        string `db:"tag"`
	AlertType  string `db:"alert_type"`
	SkipReason string `db:"skip_reason"`
	MetricType string `db:"metric_type"`
	Label      string `db:"label"`
	CreatedAt  string `db:"created_at"`
}

func CreateSkippedAlert(ctx context.Context, db db.Database, entry SkippedAlert) error {
	query := `INSERT INTO skipped_alerts (ticket_id, user_id, tag, alert_type, skip_reason, metric_type, label) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.ExecContext(ctx, query, entry.TicketID, entry.UserID, entry.Tag, entry.AlertType, entry.SkipReason, entry.MetricType, entry.Label)
	if err != nil {
		return fmt.Errorf("failed to create skipped alert: %w", err)
	}
	return nil
}

// --- Acknowledgment Logs ---

type AcknowledgmentLog struct {
	ID             int64  `db:"id"`
	AlertLogID     *int64 `db:"alert_log_id"`
	TicketID       int64  `db:"ticket_id"`
	SlackUserID    string `db:"slack_user_id"`
	SlackUserName  string `db:"slack_user_name"`
	AcknowledgedAt string `db:"acknowledged_at"`
}

func CreateAcknowledgmentLog(ctx context.Context, db db.Database, entry AcknowledgmentLog) error {
	query := `INSERT INTO acknowledgment_logs (alert_log_id, ticket_id, slack_user_id, slack_user_name) VALUES (?, ?, ?, ?)`
	_, err := db.ExecContext(ctx, query, entry.AlertLogID, entry.TicketID, entry.SlackUserID, entry.SlackUserName)
	if err != nil {
		return fmt.Errorf("failed to create acknowledgment log: %w", err)
	}
	return nil
}

// --- Daily Alert Log Sent dedup ---

type DailyAlertLogSent struct {
	ID        int64  `db:"id"`
	LogDate   string `db:"log_date"`
	CreatedAt string `db:"created_at"`
}

func GetDailyAlertLogSent(ctx context.Context, db db.Database, logDate string) (*DailyAlertLogSent, error) {
	var entry DailyAlertLogSent
	query := `SELECT id, log_date, created_at FROM daily_alert_log_sent WHERE log_date = ?`
	err := db.QueryRowContext(ctx, query, logDate).Scan(&entry.ID, &entry.LogDate, &entry.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func CreateDailyAlertLogSent(ctx context.Context, db db.Database, logDate string) error {
	query := `INSERT INTO daily_alert_log_sent (log_date) VALUES (?)`
	_, err := db.ExecContext(ctx, query, logDate)
	if err != nil {
		return fmt.Errorf("failed to create daily alert log sent entry: %w", err)
	}
	return nil
}

// --- Daily Alert Stats queries ---

type DailyAlertTypeCount struct {
	AlertType string
	Count     int
}

type DailyAlertChannelCount struct {
	ChannelName string
	Count       int
}

type DailyAlertTagCount struct {
	Tag   string
	Count int
}

type DailyTopTicket struct {
	TicketID int64
	Count    int
}

type DailySkippedGroup struct {
	SkipReason string
	Count      int
	TicketIDs  []int64
	Labels     []string
}

type DailyAckStats struct {
	TotalAcknowledged int
	AvgAckSeconds     float64
}

type DailyAlertSummary struct {
	TotalAlerts    int
	PreviousDay    int
	ByType         []DailyAlertTypeCount
	ByChannel      []DailyAlertChannelCount
	ByTag          []DailyAlertTagCount
	TopTickets     []DailyTopTicket
	Skipped        []DailySkippedGroup
	Acknowledgments DailyAckStats
}

func GetDailyAlertSummary(ctx context.Context, database db.Database, date string) (*DailyAlertSummary, error) {
	summary := &DailyAlertSummary{}
	datePrefix := date + "%"

	database.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM alert_logs WHERE timestamp LIKE ?", datePrefix).Scan(&summary.TotalAlerts)

	// Previous day count for trend
	prev, err := time.Parse("2006-01-02", date)
	if err == nil {
		prevDate := prev.AddDate(0, 0, -1).Format("2006-01-02")
		database.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM alert_logs WHERE timestamp LIKE ?", prevDate+"%").Scan(&summary.PreviousDay)
	}

	// By type
	typeRows, err := database.Query(
		"SELECT alert_type, COUNT(*) as cnt FROM alert_logs WHERE timestamp LIKE ? GROUP BY alert_type ORDER BY cnt DESC", datePrefix)
	if err == nil {
		for typeRows.Next() {
			var tc DailyAlertTypeCount
			typeRows.Scan(&tc.AlertType, &tc.Count)
			summary.ByType = append(summary.ByType, tc)
		}
		typeRows.Close()
	}

	// By channel (join with user_tag_alerts for channel name)
	channelRows, err := database.Query(
		`SELECT COALESCE(uta.slack_channel_name, uta.slack_channel_id) as ch, COUNT(*) as cnt
		 FROM alert_logs al
		 INNER JOIN user_tag_alerts uta ON al.user_id = uta.user_id AND al.tag = uta.tag AND al.alert_type = uta.alert_type
		 WHERE al.timestamp LIKE ?
		 GROUP BY ch ORDER BY cnt DESC`, datePrefix)
	if err == nil {
		for channelRows.Next() {
			var cc DailyAlertChannelCount
			channelRows.Scan(&cc.ChannelName, &cc.Count)
			summary.ByChannel = append(summary.ByChannel, cc)
		}
		channelRows.Close()
	}

	// By tag
	tagRows, err := database.Query(
		"SELECT tag, COUNT(*) as cnt FROM alert_logs WHERE timestamp LIKE ? GROUP BY tag ORDER BY cnt DESC LIMIT 10", datePrefix)
	if err == nil {
		for tagRows.Next() {
			var tc DailyAlertTagCount
			tagRows.Scan(&tc.Tag, &tc.Count)
			summary.ByTag = append(summary.ByTag, tc)
		}
		tagRows.Close()
	}

	// Top tickets
	ticketRows, err := database.Query(
		"SELECT ticket_id, COUNT(*) as cnt FROM alert_logs WHERE timestamp LIKE ? GROUP BY ticket_id ORDER BY cnt DESC LIMIT 5", datePrefix)
	if err == nil {
		for ticketRows.Next() {
			var tt DailyTopTicket
			ticketRows.Scan(&tt.TicketID, &tt.Count)
			summary.TopTickets = append(summary.TopTickets, tt)
		}
		ticketRows.Close()
	}

	// Skipped alerts grouped by reason
	skipRows, err := database.Query(
		`SELECT skip_reason, COUNT(*) as cnt FROM skipped_alerts
		 WHERE created_at LIKE ? GROUP BY skip_reason ORDER BY cnt DESC`, datePrefix)
	if err == nil {
		for skipRows.Next() {
			var sg DailySkippedGroup
			skipRows.Scan(&sg.SkipReason, &sg.Count)
			summary.Skipped = append(summary.Skipped, sg)
		}
		skipRows.Close()
	}

	// Fill in ticket IDs and labels for each skip reason
	for i := range summary.Skipped {
		reason := summary.Skipped[i].SkipReason
		detailRows, err := database.Query(
			`SELECT DISTINCT ticket_id, COALESCE(label, '') FROM skipped_alerts
			 WHERE created_at LIKE ? AND skip_reason = ? LIMIT 20`, datePrefix, reason)
		if err == nil {
			for detailRows.Next() {
				var tid int64
				var label string
				detailRows.Scan(&tid, &label)
				summary.Skipped[i].TicketIDs = append(summary.Skipped[i].TicketIDs, tid)
				if label != "" {
					summary.Skipped[i].Labels = append(summary.Skipped[i].Labels, label)
				}
			}
			detailRows.Close()
		}
	}

	// Acknowledgment stats
	database.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM acknowledgment_logs WHERE acknowledged_at LIKE ?", datePrefix).Scan(&summary.Acknowledgments.TotalAcknowledged)

	// Average ack time: seconds between alert_logs.timestamp and acknowledgment_logs.acknowledged_at
	database.QueryRowContext(ctx,
		`SELECT COALESCE(AVG(
			(julianday(ak.acknowledged_at) - julianday(al.timestamp)) * 86400
		), 0) FROM acknowledgment_logs ak
		INNER JOIN alert_logs al ON ak.alert_log_id = al.id
		WHERE ak.acknowledged_at LIKE ?`, datePrefix).Scan(&summary.Acknowledgments.AvgAckSeconds)

	return summary, nil
}

// ClearOldSkippedAlerts removes skipped_alerts entries older than the given number of days.
func ClearOldSkippedAlerts(ctx context.Context, db db.Database, days int) error {
	query := fmt.Sprintf("DELETE FROM skipped_alerts WHERE created_at < datetime('now', '-%d days')", days)
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to clear old skipped alerts: %w", err)
	}
	return nil
}

// ClearOldAcknowledgmentLogs removes acknowledgment_logs entries older than the given number of days.
func ClearOldAcknowledgmentLogs(ctx context.Context, db db.Database, days int) error {
	query := fmt.Sprintf("DELETE FROM acknowledgment_logs WHERE acknowledged_at < datetime('now', '-%d days')", days)
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to clear old acknowledgment logs: %w", err)
	}
	return nil
}
