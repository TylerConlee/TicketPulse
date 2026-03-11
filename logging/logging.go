package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Area represents a debug logging area that can be independently toggled.
type Area string

const (
	AreaZendesk   Area = "zendesk"
	AreaSLA       Area = "sla"
	AreaCache     Area = "cache"
	AreaSlack     Area = "slack"
	AreaScheduler Area = "scheduler"
	AreaPolling   Area = "polling"
	AreaTags      Area = "tags"
	AreaDB        Area = "db"
)

var allAreas = []Area{
	AreaZendesk, AreaSLA, AreaCache, AreaSlack,
	AreaScheduler, AreaPolling, AreaTags, AreaDB,
}

var (
	enabledAreas map[Area]bool
	mu           sync.RWMutex
	initialized  bool
)

func init() {
	enabledAreas = make(map[Area]bool)
	loadFromEnv()
}

// LoadFromEnv re-reads DEBUG_AREAS from the environment. Call this after
// loading .env files (e.g. via godotenv) so that values set there take effect.
func LoadFromEnv() {
	mu.Lock()
	enabledAreas = make(map[Area]bool)
	initialized = false
	mu.Unlock()
	loadFromEnv()
}

func loadFromEnv() {
	raw := os.Getenv("DEBUG_AREAS")
	if raw == "" {
		initialized = true
		return
	}

	raw = strings.TrimSpace(raw)
	if strings.EqualFold(raw, "all") {
		for _, a := range allAreas {
			enabledAreas[a] = true
		}
		initialized = true
		slog.Info("All debug areas enabled", "areas", allAreas)
		return
	}

	parts := strings.Split(raw, ",")
	for _, p := range parts {
		area := Area(strings.TrimSpace(strings.ToLower(p)))
		if isValidArea(area) {
			enabledAreas[area] = true
		} else {
			slog.Warn("Unknown debug area", "area", p, "valid", allAreas)
		}
	}

	if len(enabledAreas) > 0 {
		enabled := make([]Area, 0, len(enabledAreas))
		for a := range enabledAreas {
			enabled = append(enabled, a)
		}
		slog.Info("Debug areas enabled", "areas", enabled)
	}
	initialized = true
}

func isValidArea(a Area) bool {
	for _, valid := range allAreas {
		if a == valid {
			return true
		}
	}
	return false
}

// Enable turns on debug logging for the given area at runtime.
func Enable(area Area) {
	mu.Lock()
	defer mu.Unlock()
	enabledAreas[area] = true
}

// Disable turns off debug logging for the given area at runtime.
func Disable(area Area) {
	mu.Lock()
	defer mu.Unlock()
	delete(enabledAreas, area)
}

// EnableAll turns on debug logging for every area.
func EnableAll() {
	mu.Lock()
	defer mu.Unlock()
	for _, a := range allAreas {
		enabledAreas[a] = true
	}
}

// DisableAll turns off all debug logging.
func DisableAll() {
	mu.Lock()
	defer mu.Unlock()
	enabledAreas = make(map[Area]bool)
}

// IsEnabled checks whether a specific area has debug logging on.
func IsEnabled(area Area) bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabledAreas[area]
}

// EnabledAreas returns a slice of currently enabled areas.
func EnabledAreas() []Area {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]Area, 0, len(enabledAreas))
	for a := range enabledAreas {
		result = append(result, a)
	}
	return result
}

// Debug logs a structured debug message if the given area is enabled.
func Debug(area Area, format string, args ...interface{}) {
	mu.RLock()
	enabled := enabledAreas[area]
	mu.RUnlock()

	if !enabled {
		return
	}

	slog.Debug(fmt.Sprintf(format, args...), "area", string(area))
}

// InitLogger sets up the default slog logger.
// When format is "json", output is JSON. Otherwise, plain text.
func InitLogger(format string) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}

	if strings.EqualFold(format, "json") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
