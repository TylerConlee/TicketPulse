// Package testutil provides utilities for testing TicketPulse components.
package testutil

import (
	"github.com/TylerConlee/TicketPulse/db"
)

// SetupTestDB creates an in-memory SQLite database for testing.
// The database is initialized with all required tables.
func SetupTestDB() *db.SQLDatabase {
	return db.InitDB(":memory:")
}

// CleanupTestDB closes the test database connection.
func CleanupTestDB(database *db.SQLDatabase) {
	if database != nil {
		database.Close()
	}
}

// TestDBHelper provides a helper struct for managing test databases.
type TestDBHelper struct {
	DB *db.SQLDatabase
}

// NewTestDBHelper creates a new test database helper with an in-memory database.
func NewTestDBHelper() *TestDBHelper {
	return &TestDBHelper{
		DB: SetupTestDB(),
	}
}

// Cleanup closes the database connection.
func (h *TestDBHelper) Cleanup() {
	CleanupTestDB(h.DB)
}

// GetDB returns the underlying database.
func (h *TestDBHelper) GetDB() *db.SQLDatabase {
	return h.DB
}
