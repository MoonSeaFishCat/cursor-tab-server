package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	SecretHash []byte     `json:"-"`
	CreatedAt  time.Time  `json:"created_at"`
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func (s *Store) CreateAPIKey(ctx context.Context, key APIKey) error {
	_, err := s.ExecContext(ctx, `INSERT INTO api_keys (id, name, prefix, secret_hash, created_at) VALUES (?, ?, ?, ?, ?)`, key.ID, key.Name, key.Prefix, key.SecretHash, key.CreatedAt.UTC().Unix())
	return err
}

func (s *Store) FindActiveAPIKeyByHash(ctx context.Context, hash []byte) (APIKey, error) {
	var key APIKey
	var createdAt int64
	var disabledAt, lastUsedAt sql.NullInt64
	err := s.QueryRowContext(ctx, `SELECT id, name, prefix, secret_hash, created_at, disabled_at, last_used_at FROM api_keys WHERE secret_hash = ? AND disabled_at IS NULL`, hash).Scan(&key.ID, &key.Name, &key.Prefix, &key.SecretHash, &createdAt, &disabledAt, &lastUsedAt)
	if err != nil {
		return APIKey{}, err
	}
	key.CreatedAt = time.Unix(createdAt, 0).UTC()
	key.DisabledAt = nullUnix(disabledAt)
	key.LastUsedAt = nullUnix(lastUsedAt)
	return key, nil
}

func (s *Store) GetAPIKey(ctx context.Context, id string) (APIKey, error) {
	var key APIKey
	var createdAt int64
	var disabledAt, lastUsedAt sql.NullInt64
	err := s.QueryRowContext(ctx, `SELECT id, name, prefix, created_at, disabled_at, last_used_at FROM api_keys WHERE id = ?`, id).Scan(&key.ID, &key.Name, &key.Prefix, &createdAt, &disabledAt, &lastUsedAt)
	if err != nil {
		return APIKey{}, err
	}
	key.CreatedAt = time.Unix(createdAt, 0).UTC()
	key.DisabledAt = nullUnix(disabledAt)
	key.LastUsedAt = nullUnix(lastUsedAt)
	return key, nil
}

func (s *Store) ListAPIKeys(ctx context.Context, limit, offset int) ([]APIKey, error) {
	rows, err := s.QueryContext(ctx, `SELECT id, name, prefix, created_at, disabled_at, last_used_at FROM api_keys ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make([]APIKey, 0)
	for rows.Next() {
		var key APIKey
		var createdAt int64
		var disabledAt, lastUsedAt sql.NullInt64
		if err := rows.Scan(&key.ID, &key.Name, &key.Prefix, &createdAt, &disabledAt, &lastUsedAt); err != nil {
			return nil, err
		}
		key.CreatedAt = time.Unix(createdAt, 0).UTC()
		key.DisabledAt, key.LastUsedAt = nullUnix(disabledAt), nullUnix(lastUsedAt)
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *Store) APIKeyCount(ctx context.Context) (int64, error) {
	var count int64
	if err := s.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) DisableAPIKey(ctx context.Context, id string, now time.Time) error {
	result, err := s.ExecContext(ctx, `UPDATE api_keys SET disabled_at = COALESCE(disabled_at, ?) WHERE id = ?`, now.UTC().Unix(), id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) EnableAPIKey(ctx context.Context, id string) error {
	result, err := s.ExecContext(ctx, `UPDATE api_keys SET disabled_at = NULL WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SetAPIKeysEnabled(ctx context.Context, ids []string, enabled bool, now time.Time) error {
	if len(ids) == 0 {
		return fmt.Errorf("no API keys selected")
	}
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		var result sql.Result
		if enabled {
			result, err = tx.ExecContext(ctx, `UPDATE api_keys SET disabled_at = NULL WHERE id = ?`, id)
		} else {
			result, err = tx.ExecContext(ctx, `UPDATE api_keys SET disabled_at = COALESCE(disabled_at, ?) WHERE id = ?`, now.UTC().Unix(), id)
		}
		if err != nil {
			return err
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil || affected == 0 {
			return sql.ErrNoRows
		}
	}
	return tx.Commit()
}

// DeleteAPIKeys removes keys while preserving their audit records as anonymous
// records. This keeps audit history available without violating the foreign key.
func (s *Store) DeleteAPIKeys(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("no API keys selected")
	}
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE audit_logs SET api_key_id = NULL WHERE api_key_id = ?`, id); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			return sql.ErrNoRows
		}
	}
	return tx.Commit()
}

func (s *Store) MarkAPIKeyUsed(ctx context.Context, id string, now time.Time) error {
	_, err := s.ExecContext(ctx, `UPDATE api_keys SET last_used_at = ? WHERE id = ?`, now.UTC().Unix(), id)
	return err
}

func nullUnix(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	result := time.Unix(value.Int64, 0).UTC()
	return &result
}
