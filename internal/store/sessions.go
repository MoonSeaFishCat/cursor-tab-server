package store

import (
	"context"
	"database/sql"
	"time"
)

type AdminSession struct {
	ID         string
	SecretHash []byte
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

func (s *Store) CreateAdminSession(ctx context.Context, session AdminSession) error {
	_, err := s.ExecContext(ctx, `INSERT INTO admin_sessions (id, secret_hash, created_at, expires_at) VALUES (?, ?, ?, ?)`, session.ID, session.SecretHash, session.CreatedAt.UTC().Unix(), session.ExpiresAt.UTC().Unix())
	return err
}

func (s *Store) FindActiveAdminSession(ctx context.Context, hash []byte, now time.Time) (AdminSession, error) {
	_, _ = s.ExecContext(ctx, `DELETE FROM admin_sessions WHERE expires_at <= ?`, now.UTC().Unix())
	var session AdminSession
	var createdAt, expiresAt int64
	err := s.QueryRowContext(ctx, `SELECT id, secret_hash, created_at, expires_at FROM admin_sessions WHERE secret_hash = ? AND expires_at > ?`, hash, now.UTC().Unix()).Scan(&session.ID, &session.SecretHash, &createdAt, &expiresAt)
	if err != nil {
		return AdminSession{}, err
	}
	session.CreatedAt, session.ExpiresAt = time.Unix(createdAt, 0).UTC(), time.Unix(expiresAt, 0).UTC()
	return session, nil
}

func (s *Store) DeleteAdminSession(ctx context.Context, hash []byte) error {
	_, err := s.ExecContext(ctx, `DELETE FROM admin_sessions WHERE secret_hash = ?`, hash)
	return err
}

func IsNotFound(err error) bool { return err == sql.ErrNoRows }
