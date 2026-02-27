// Package database provides SQLite-based storage for mee.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection.
type DB struct {
	*sql.DB
	mu sync.RWMutex
}

// Open creates or opens the database at the given path.
func Open(path string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		// Non-fatal, continue
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite works best with single writer
	db.SetMaxIdleConns(1)

	return &DB{DB: db}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.DB.Close()
}

// Migrate runs all database migrations.
func (db *DB) Migrate() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Create plugins table
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    url TEXT,
    local_path TEXT NOT NULL,
    runtime TEXT DEFAULT 'yaegi',
    trigger TEXT,
    description TEXT,
    installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    enabled INTEGER DEFAULT 1
)`)
	if err != nil {
		return fmt.Errorf("failed to create plugins table: %w", err)
	}

	// Create history table
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plugin_id TEXT,
    query TEXT NOT NULL,
    selected_item TEXT,
    selected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    selection_count INTEGER DEFAULT 1
)`)
	if err != nil {
		return fmt.Errorf("failed to create history table: %w", err)
	}

	// Create index for faster history queries
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_history_query ON history(query)`)
	if err != nil {
		return fmt.Errorf("failed to create history index: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_history_plugin ON history(plugin_id)`)
	if err != nil {
		return fmt.Errorf("failed to create plugin index: %w", err)
	}

	return nil
}

// Tx executes a function within a transaction.
func (db *DB) Tx(fn func(*sql.Tx) error) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
