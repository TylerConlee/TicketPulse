package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextKeyType(t *testing.T) {
	// Verify the context key is defined correctly
	assert.Equal(t, contextKey("user_id"), userIDKey)
}

func TestGetUserIDFromContext_ValidUserID(t *testing.T) {
	ctx := context.WithValue(context.Background(), userIDKey, 123)
	userID, ok := GetUserIDFromContext(ctx)

	assert.True(t, ok)
	assert.Equal(t, 123, userID)
}

func TestGetUserIDFromContext_NoUserID(t *testing.T) {
	ctx := context.Background()
	userID, ok := GetUserIDFromContext(ctx)

	assert.False(t, ok)
	assert.Equal(t, 0, userID)
}

func TestGetUserIDFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), userIDKey, "not-an-int")
	userID, ok := GetUserIDFromContext(ctx)

	assert.False(t, ok)
	assert.Equal(t, 0, userID)
}

func TestGetUserIDFromContext_NilContext(t *testing.T) {
	// Test with a context that has a nil value for the key
	ctx := context.WithValue(context.Background(), userIDKey, nil)
	userID, ok := GetUserIDFromContext(ctx)

	assert.False(t, ok)
	assert.Equal(t, 0, userID)
}

func TestGetUserIDFromContext_DifferentKey(t *testing.T) {
	// Test with a different key - should not find user ID
	differentKey := contextKey("different_key")
	ctx := context.WithValue(context.Background(), differentKey, 456)
	userID, ok := GetUserIDFromContext(ctx)

	assert.False(t, ok)
	assert.Equal(t, 0, userID)
}
