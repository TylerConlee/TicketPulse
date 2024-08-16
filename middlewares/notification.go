package middlewares

import (
	"context"
	"log"
	"net/http"
	"sync"
)

type contextKey string

const notificationKey contextKey = "notifications"

// Notification struct to hold categorized messages with severity
type Notification struct {
	Category string
	Message  string
	Severity string
}

var (
	globalNotifications []Notification
	notificationsMutex  sync.Mutex
)

// AddGlobalNotification adds a notification to the global list and triggers SSE
func AddGlobalNotification(sseServer *SSEServer, category, message, severity string) {
	notificationsMutex.Lock()
	defer notificationsMutex.Unlock()

	notification := Notification{
		Category: category,
		Message:  message,
		Severity: severity,
	}

	globalNotifications = append(globalNotifications, notification)

	// Trigger SSE notification
	if sseServer != nil {
		sseMessage := formatNotificationMessage(notification)
		sseServer.NotifyAll(sseMessage)
	}
}

// Formats the notification message for SSE
func formatNotificationMessage(notification Notification) string {
	return notification.Category + ": " + notification.Message + " (" + notification.Severity + ")"
}

// NotificationMiddleware to attach notifications to the request context
func NotificationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Entering NotificationMiddleware")
		defer log.Println("Exiting NotificationMiddleware")
		notifications := []Notification{}

		// Lock the mutex and add global notifications
		notificationsMutex.Lock()
		notifications = append(notifications, globalNotifications...)
		notificationsMutex.Unlock()

		// Store the notifications in the request context
		ctx := context.WithValue(r.Context(), notificationKey, notifications)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetNotificationsFromContext retrieves the notifications from the request context
func GetNotificationsFromContext(r *http.Request) []Notification {
	notifications, ok := r.Context().Value(notificationKey).([]Notification)
	if !ok {
		return nil
	}
	return notifications
}
