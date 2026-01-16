// Package services provides core business logic for TicketPulse.
package services

import (
	"sync"
	"time"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/TylerConlee/TicketPulse/models"
)

// ConfigCache provides an in-memory cache for configuration values with TTL.
// This reduces database queries for frequently accessed configuration.
type ConfigCache struct {
	mu    sync.RWMutex
	cache map[string]cachedValue
	ttl   time.Duration
	db    db.Database
	clock Clock
}

// cachedValue holds a cached configuration value with its expiration time.
type cachedValue struct {
	value     string
	expiresAt time.Time
}

// NewConfigCache creates a new configuration cache with the specified TTL.
func NewConfigCache(database db.Database, ttl time.Duration) *ConfigCache {
	return &ConfigCache{
		cache: make(map[string]cachedValue),
		ttl:   ttl,
		db:    database,
		clock: RealClock{},
	}
}

// NewConfigCacheWithClock creates a new configuration cache with a custom clock for testing.
func NewConfigCacheWithClock(database db.Database, ttl time.Duration, clock Clock) *ConfigCache {
	return &ConfigCache{
		cache: make(map[string]cachedValue),
		ttl:   ttl,
		db:    database,
		clock: clock,
	}
}

// Get retrieves a configuration value, using the cache if available and not expired.
func (c *ConfigCache) Get(key string) (string, error) {
	c.mu.RLock()
	if cached, exists := c.cache[key]; exists {
		if c.clock.Now().Before(cached.expiresAt) {
			c.mu.RUnlock()
			return cached.value, nil
		}
	}
	c.mu.RUnlock()

	// Cache miss or expired - fetch from database
	value, err := models.GetConfiguration(c.db, key)
	if err != nil {
		return "", err
	}

	// Store in cache
	c.mu.Lock()
	c.cache[key] = cachedValue{
		value:     value,
		expiresAt: c.clock.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return value, nil
}

// Set stores a value in both the database and cache.
func (c *ConfigCache) Set(key, value string) error {
	err := models.SetConfiguration(c.db, key, value)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.cache[key] = cachedValue{
		value:     value,
		expiresAt: c.clock.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return nil
}

// Invalidate removes a specific key from the cache.
func (c *ConfigCache) Invalidate(key string) {
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()
}

// InvalidateAll clears the entire cache.
func (c *ConfigCache) InvalidateAll() {
	c.mu.Lock()
	c.cache = make(map[string]cachedValue)
	c.mu.Unlock()
}

// Preload loads commonly used configuration keys into the cache.
func (c *ConfigCache) Preload(keys []string) error {
	for _, key := range keys {
		_, err := c.Get(key)
		if err != nil {
			return err
		}
	}
	return nil
}

// PreloadZendeskConfig loads all Zendesk-related configuration into the cache.
func (c *ConfigCache) PreloadZendeskConfig() error {
	return c.Preload([]string{
		"zendesk_subdomain",
		"zendesk_email",
		"zendesk_api_key",
	})
}

// PreloadSlackConfig loads all Slack-related configuration into the cache.
func (c *ConfigCache) PreloadSlackConfig() error {
	return c.Preload([]string{
		"slack_bot_token",
		"slack_app_token",
	})
}
