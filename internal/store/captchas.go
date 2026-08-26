package store

import (
	"context"
	"time"
)

type LoginCaptcha struct {
	ID         string
	AnswerHash []byte
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

func (s *Store) CreateLoginCaptcha(ctx context.Context, captcha LoginCaptcha) error {
	_, err := s.ExecContext(ctx, `INSERT INTO login_captchas (id, answer_hash, created_at, expires_at) VALUES (?, ?, ?, ?)`, captcha.ID, captcha.AnswerHash, captcha.CreatedAt.UTC().Unix(), captcha.ExpiresAt.UTC().Unix())
	return err
}

func (s *Store) DeleteExpiredLoginCaptchas(ctx context.Context, now time.Time) error {
	_, err := s.ExecContext(ctx, `DELETE FROM login_captchas WHERE expires_at <= ?`, now.UTC().Unix())
	return err
}

func (s *Store) ConsumeLoginCaptcha(ctx context.Context, id string, now time.Time) (LoginCaptcha, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return LoginCaptcha{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM login_captchas WHERE expires_at <= ?`, now.UTC().Unix()); err != nil {
		return LoginCaptcha{}, err
	}
	var captcha LoginCaptcha
	var createdAt, expiresAt int64
	err = tx.QueryRowContext(ctx, `SELECT id, answer_hash, created_at, expires_at FROM login_captchas WHERE id = ? AND expires_at > ?`, id, now.UTC().Unix()).Scan(&captcha.ID, &captcha.AnswerHash, &createdAt, &expiresAt)
	if err != nil {
		if commitErr := tx.Commit(); commitErr != nil {
			return LoginCaptcha{}, commitErr
		}
		return LoginCaptcha{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM login_captchas WHERE id = ?`, id); err != nil {
		return LoginCaptcha{}, err
	}
	if err = tx.Commit(); err != nil {
		return LoginCaptcha{}, err
	}
	captcha.CreatedAt = time.Unix(createdAt, 0).UTC()
	captcha.ExpiresAt = time.Unix(expiresAt, 0).UTC()
	return captcha, nil
}
