package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/logging"
	"github.com/TylerConlee/TicketPulse/models"
)

// SchedulerService handles scheduled tasks like daily summaries
type SchedulerService struct {
	db           db.Database
	slackService *SlackService
	configCache  *ConfigCache
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(db db.Database, slackService *SlackService, configCache *ConfigCache) *SchedulerService {
	return &SchedulerService{
		db:           db,
		slackService: slackService,
		configCache:  configCache,
	}
}

// StartScheduler starts the scheduler that checks for users who need daily summaries
// and periodically refreshes the Zendesk tag cache.
func (s *SchedulerService) StartScheduler(ctx context.Context) {
	summaryTicker := time.NewTicker(1 * time.Minute)
	defer summaryTicker.Stop()

	tagCacheTicker := time.NewTicker(6 * time.Hour)
	defer tagCacheTicker.Stop()

	log.Println("Scheduler started (daily summaries + tag cache refresh)")

	s.refreshTagCache()

	for {
		select {
		case <-ctx.Done():
			log.Println("Scheduler stopped")
			return
		case <-summaryTicker.C:
			if err := s.checkAndSendSummaries(ctx); err != nil {
				log.Printf("Error checking for daily summaries: %v", err)
			}
			if err := s.checkAndSendDailyAlertLog(ctx); err != nil {
				log.Printf("Error checking for daily alert log: %v", err)
			}
		case <-tagCacheTicker.C:
			s.refreshTagCache()
			s.cleanupOldAlertData(ctx)
		}
	}
}

// checkAndSendSummaries checks all users with daily summary enabled and sends summaries if needed
func (s *SchedulerService) checkAndSendSummaries(ctx context.Context) error {
	users, err := models.GetUsersWithDailySummaryEnabled(s.db)
	if err != nil {
		return fmt.Errorf("failed to get users with daily summary enabled: %w", err)
	}

	logging.Debug(logging.AreaScheduler, "checkAndSendSummaries: found %d users with daily summary enabled", len(users))

	now := time.Now()

	// Collect users that need summaries
	type summaryTask struct {
		user  models.User
		today time.Time
	}
	var tasks []summaryTask

	for _, user := range users {
		if !user.SummaryTime.Valid {
			logging.Debug(logging.AreaScheduler, "User %s: no summary time configured, skipping", user.Email)
			continue
		}

		if !user.WorkDayStartTime.Valid || !user.WorkDayEndTime.Valid || !user.Timezone.Valid {
			log.Printf("User %s has daily summary enabled but missing work day settings, skipping", user.Email)
			continue
		}

		loc, err := time.LoadLocation(user.Timezone.String)
		if err != nil {
			log.Printf("Invalid timezone for user %s: %v", user.Email, err)
			continue
		}

		nowInTZ := now.In(loc)
		summaryTime := user.SummaryTime.Time

		currentMinutes := nowInTZ.Hour()*60 + nowInTZ.Minute()
		targetMinutes := summaryTime.Hour()*60 + summaryTime.Minute()

		timeDiff := currentMinutes - targetMinutes
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}

		logging.Debug(logging.AreaScheduler, "User %s: current=%02d:%02d target=%02d:%02d diff=%dm (tz=%s)",
			user.Email, nowInTZ.Hour(), nowInTZ.Minute(), summaryTime.Hour(), summaryTime.Minute(), timeDiff, user.Timezone.String)

		if timeDiff <= 1 {
			today := time.Date(nowInTZ.Year(), nowInTZ.Month(), nowInTZ.Day(), 0, 0, 0, 0, loc)
			_, err := models.GetDailySummaryLog(ctx, s.db, user.ID, today)
			if err == nil {
				logging.Debug(logging.AreaScheduler, "User %s: summary already sent today, skipping", user.Email)
				continue
			}
			tasks = append(tasks, summaryTask{user: user, today: today})
		}
	}

	if len(tasks) == 0 {
		return nil
	}

	// Send summaries concurrently with bounded parallelism
	const maxConcurrency = 3
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t summaryTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			logging.Debug(logging.AreaScheduler, "User %s: sending daily summary now", t.user.Email)
			if err := s.sendDailySummary(ctx, t.user); err != nil {
				log.Printf("Failed to send daily summary to user %s: %v", t.user.Email, err)
				return
			}

			if err := models.CreateDailySummaryLog(ctx, s.db, t.user.ID, t.today); err != nil {
				log.Printf("Failed to log daily summary for user %s: %v", t.user.Email, err)
			}
			logging.Debug(logging.AreaScheduler, "User %s: daily summary sent and logged successfully", t.user.Email)
		}(task)
	}

	wg.Wait()
	return nil
}

