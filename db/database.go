package db

import (
	"context"
	"database/sql"
	"log"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var DB *SQLDatabase

type Database interface {
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Select(dest interface{}, query string, args ...interface{}) error
	NamedExec(query string, arg interface{}) (sql.Result, error)
	Get(dest interface{}, query string, args ...interface{}) error
	Close() error
	Beginx() (*sqlx.Tx, error)                 // Changed from *sql.Tx to *sqlx.Tx
	Preparex(query string) (*sqlx.Stmt, error) // Changed from *sql.Stmt to *sqlx.Stmt
	GetDB() *sqlx.DB
}

type SQLDatabase struct {
	DB *sqlx.DB // Use sqlx.DB instead of sql.DB
}

func NewDatabase() Database {
	return DB
}
func (s *SQLDatabase) Close() error {
	return s.DB.Close()
}

func (s *SQLDatabase) Beginx() (*sqlx.Tx, error) {
	return s.DB.Beginx() // Returning *sqlx.Tx
}

func (s *SQLDatabase) Preparex(query string) (*sqlx.Stmt, error) {
	return s.DB.Preparex(query) // Returning *sqlx.Stmt
}

func (s *SQLDatabase) GetDB() *sqlx.DB {
	return s.DB
}

func (s *SQLDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	return s.DB.QueryRow(query, args...)
}

func (s *SQLDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	return s.DB.Exec(query, args...)
}

func (s *SQLDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.DB.Query(query, args...)
}

func (s *SQLDatabase) Select(dest interface{}, query string, args ...interface{}) error {
	return s.DB.Select(dest, query, args...)
}

func (s *SQLDatabase) NamedExec(query string, arg interface{}) (sql.Result, error) {
	return s.DB.NamedExec(query, arg)
}

func (s *SQLDatabase) Get(dest interface{}, query string, args ...interface{}) error {
	return s.DB.Get(dest, query, args...)
}

func (s *SQLDatabase) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.DB.ExecContext(ctx, query, args...)
}

func (s *SQLDatabase) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.DB.QueryRowContext(ctx, query, args...)
}

func InitDB(filepath string) *SQLDatabase {
	db, err := sqlx.Open("sqlite", filepath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	sqlDB := &SQLDatabase{DB: db}

	// Initialize tables
	if err := sqlDB.initTables(); err != nil {
		log.Fatal("Error initializing tables:", err)
	}

	DB = sqlDB
	return sqlDB
}
func (s *SQLDatabase) initTables() error {
	tablesSQL := []string{
		`CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            email TEXT UNIQUE,
            name TEXT,
            role TEXT,
            daily_summary BOOLEAN,
            selected_tags TEXT,
            summary_time DATETIME,
            slack_user_id TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS user_tag_alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			tag TEXT NOT NULL,
			slack_channel_id TEXT NOT NULL,
			alert_type TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS configuration (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS alert_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, 
			user_id INTEGER NOT NULL,   
			ticket_id INTEGER NOT NULL,
			tag TEXT NOT NULL,  
			alert_type TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS sla_alert_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT, -- Use INTEGER for AUTOINCREMENT
			user_id INT NOT NULL,
			ticket_id INT NOT NULL,
			alert_type VARCHAR(255) NOT NULL,
			breach_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, 
			UNIQUE(user_id, ticket_id, alert_type),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
	}

	for _, stmt := range tablesSQL {
		if _, err := s.Exec(stmt); err != nil {
			return err // Return the error to be handled by the caller
		}
	}

	return nil
}
