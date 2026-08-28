package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type CursorToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Ciphertext []byte     `json:"-"`
	Nonce      []byte     `json:"-"`
	Masked     string     `json:"masked"`
	Enabled    bool       `json:"enabled"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
}

func (s *Store) CreateCursorToken(ctx context.Context, token CursorToken) error {
	now := token.CreatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	_, err := s.ExecContext(ctx, `INSERT INTO cursor_tokens(id, name, ciphertext, nonce, masked, enabled, created_at, updated_at, last_error) VALUES(?, ?, ?, ?, ?, ?, ?, ?, '')`, token.ID, token.Name, token.Ciphertext, token.Nonce, token.Masked, token.Enabled, now.Unix(), now.Unix())
	if err != nil {
		return fmt.Errorf("create cursor token: %w", err)
	}
	return nil
}

func (s *Store) ListCursorTokens(ctx context.Context) ([]CursorToken, error) {
	rows, err := s.QueryContext(ctx, `SELECT id, name, ciphertext, nonce, masked, enabled, created_at, updated_at, last_used_at, last_error, disabled_at FROM cursor_tokens ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list cursor tokens: %w", err)
	}
	defer rows.Close()
	items := make([]CursorToken, 0)
	for rows.Next() {
		token, err := scanCursorToken(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, token)
	}
	return items, rows.Err()
}

func (s *Store) ActiveCursorTokens(ctx context.Context) ([]CursorToken, error) {
	rows, err := s.QueryContext(ctx, `SELECT id, name, ciphertext, nonce, masked, enabled, created_at, updated_at, last_used_at, last_error, disabled_at FROM cursor_tokens WHERE enabled = 1 AND disabled_at IS NULL ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list active cursor tokens: %w", err)
	}
	defer rows.Close()
	items := make([]CursorToken, 0)
	for rows.Next() {
		token, err := scanCursorToken(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, token)
	}
	return items, rows.Err()
}

func (s *Store) CursorTokenCount(ctx context.Context) (int, error) {
	var count int
	if err := s.QueryRowContext(ctx, `SELECT COUNT(*) FROM cursor_tokens`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count cursor tokens: %w", err)
	}
	return count, nil
}

func (s *Store) SetCursorTokenEnabled(ctx context.Context, id string, enabled bool) error {
	var disabled any
	if !enabled {
		disabled = time.Now().UTC().Unix()
	}
	result, err := s.ExecContext(ctx, `UPDATE cursor_tokens SET enabled = ?, disabled_at = ?, updated_at = ? WHERE id = ?`, enabled, disabled, time.Now().UTC().Unix(), id)
	if err != nil {
		return fmt.Errorf("update cursor token: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteCursorToken(ctx context.Context, id string) error {
	result, err := s.ExecContext(ctx, `DELETE FROM cursor_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete cursor token: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) MarkCursorTokenUsed(ctx context.Context, id string, now time.Time) error {
	_, err := s.ExecContext(ctx, `UPDATE cursor_tokens SET last_used_at = ?, last_error = '', updated_at = ? WHERE id = ?`, now.UTC().Unix(), now.UTC().Unix(), id)
	return err
}

func (s *Store) MarkCursorTokenError(ctx context.Context, id, message string, now time.Time) error {
	_, err := s.ExecContext(ctx, `UPDATE cursor_tokens SET last_error = ?, updated_at = ? WHERE id = ?`, message, now.UTC().Unix(), id)
	return err
}

func scanCursorToken(scanner interface{ Scan(...any) error }) (CursorToken, error) {
	var token CursorToken
	var enabled bool
	var createdAt, updatedAt int64
	var lastUsedAt, disabledAt sql.NullInt64
	if err := scanner.Scan(&token.ID, &token.Name, &token.Ciphertext, &token.Nonce, &token.Masked, &enabled, &createdAt, &updatedAt, &lastUsedAt, &token.LastError, &disabledAt); err != nil {
		return CursorToken{}, fmt.Errorf("scan cursor token: %w", err)
	}
	token.Enabled = enabled
	token.CreatedAt = time.Unix(createdAt, 0).UTC()
	token.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	token.LastUsedAt = nullUnix(lastUsedAt)
	token.DisabledAt = nullUnix(disabledAt)
	return token, nil
}

func (s *Store) DeleteSetting(ctx context.Context, name string) error {
	_, err := s.ExecContext(ctx, `DELETE FROM settings WHERE name = ?`, name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("delete setting %q: %w", name, err)
	}
	return nil
}
