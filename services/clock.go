// Package services provides core business logic for TicketPulse.
package services

import "time"

// Clock is an interface for time operations, enabling testability.
type Clock interface {
	Now() time.Time
}

// RealClock provides the actual system time.
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// MockClock provides a controllable time for testing.
type MockClock struct {
	CurrentTime time.Time
}

// Now returns the mock's current time.
func (m *MockClock) Now() time.Time {
	return m.CurrentTime
}

// Advance moves the mock clock forward by the specified duration.
func (m *MockClock) Advance(d time.Duration) {
	m.CurrentTime = m.CurrentTime.Add(d)
}

// Set sets the mock clock to a specific time.
func (m *MockClock) Set(t time.Time) {
	m.CurrentTime = t
}

// NewMockClock creates a new MockClock set to the specified time.
func NewMockClock(t time.Time) *MockClock {
	return &MockClock{CurrentTime: t}
}

// NewMockClockNow creates a new MockClock set to the current time.
func NewMockClockNow() *MockClock {
	return &MockClock{CurrentTime: time.Now()}
}
