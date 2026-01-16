package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/models"
)

// SchedulerService handles scheduled tasks like daily summaries
type SchedulerService struct {
	db            db.Database
	slackService  *SlackService
	zendeskClient *ZendeskClient
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(db db.Database, slackService *SlackService) *SchedulerService {
	return &SchedulerService{
		db:           db,
		slackService: slackService,
	}
}

// StartScheduler starts the scheduler that checks for users who need daily summaries
func (s *SchedulerService) StartScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Println("Daily summary scheduler started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Scheduler stopped")
			return
		case <-ticker.C:
			if err := s.checkAndSendSummaries(ctx); err != nil {
				log.Printf("Error checking for daily summaries: %v", err)
			}
		}
	}
}

// checkAndSendSummaries checks all users with daily summary enabled and sends summaries if needed
func (s *SchedulerService) checkAndSendSummaries(ctx context.Context) error {
	users, err := models.GetUsersWithDailySummaryEnabled(s.db)
	if err != nil {
		return fmt.Errorf("failed to get users with daily summary enabled: %w", err)
	}

	now := time.Now()

	for _, user := range users {
		// Check if user has summary time configured
		if !user.SummaryTime.Valid {
			continue
		}

		// Check if user has work day settings configured
		if !user.WorkDayStartTime.Valid || !user.WorkDayEndTime.Valid || !user.Timezone.Valid {
			log.Printf("User %s has daily summary enabled but missing work day settings, skipping", user.Email)
			continue
		}

		// Load user's timezone
		loc, err := time.LoadLocation(user.Timezone.String)
		if err != nil {
			log.Printf("Invalid timezone for user %s: %v", user.Email, err)
			continue
		}

		// Get current time in user's timezone
		nowInTZ := now.In(loc)
		summaryTime := user.SummaryTime.Time

		// Check if current time matches summary time (within 1 minute window)
		currentHour := nowInTZ.Hour()
		currentMinute := nowInTZ.Minute()
		targetHour := summaryTime.Hour()
		targetMinute := summaryTime.Minute()

		// Calculate time difference in minutes
		currentMinutes := currentHour*60 + currentMinute
		targetMinutes := targetHour*60 + targetMinute

		timeDiff := currentMinutes - targetMinutes
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}

		// If we're within 1 minute of the target time, send summary
		if timeDiff <= 1 {
			// Check if summary was already sent today
			today := time.Date(nowInTZ.Year(), nowInTZ.Month(), nowInTZ.Day(), 0, 0, 0, 0, loc)
			_, err := models.GetDailySummaryLog(ctx, s.db, user.ID, today)
			if err == nil {
				// Summary already sent today, skip
				continue
			}

			// Send summary
			if err := s.sendDailySummary(ctx, user); err != nil {
				log.Printf("Failed to send daily summary to user %s: %v", user.Email, err)
				continue
			}

			// Log that summary was sent
			if err := models.CreateDailySummaryLog(ctx, s.db, user.ID, today); err != nil {
				log.Printf("Failed to log daily summary for user %s: %v", user.Email, err)
			}
		}
	}

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
