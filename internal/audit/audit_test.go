package audit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cursor-tab-server/internal/store"
)

func TestCleanupRetainsOnlyLastThirtyDays(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	service := New(db)
	for _, occurredAt := range []time.Time{now.Add(-31 * 24 * time.Hour), now.Add(-29 * 24 * time.Hour)} {
		if err := service.Record(ctx, Record{OccurredAt: occurredAt, SourceIP: "127.0.0.1", Method: "GET", Path: "/allowed", StatusCode: 200}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.DeleteOlderThan(ctx, now.Add(-30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	logs, err := service.Query(ctx, Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("records = %d, want 1", len(logs))
	}
}

func TestRecordRejectsUntrustedErrorText(t *testing.T) {
	if err := ValidateErrorKind("database password=secret"); err == nil {
		t.Fatal("expected validation error")
	}
}
