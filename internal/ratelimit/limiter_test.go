package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterSeparatesSubjects(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	limiter := New(1, time.Minute)
	if !limiter.Allow("key-a|127.0.0.1", now) || limiter.Allow("key-a|127.0.0.1", now) || !limiter.Allow("key-b|127.0.0.1", now) {
		t.Fatal("unexpected limits")
	}
}

func TestLimiterResetsAtNextWindow(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 30, 0, time.UTC)
	limiter := New(1, time.Minute)
	if !limiter.Allow("subject", now) || !limiter.Allow("subject", now.Add(time.Minute)) {
		t.Fatal("expected one request per window")
	}
}
