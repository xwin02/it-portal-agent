package storage

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"path/filepath"

	"github.com/itportal/it-portal-agent/internal/config"
	"github.com/itportal/it-portal-agent/internal/security"
	_ "modernc.org/sqlite"
)

func Open() (*sql.DB, error) {
	db, err := sql.Open("sqlite", filepath.Join(config.RootDir(), "agent.db"))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	for _, statement := range schema {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate database: %w", err)
		}
	}
	return db, nil
}

func Put(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT INTO config(key, value, updated_at) VALUES(?, ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value)
	if err != nil {
		return fmt.Errorf("store %s: %w", key, err)
	}
	return nil
}

func Get(db *sql.DB, key string) (string, bool, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", key, err)
	}
	return value, true, nil
}

func PutSecret(db *sql.DB, key, value string) error {
	protected, err := security.ProtectSecret([]byte(value))
	if err != nil {
		return fmt.Errorf("protect %s: %w", key, err)
	}
	return Put(db, key, base64.StdEncoding.EncodeToString(protected))
}

func GetSecret(db *sql.DB, key string) (string, bool, error) {
	encoded, found, err := Get(db, key)
	if err != nil || !found {
		return "", found, err
	}
	protected, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false, fmt.Errorf("decode protected %s: %w", key, err)
	}
	plain, err := security.UnprotectSecret(protected)
	if err != nil {
		return "", false, fmt.Errorf("unprotect %s: %w", key, err)
	}
	return string(plain), true, nil
}

var schema = []string{
	`CREATE TABLE IF NOT EXISTS queue (id INTEGER PRIMARY KEY, type TEXT NOT NULL, payload BLOB, status TEXT NOT NULL DEFAULT 'pending', created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS cache (key TEXT PRIMARY KEY, value BLOB, expires_at TEXT)`,
}