// sendDailySummary sends a daily summary to a user
func (s *SchedulerService) sendDailySummary(ctx context.Context, user models.User) error {
	// Create Zendesk client
	zc, err := NewZendeskClient(s.db)
	if err != nil {
		return fmt.Errorf("failed to create Zendesk client: %w", err)
	}

	// Get user's configured tags if tag filter is set to configured_tags
	var userTags []string
	if user.SummaryTagFilter == TagFilterConfiguredTags {
		tagAlerts, err := models.GetTagAlertsByUser(s.db, user.ID)
		if err != nil {
			log.Printf("Failed to get user tags for %s: %v", user.Email, err)
		} else {
			// Extract unique tags
			tagMap := make(map[string]bool)
			for _, alert := range tagAlerts {
				if !tagMap[alert.Tag] {
					tagMap[alert.Tag] = true
					userTags = append(userTags, alert.Tag)
				}
			}
		}
	}

	// Prepare work day times
	workDayStart := user.WorkDayStartTime.Time
	workDayEnd := user.WorkDayEndTime.Time
	timezone := user.Timezone.String

	// Set default filter modes if not set
	tagFilterMode := user.SummaryTagFilter
	if tagFilterMode == "" {
		tagFilterMode = TagFilterAllTags
	}
	ticketFilterMode := user.SummaryTicketFilter
	if ticketFilterMode == "" {
		ticketFilterMode = TicketFilterAssigned
	}

	// Generate and send summary
	_, err = zc.GenerateDailySummary(user.Email, s.slackService, workDayStart, workDayEnd, timezone, tagFilterMode, ticketFilterMode, userTags)
	if err != nil {
		return fmt.Errorf("failed to generate daily summary: %w", err)
	}

	log.Printf("Daily summary sent to user %s", user.Email)
	return nil
}

// checkAndSendDailyAlertLog checks if the daily alert log should be sent.
func (s *SchedulerService) checkAndSendDailyAlertLog(ctx context.Context) error {
	if s.configCache == nil {
		return nil
	}

	enabled, err := s.configCache.Get("daily_alert_log_enabled")
	if err != nil || enabled != "on" {
		return nil
	}

	channelID, err := s.configCache.Get("daily_alert_log_channel_id")
	if err != nil || channelID == "" {
		return nil
	}

	targetTime, err := s.configCache.Get("daily_alert_log_time")
	if err != nil || targetTime == "" {
		return nil
	}

	tz, err := s.configCache.Get("daily_alert_log_timezone")
	if err != nil || tz == "" {
		tz = "UTC"
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("Invalid daily alert log timezone %q: %v", tz, err)
		return nil
	}

	nowInTZ := time.Now().In(loc)

	parsedTarget, err := time.Parse("15:04", targetTime)
	if err != nil {
		log.Printf("Invalid daily alert log time %q: %v", targetTime, err)
		return nil
	}

	currentMinutes := nowInTZ.Hour()*60 + nowInTZ.Minute()
	targetMinutes := parsedTarget.Hour()*60 + parsedTarget.Minute()

	timeDiff := currentMinutes - targetMinutes
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	if timeDiff > 1 {
		return nil
	}

	logging.Debug(logging.AreaScheduler, "Daily alert log: time match (current=%02d:%02d target=%s tz=%s), sending...",
		nowInTZ.Hour(), nowInTZ.Minute(), targetTime, tz)

	return sendDailyAlertLog(ctx, s.db, s.slackService, s.configCache)
}

func (s *SchedulerService) cleanupOldAlertData(ctx context.Context) {
	const retentionDays = 30
	if err := models.ClearOldSkippedAlerts(ctx, s.db, retentionDays); err != nil {
		log.Printf("Failed to clean up old skipped alerts: %v", err)
	}
	if err := models.ClearOldAcknowledgmentLogs(ctx, s.db, retentionDays); err != nil {
		log.Printf("Failed to clean up old acknowledgment logs: %v", err)
	}
}

// refreshTagCache fetches all tags from Zendesk and stores them in the cache table.
func (s *SchedulerService) refreshTagCache() {
	zc, err := NewZendeskClient(s.db)
	if err != nil {
		log.Printf("Tag cache refresh: failed to create Zendesk client: %v", err)
		return
	}

	tags, err := zc.ListAllTags()
	if err != nil {
		log.Printf("Tag cache refresh: failed to list tags from Zendesk: %v", err)
		return
	}

	if err := models.ClearAndReplaceCachedTags(s.db, tags); err != nil {
		log.Printf("Tag cache refresh: failed to store tags: %v", err)
		return
	}

	log.Printf("Tag cache refresh: cached %d tags from Zendesk", len(tags))
}
