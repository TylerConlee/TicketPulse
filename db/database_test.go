package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/TylerConlee/TicketPulse/db"
	"github.com/stretchr/testify/assert"
)

var testDB *db.SQLDatabase

// setup initializes the test database before each test
func setup() {
	testDB = db.InitDB(":memory:") // Use an in-memory database for testing
}

// teardown closes the test database after each test
func teardown() {
	testDB.Close()
}

func TestNewDatabase(t *testing.T) {
	setup()
	defer teardown()

	database := db.NewDatabase()

	assert.NotNil(t, database, "Expected database not to be nil")
	assert.IsType(t, &db.SQLDatabase{}, database, "Expected database to be of type *db.SQLDatabase")
}

func TestSQLDatabase_Close(t *testing.T) {
	setup()
	defer teardown()

	err := testDB.Close()
	assert.NoError(t, err, "Expected Close to not return an error")
}

func TestSQLDatabase_Beginx(t *testing.T) {
	setup()
	defer teardown()

	tx, err := testDB.Beginx()
	assert.NoError(t, err, "Expected Beginx to not return an error")
	assert.NotNil(t, tx, "Expected tx to be initialized")
}

func TestSQLDatabase_Preparex(t *testing.T) {
	setup()
	defer teardown()

	stmt, err := testDB.Preparex("SELECT 1")
	assert.NoError(t, err, "Expected Preparex to not return an error")
	assert.NotNil(t, stmt, "Expected stmt to be initialized")
}

func TestSQLDatabase_GetDB(t *testing.T) {
	setup()
	defer teardown()

	db := testDB.GetDB()
	assert.NotNil(t, db, "Expected *sqlx.DB to be returned")
}

func TestSQLDatabase_QueryRow(t *testing.T) {
	setup()
	defer teardown()

	row := testDB.QueryRow("SELECT 1")
	assert.NotNil(t, row, "Expected row to be initialized")

	var result int
	err := row.Scan(&result)
	assert.NoError(t, err, "Expected QueryRow to execute without error")
	assert.Equal(t, 1, result, "Expected result to be 1")
}

func TestSQLDatabase_Exec(t *testing.T) {
	setup()
	defer teardown()

	result, err := testDB.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT)")
	assert.NoError(t, err, "Expected Exec to not return an error")

	rowsAffected, err := result.RowsAffected()
	assert.NoError(t, err, "Expected RowsAffected to not return an error")
	assert.Equal(t, int64(0), rowsAffected, "Expected no rows to be affected")
}

func TestSQLDatabase_Query(t *testing.T) {
	setup()
	defer teardown()

	_, err := testDB.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT)")
	assert.NoError(t, err, "Expected table creation to not return an error")

	rows, err := testDB.Query("SELECT id FROM test")
	assert.NoError(t, err, "Expected Query to not return an error")
	assert.NotNil(t, rows, "Expected rows to be initialized")
}

func TestSQLDatabase_Select(t *testing.T) {
	setup()
	defer teardown()

	_, err := testDB.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT)")
	assert.NoError(t, err, "Expected table creation to not return an error")

	var ids []int
	err = testDB.Select(&ids, "SELECT id FROM test")
	assert.NoError(t, err, "Expected Select to not return an error")
	assert.Empty(t, ids, "Expected no results from empty table")
}

func TestSQLDatabase_NamedExec(t *testing.T) {
	setup()
	defer teardown()

	_, err := testDB.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	assert.NoError(t, err, "Expected table creation to not return an error")

	_, err = testDB.NamedExec("INSERT INTO test (name) VALUES (:name)", map[string]interface{}{"name": "test"})
	assert.NoError(t, err, "Expected NamedExec to not return an error")
}

func TestSQLDatabase_Get(t *testing.T) {
	setup()
	defer teardown()

	_, err := testDB.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	assert.NoError(t, err, "Expected table creation to not return an error")

	_, err = testDB.Exec("INSERT INTO test (name) VALUES ('test')")
	assert.NoError(t, err, "Expected row insertion to not return an error")

	var name string
	err = testDB.Get(&name, "SELECT name FROM test WHERE id = 1")
	assert.NoError(t, err, "Expected Get to not return an error")
	assert.Equal(t, "test", name, "Expected name to be 'test'")
}

func TestSQLDatabase_ExecContext(t *testing.T) {
	setup()
	defer teardown()

	ctx := context.Background()
	_, err := testDB.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT)")
	assert.NoError(t, err, "Expected ExecContext to not return an error")
}

func TestSQLDatabase_QueryRowContext(t *testing.T) {
	setup()
	defer teardown()

	ctx := context.Background()
	row := testDB.QueryRowContext(ctx, "SELECT 1")
	assert.NotNil(t, row, "Expected row to be initialized")

	var result int
	err := row.Scan(&result)
	assert.NoError(t, err, "Expected QueryRowContext to execute without error")
	assert.Equal(t, 1, result, "Expected result to be 1")
}

func TestInitDB(t *testing.T) {
	// Set up a temporary database file
	tmpFile, err := os.CreateTemp("", "testdb-*.sqlite")
	assert.NoError(t, err, "Expected no error creating temporary database file")
	defer os.Remove(tmpFile.Name()) // Clean up the file afterward

	// Initialize the database
	database := db.InitDB(tmpFile.Name())

	// Verify that the database is initialized and tables are created
	assert.NotNil(t, database, "Expected the database to be initialized")
	assert.NotNil(t, database.GetDB(), "Expected the underlying *sqlx.DB to be initialized")

	// Check that the tables exist
	tables := []string{"users", "user_tag_alerts", "configuration", "alert_logs", "sla_alert_cache"}
	for _, table := range tables {
		var tableName string
		err := database.Get(&tableName, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table)
		assert.NoError(t, err, "Expected table %s to exist", table)
		assert.Equal(t, table, tableName, "Expected table name to match")
	}
}
