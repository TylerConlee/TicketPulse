package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRealClock_Now(t *testing.T) {
	clock := RealClock{}
	before := time.Now()
	result := clock.Now()
	after := time.Now()

	// The result should be between before and after
	assert.True(t, result.After(before) || result.Equal(before))
	assert.True(t, result.Before(after) || result.Equal(after))
}

func TestMockClock_Now(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := &MockClock{CurrentTime: fixedTime}

	result := clock.Now()

	assert.Equal(t, fixedTime, result)
}

func TestMockClock_Advance(t *testing.T) {
	startTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := &MockClock{CurrentTime: startTime}

	clock.Advance(1 * time.Hour)

	expected := time.Date(2025, 1, 15, 11, 30, 0, 0, time.UTC)
	assert.Equal(t, expected, clock.Now())
}

func TestMockClock_AdvanceMultiple(t *testing.T) {
	startTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := &MockClock{CurrentTime: startTime}

	clock.Advance(30 * time.Minute)
	clock.Advance(15 * time.Minute)

	expected := time.Date(2025, 1, 15, 11, 15, 0, 0, time.UTC)
	assert.Equal(t, expected, clock.Now())
}

func TestMockClock_Set(t *testing.T) {
	startTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := &MockClock{CurrentTime: startTime}

	newTime := time.Date(2025, 6, 1, 14, 0, 0, 0, time.UTC)
	clock.Set(newTime)

	assert.Equal(t, newTime, clock.Now())
}

func TestNewMockClock(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	clock := NewMockClock(fixedTime)

	assert.NotNil(t, clock)
	assert.Equal(t, fixedTime, clock.Now())
}

func TestNewMockClockNow(t *testing.T) {
	before := time.Now()
	clock := NewMockClockNow()
	after := time.Now()

	assert.NotNil(t, clock)
	assert.True(t, clock.Now().After(before) || clock.Now().Equal(before))
	assert.True(t, clock.Now().Before(after) || clock.Now().Equal(after))
}

func TestClockInterface(t *testing.T) {
	// Verify both implementations satisfy the Clock interface
	var _ Clock = RealClock{}
	var _ Clock = &MockClock{}

	// Test using Clock interface
	testWithClock := func(c Clock) time.Time {
		return c.Now()
	}

	realResult := testWithClock(RealClock{})
	assert.False(t, realResult.IsZero())

	mockTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mockResult := testWithClock(&MockClock{CurrentTime: mockTime})
	assert.Equal(t, mockTime, mockResult)
}
