package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestConsumeLoginCaptchaRemovesExpiredChallenges(t *testing.T) {
	db := openTestStore(t)
	now := time.Now().UTC()
	if err := db.CreateLoginCaptcha(context.Background(), LoginCaptcha{ID: "expired", AnswerHash: []byte("hash"), CreatedAt: now.Add(-10 * time.Minute), ExpiresAt: now.Add(-5 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ConsumeLoginCaptcha(context.Background(), "missing", now); err != sql.ErrNoRows {
		t.Fatalf("consume error = %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM login_captchas WHERE id = 'expired'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expired captcha count = %d", count)
	}
}
