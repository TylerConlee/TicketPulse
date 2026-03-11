package db

import (
	"context"
	"database/sql"
	"embed"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

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
	Beginx() (*sqlx.Tx, error)
	Preparex(query string) (*sqlx.Stmt, error)
	GetDB() *sqlx.DB
}

type SQLDatabase struct {
	DB *sqlx.DB
}

func NewDatabase() Database {
	return DB
}

func (s *SQLDatabase) Close() error {
	return s.DB.Close()
}

func (s *SQLDatabase) Beginx() (*sqlx.Tx, error) {
	return s.DB.Beginx()
}

func (s *SQLDatabase) Preparex(query string) (*sqlx.Stmt, error) {
	return s.DB.Preparex(query)
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

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	sqlDB := &SQLDatabase{DB: db}

	if err := runMigrations(db.DB); err != nil {
		log.Fatal("Error running migrations:", err)
	}

	DB = sqlDB
	return sqlDB
}

func runMigrations(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}

	return goose.Up(db, "migrations")
}
