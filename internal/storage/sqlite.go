package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/itportal/it-portal-agent/internal/config"
	_ "modernc.org/sqlite"
)

func Open() (*sql.DB, error) {
	db, err := sql.Open("sqlite", filepath.Join(config.RootDir(), "agent.db"))
	if err != nil { return nil, fmt.Errorf("open database: %w", err) }
	if err := db.Ping(); err != nil { db.Close(); return nil, fmt.Errorf("ping database: %w", err) }
	for _, statement := range schema {
		if _, err := db.Exec(statement); err != nil { db.Close(); return nil, fmt.Errorf("migrate database: %w", err) }
	}
	return db, nil
}

var schema = []string{
	`CREATE TABLE IF NOT EXISTS queue (id INTEGER PRIMARY KEY, type TEXT NOT NULL, payload BLOB, status TEXT NOT NULL DEFAULT 'pending', created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS cache (key TEXT PRIMARY KEY, value BLOB, expires_at TEXT)`,
}
