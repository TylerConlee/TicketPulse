package services

import (
	"context"
	"testing"
	"time"

	"github.com/nukosuke/go-zendesk/zendesk"
	"github.com/stretchr/testify/assert"
)

func TestNewCommentFetcher(t *testing.T) {
	// Create a minimal ZendeskClient for testing
	zc := &ZendeskClient{}

	fetcher := NewCommentFetcher(zc, 5)

	assert.NotNil(t, fetcher)
	assert.Equal(t, 5, fetcher.workerCount)
	assert.Equal(t, zc, fetcher.zc)
}

func TestNewCommentFetcherWithRateLimit(t *testing.T) {
	zc := &ZendeskClient{}
	rateLimit := 50 * time.Millisecond

	fetcher := NewCommentFetcherWithRateLimit(zc, 3, rateLimit)

	assert.NotNil(t, fetcher)
	assert.Equal(t, 3, fetcher.workerCount)
	assert.Equal(t, rateLimit, fetcher.rateLimit)
}

func TestCommentFetcher_FetchBatch_Empty(t *testing.T) {
	zc := &ZendeskClient{}
	fetcher := NewCommentFetcherWithRateLimit(zc, 3, 10*time.Millisecond)

	result := fetcher.FetchBatch(context.Background(), []int64{})

	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestCommentInfo_Fields(t *testing.T) {
	now := time.Now()
	info := CommentInfo{
		TicketID:              12345,
		LastPublicCommentTime: now,
		IsEndUser:             true,
		Error:                 nil,
	}

	assert.Equal(t, int64(12345), info.TicketID)
	assert.Equal(t, now, info.LastPublicCommentTime)
	assert.True(t, info.IsEndUser)
	assert.Nil(t, info.Error)
}

func TestCommentCache(t *testing.T) {
	cache := NewCommentCache()

	assert.NotNil(t, cache)

	// Store a value
	info := CommentInfo{
		TicketID:              100,
		LastPublicCommentTime: time.Now(),
		IsEndUser:             true,
	}
	cache.Store(100, info)

	// Load the value
	loaded, ok := cache.Load(100)
	assert.True(t, ok)
	assert.Equal(t, info.TicketID, loaded.TicketID)
	assert.Equal(t, info.IsEndUser, loaded.IsEndUser)

	// Load non-existent
	_, ok = cache.Load(999)
	assert.False(t, ok)
}

func TestCommentCache_LoadAll(t *testing.T) {
	cache := NewCommentCache()

	infos := map[int64]CommentInfo{
		1: {TicketID: 1, IsEndUser: true},
		2: {TicketID: 2, IsEndUser: false},
		3: {TicketID: 3, IsEndUser: true},
	}

	cache.LoadAll(infos)

	for id, expected := range infos {
		loaded, ok := cache.Load(id)
		assert.True(t, ok)
		assert.Equal(t, expected.TicketID, loaded.TicketID)
		assert.Equal(t, expected.IsEndUser, loaded.IsEndUser)
	}
}

func TestCommentCache_Clear(t *testing.T) {
	cache := NewCommentCache()

	cache.Store(1, CommentInfo{TicketID: 1})
	cache.Store(2, CommentInfo{TicketID: 2})

	cache.Clear()

	_, ok := cache.Load(1)
	assert.False(t, ok)
	_, ok = cache.Load(2)
	assert.False(t, ok)
}

func TestFilterTicketsNeedingCommentCheck(t *testing.T) {
	lastPollTime := time.Now().Add(-10 * time.Minute)
	recentTime := time.Now().Add(-5 * time.Minute)
	oldTime := time.Now().Add(-20 * time.Minute)

	tickets := []TicketContext{
		{Ticket: zendesk.Ticket{ID: 1, UpdatedAt: &recentTime}},   // Should include
		{Ticket: zendesk.Ticket{ID: 2, UpdatedAt: &oldTime}},     // Should not include
		{Ticket: zendesk.Ticket{ID: 3, UpdatedAt: &recentTime}},  // Should include
		{Ticket: zendesk.Ticket{ID: 4, UpdatedAt: nil}},          // Should not include (nil)
	}

	result := FilterTicketsNeedingCommentCheck(tickets, lastPollTime)

	assert.Len(t, result, 2)
	assert.Contains(t, result, int64(1))
	assert.Contains(t, result, int64(3))
	assert.NotContains(t, result, int64(2))
	assert.NotContains(t, result, int64(4))
}

func TestCommentFetcher_ContextCancellation(t *testing.T) {
	zc := &ZendeskClient{}
	fetcher := NewCommentFetcherWithRateLimit(zc, 2, 100*time.Millisecond)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := fetcher.FetchSingle(ctx, 12345)

	assert.Equal(t, int64(12345), result.TicketID)
	assert.Error(t, result.Error)
	assert.Equal(t, context.Canceled, result.Error)
}
