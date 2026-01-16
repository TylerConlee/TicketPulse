// Package services provides core business logic for TicketPulse.
package services

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

// CommentInfo holds the result of fetching comment information for a ticket.
type CommentInfo struct {
	TicketID              int64
	LastPublicCommentTime time.Time
	IsEndUser             bool
	Error                 error
}

// CommentFetcher fetches ticket comment information concurrently with rate limiting.
type CommentFetcher struct {
	zc          *ZendeskClient
	workerCount int
	rateLimit   time.Duration
}

// NewCommentFetcher creates a new CommentFetcher.
// workers specifies the number of concurrent workers.
// Rate limit is read from ZENDESK_RATE_LIMIT_MS env var, defaulting to 100ms.
func NewCommentFetcher(zc *ZendeskClient, workers int) *CommentFetcher {
	rateLimit := 100 * time.Millisecond
	if rateLimitStr := os.Getenv("ZENDESK_RATE_LIMIT_MS"); rateLimitStr != "" {
		if ms, err := strconv.Atoi(rateLimitStr); err == nil && ms > 0 {
			rateLimit = time.Duration(ms) * time.Millisecond
		}
	}

	return &CommentFetcher{
		zc:          zc,
		workerCount: workers,
		rateLimit:   rateLimit,
	}
}

// NewCommentFetcherWithRateLimit creates a CommentFetcher with explicit rate limit.
func NewCommentFetcherWithRateLimit(zc *ZendeskClient, workers int, rateLimit time.Duration) *CommentFetcher {
	return &CommentFetcher{
		zc:          zc,
		workerCount: workers,
		rateLimit:   rateLimit,
	}
}

// FetchBatch fetches comment information for multiple tickets concurrently.
// Returns a map of ticket ID to CommentInfo.
func (cf *CommentFetcher) FetchBatch(ctx context.Context, ticketIDs []int64) map[int64]CommentInfo {
	if len(ticketIDs) == 0 {
		return make(map[int64]CommentInfo)
	}

	jobs := make(chan int64, len(ticketIDs))
	results := make(chan CommentInfo, len(ticketIDs))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < cf.workerCount; i++ {
		wg.Add(1)
		go cf.worker(ctx, jobs, results, &wg)
	}

	// Send jobs
	for _, id := range ticketIDs {
		jobs <- id
	}
	close(jobs)

	// Wait for workers to finish and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	resultMap := make(map[int64]CommentInfo, len(ticketIDs))
	for info := range results {
		resultMap[info.TicketID] = info
	}

	return resultMap
}

// worker processes jobs from the jobs channel and sends results to the results channel.
func (cf *CommentFetcher) worker(ctx context.Context, jobs <-chan int64, results chan<- CommentInfo, wg *sync.WaitGroup) {
	defer wg.Done()

	for ticketID := range jobs {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			results <- CommentInfo{
				TicketID: ticketID,
				Error:    ctx.Err(),
			}
			continue
		default:
		}

		// Rate limiting - sleep before making the request
		time.Sleep(cf.rateLimit)

		// Fetch the comment info
		lastTime, isEndUser, err := cf.zc.GetLastPublicCommentTime(ticketID)
		if err != nil {
			log.Printf("CommentFetcher: Error fetching comments for ticket %d: %v", ticketID, err)
		}

		results <- CommentInfo{
			TicketID:              ticketID,
			LastPublicCommentTime: lastTime,
			IsEndUser:             isEndUser,
			Error:                 err,
		}
	}
}

// FetchSingle fetches comment information for a single ticket.
// Useful for when you only need one ticket's info.
func (cf *CommentFetcher) FetchSingle(ctx context.Context, ticketID int64) CommentInfo {
	select {
	case <-ctx.Done():
		return CommentInfo{
			TicketID: ticketID,
			Error:    ctx.Err(),
		}
	default:
	}

	lastTime, isEndUser, err := cf.zc.GetLastPublicCommentTime(ticketID)
	return CommentInfo{
		TicketID:              ticketID,
		LastPublicCommentTime: lastTime,
		IsEndUser:             isEndUser,
		Error:                 err,
	}
}

// FilterTicketsNeedingCommentCheck returns ticket IDs that need comment checking.
// A ticket needs checking if it was updated after the last poll time.
func FilterTicketsNeedingCommentCheck(tickets []TicketContext, lastPollTime time.Time) []int64 {
	var result []int64
	for _, tc := range tickets {
		if tc.Ticket.UpdatedAt != nil && tc.Ticket.UpdatedAt.After(lastPollTime) {
			result = append(result, tc.Ticket.ID)
		}
	}
	return result
}

// CommentCache provides a way to cache comment info alongside ticket contexts.
type CommentCache struct {
	mu    sync.RWMutex
	cache map[int64]CommentInfo
}

// NewCommentCache creates a new CommentCache.
func NewCommentCache() *CommentCache {
	return &CommentCache{
		cache: make(map[int64]CommentInfo),
	}
}

// Load retrieves comment info from the cache.
func (c *CommentCache) Load(ticketID int64) (CommentInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info, ok := c.cache[ticketID]
	return info, ok
}

// Store saves comment info to the cache.
func (c *CommentCache) Store(ticketID int64, info CommentInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[ticketID] = info
}

// LoadAll loads multiple comment infos from a map.
func (c *CommentCache) LoadAll(infos map[int64]CommentInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, info := range infos {
		c.cache[id] = info
	}
}

// Clear removes all entries from the cache.
func (c *CommentCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[int64]CommentInfo)
}
