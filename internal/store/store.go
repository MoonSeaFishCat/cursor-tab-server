package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	*sql.DB
}

func Open(path string) (*Store, error) {
	if err := ensureParentDirectory(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return &Store{DB: db}, nil
}

func ensureParentDirectory(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return osMkdirAll(dir)
}

func (s *Store) Migrate(ctx context.Context) error {
	for _, statement := range schema {
		if _, err := s.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate database: %w", err)
		}
	}
	return nil
}

var schema = []string{
	`CREATE TABLE IF NOT EXISTS settings (
		name TEXT PRIMARY KEY,
		value INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS cursor_tokens (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		ciphertext BLOB NOT NULL,
		nonce BLOB NOT NULL,
		masked TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		last_used_at INTEGER,
		last_error TEXT NOT NULL DEFAULT '',
		disabled_at INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cursor_tokens_enabled ON cursor_tokens(enabled, created_at)`,
	`CREATE TABLE IF NOT EXISTS api_keys (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		prefix TEXT NOT NULL,
		secret_hash BLOB NOT NULL UNIQUE,
		created_at INTEGER NOT NULL,
		disabled_at INTEGER,
		last_used_at INTEGER
	)`,
	`CREATE TABLE IF NOT EXISTS admin_sessions (
		id TEXT PRIMARY KEY,
		secret_hash BLOB NOT NULL UNIQUE,
		created_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS login_captchas (
		id TEXT PRIMARY KEY,
		answer_hash BLOB NOT NULL,
		created_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_login_captchas_expires_at ON login_captchas(expires_at)`,
	`CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		occurred_at INTEGER NOT NULL,
		api_key_id TEXT,
		source_ip TEXT NOT NULL,
		method TEXT NOT NULL,
		path TEXT NOT NULL,
		status_code INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL,
		request_bytes INTEGER NOT NULL,
		response_bytes INTEGER NOT NULL,
		error_kind TEXT NOT NULL DEFAULT '',
		FOREIGN KEY(api_key_id) REFERENCES api_keys(id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_api_keys_created_at ON api_keys(created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires_at ON admin_sessions(expires_at)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_logs_occurred_at ON audit_logs(occurred_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_logs_api_key_id ON audit_logs(api_key_id)`,
}
