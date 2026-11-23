// Package db persists Warden Web UI's administrator and browser sessions.
// It never reads or modifies the Warden application's database.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// ErrNotFound indicates that an administrator or browser session does not exist.
var ErrNotFound = errors.New("record not found")

// ErrSetupComplete means the initial administrator has already been created.
var ErrSetupComplete = errors.New("initial setup is already complete")

// Store owns the dedicated Web UI SQLite connection.
type Store struct {
	// db serializes writes to the small, Web UI database.
	db *sql.DB
}

// Open initializes the private Web UI database and its schema.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open database file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close database file: %w", err)
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}

	// Escape filenames and reapply connection-local settings after reconnection.
	location := url.URL{Scheme: "file", Path: absolute}
	location.RawQuery = url.Values{
		"_pragma": {"busy_timeout(5000)", "foreign_keys(1)"},
	}.Encode()

	conn, err := sql.Open("sqlite", location.String())
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	conn.SetMaxOpenConns(1)

	store := &Store{db: conn}
	if err := store.initialize(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return store, nil
}

// Close releases SQLite resources after HTTP listeners have stopped.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping checks that the local administrator store is responsive.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// initialize creates the initial schema and configures safe transactional behavior.
func (s *Store) initialize(ctx context.Context) error {
	for _, statement := range []string{
		`
			PRAGMA journal_mode = WAL
		`,
		`
			CREATE TABLE IF NOT EXISTS sessions (
			    token_hash TEXT PRIMARY KEY,
			    username TEXT NOT NULL,
			    is_local INTEGER NOT NULL CHECK (is_local IN (0, 1)),
			    expires_at INTEGER NOT NULL
			)
		`,
		`
			CREATE INDEX IF NOT EXISTS sessions_expires_at
			    ON sessions(expires_at)
		`,
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize Web UI database: %w", err)
		}
	}
	return s.initializeUsers(ctx)
}
