package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomState(t *testing.T) {
	// Test that randomState generates a non-empty string
	state1 := randomState()
	state2 := randomState()

	assert.NotEmpty(t, state1)
	assert.NotEmpty(t, state2)
	// Two calls should produce different values (with very high probability)
	assert.NotEqual(t, state1, state2)
}

func TestRandomState_Length(t *testing.T) {
	// Random state should be base64 encoded 16 bytes
	// Base64 encoding of 16 bytes produces 22-24 characters
	state := randomState()

	assert.GreaterOrEqual(t, len(state), 20)
	assert.LessOrEqual(t, len(state), 24)
}

func TestRandomState_MultipleGenerations(t *testing.T) {
	// Generate multiple states and ensure they're all unique
	states := make(map[string]bool)
	for i := 0; i < 100; i++ {
		state := randomState()
		assert.False(t, states[state], "Generated duplicate state")
		states[state] = true
	}
}
