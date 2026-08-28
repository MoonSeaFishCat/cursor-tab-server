package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	SettingProxyRatePerMinute   = "proxy_rate_per_minute"
	SettingAdminRatePerMinute   = "admin_rate_per_minute"
	SettingCaptchaRatePerMinute = "captcha_rate_per_minute"
	SettingLoginRatePerMinute   = "login_rate_per_minute"
	SettingLogRetentionDays     = "log_retention_days"
	SettingCursorToken          = "cursor_token"
	SettingAllowAnonymousProxy  = "allow_anonymous_proxy"
)

// SettingInt reads an integer override from the settings table. The second
// return value reports whether an override exists.
func (s *Store) SettingInt(ctx context.Context, name string) (int, bool, error) {
	var value int
	err := s.QueryRowContext(ctx, `SELECT value FROM settings WHERE name = ?`, name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read setting %q: %w", name, err)
	}
	return value, true, nil
}

// SaveSettingInt upserts an integer override so it survives restarts.
func (s *Store) SaveSettingInt(ctx context.Context, name string, value int) error {
	_, err := s.ExecContext(ctx, `INSERT INTO settings(name, value, updated_at) VALUES(?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, name, value, time.Now().UTC().Unix())
	if err != nil {
		return fmt.Errorf("save setting %q: %w", name, err)
	}
	return nil
}

// SettingString reads a string override from the settings table. The value
// column has INTEGER affinity, but SQLite stores non-numeric strings as TEXT,
// so secrets such as tokens round-trip unchanged.
func (s *Store) SettingString(ctx context.Context, name string) (string, bool, error) {
	var value string
	err := s.QueryRowContext(ctx, `SELECT value FROM settings WHERE name = ?`, name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read setting %q: %w", name, err)
	}
	return value, true, nil
}

// SaveSettingString upserts a string override so it survives restarts.
func (s *Store) SaveSettingString(ctx context.Context, name, value string) error {
	_, err := s.ExecContext(ctx, `INSERT INTO settings(name, value, updated_at) VALUES(?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, name, value, time.Now().UTC().Unix())
	if err != nil {
		return fmt.Errorf("save setting %q: %w", name, err)
	}
	return nil
}
