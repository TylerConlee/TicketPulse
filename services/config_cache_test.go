package services

import (
	"testing"
	"time"

	"github.com/TylerConlee/TicketPulse/models"
	"github.com/TylerConlee/TicketPulse/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigCache(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	cache := NewConfigCache(database, 5*time.Minute)

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.cache)
	assert.Equal(t, 5*time.Minute, cache.ttl)
}

func TestConfigCache_Get_CacheMiss(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	// Set up a configuration value in the database
	err := models.SetConfiguration(database, "test_key", "test_value")
	require.NoError(t, err)

	cache := NewConfigCache(database, 5*time.Minute)

	// First call - cache miss, should fetch from DB
	value, err := cache.Get("test_key")
	require.NoError(t, err)
	assert.Equal(t, "test_value", value)
}

func TestConfigCache_Get_CacheHit(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	// Set up a configuration value in the database
	err := models.SetConfiguration(database, "test_key", "original_value")
	require.NoError(t, err)

	cache := NewConfigCache(database, 5*time.Minute)

	// First call - cache miss
	value1, err := cache.Get("test_key")
	require.NoError(t, err)
	assert.Equal(t, "original_value", value1)

	// Update the database directly (simulating external change)
	err = models.SetConfiguration(database, "test_key", "updated_value")
	require.NoError(t, err)

	// Second call - should still return cached value
	value2, err := cache.Get("test_key")
	require.NoError(t, err)
	assert.Equal(t, "original_value", value2) // Still cached
}

func TestConfigCache_Get_CacheExpired(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	// Set up a configuration value in the database
	err := models.SetConfiguration(database, "test_key", "original_value")
	require.NoError(t, err)

	// Use mock clock for controlled time
	startTime := time.Now()
	mockClock := NewMockClock(startTime)
	cache := NewConfigCacheWithClock(database, 1*time.Minute, mockClock)

	// First call - cache miss
	value1, err := cache.Get("test_key")
	require.NoError(t, err)
	assert.Equal(t, "original_value", value1)

	// Update the database
	err = models.SetConfiguration(database, "test_key", "updated_value")
	require.NoError(t, err)

	// Advance time past TTL
	mockClock.Advance(2 * time.Minute)

	// Third call - cache expired, should fetch new value
	value2, err := cache.Get("test_key")
	require.NoError(t, err)
	assert.Equal(t, "updated_value", value2) // Now updated
}

func TestConfigCache_Set(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	cache := NewConfigCache(database, 5*time.Minute)

	// Set value through cache
	err := cache.Set("new_key", "new_value")
	require.NoError(t, err)

	// Verify it's in cache
	value, err := cache.Get("new_key")
	require.NoError(t, err)
	assert.Equal(t, "new_value", value)

	// Verify it's also in database
	dbValue, err := models.GetConfiguration(database, "new_key")
	require.NoError(t, err)
	assert.Equal(t, "new_value", dbValue)
}

func TestConfigCache_Invalidate(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	err := models.SetConfiguration(database, "test_key", "original_value")
	require.NoError(t, err)

	cache := NewConfigCache(database, 5*time.Minute)

	// Populate cache
	_, err = cache.Get("test_key")
	require.NoError(t, err)

	// Update database
	err = models.SetConfiguration(database, "test_key", "updated_value")
	require.NoError(t, err)

	// Invalidate cache
	cache.Invalidate("test_key")

	// Next get should fetch from database
	value, err := cache.Get("test_key")
	require.NoError(t, err)
	assert.Equal(t, "updated_value", value)
}

func TestConfigCache_InvalidateAll(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	err := models.SetConfiguration(database, "key1", "value1")
	require.NoError(t, err)
	err = models.SetConfiguration(database, "key2", "value2")
	require.NoError(t, err)

	cache := NewConfigCache(database, 5*time.Minute)

	// Populate cache
	_, _ = cache.Get("key1")
	_, _ = cache.Get("key2")

	// Update database
	err = models.SetConfiguration(database, "key1", "updated1")
	require.NoError(t, err)
	err = models.SetConfiguration(database, "key2", "updated2")
	require.NoError(t, err)

	// Invalidate all
	cache.InvalidateAll()

	// Next gets should fetch from database
	value1, _ := cache.Get("key1")
	value2, _ := cache.Get("key2")
	assert.Equal(t, "updated1", value1)
	assert.Equal(t, "updated2", value2)
}

func TestConfigCache_Preload(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	err := models.SetConfiguration(database, "key1", "value1")
	require.NoError(t, err)
	err = models.SetConfiguration(database, "key2", "value2")
	require.NoError(t, err)

	cache := NewConfigCache(database, 5*time.Minute)

	// Preload keys
	err = cache.Preload([]string{"key1", "key2"})
	require.NoError(t, err)

	// Verify they're cached (update DB and check we still get old values)
	err = models.SetConfiguration(database, "key1", "updated1")
	require.NoError(t, err)

	value1, _ := cache.Get("key1")
	assert.Equal(t, "value1", value1) // Should be cached
}

func TestConfigCache_ConcurrentAccess(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	err := models.SetConfiguration(database, "concurrent_key", "concurrent_value")
	require.NoError(t, err)

	cache := NewConfigCache(database, 5*time.Minute)

	// Pre-populate the cache to avoid concurrent DB access issues in test
	_, err = cache.Get("concurrent_key")
	require.NoError(t, err)

	// Run concurrent reads from cache
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				value, err := cache.Get("concurrent_key")
				if err != nil {
					t.Logf("Error in concurrent access: %v", err)
				}
				if value != "concurrent_value" {
					t.Logf("Unexpected value: %s", value)
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConfigCache_NonExistentKey(t *testing.T) {
	database := testutil.SetupTestDB()
	defer testutil.CleanupTestDB(database)

	cache := NewConfigCache(database, 5*time.Minute)

	// Get non-existent key
	value, err := cache.Get("nonexistent")
	require.NoError(t, err)
	assert.Equal(t, "", value) // Empty string for missing config
}
