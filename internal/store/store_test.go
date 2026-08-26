package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestMigrateCreatesRequiredTables(t *testing.T) {
	db := openTestStore(t)
	for _, table := range []string{"api_keys", "admin_sessions", "audit_logs"} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("table %s: %v", table, err)
		}
	}
}

func TestQueryAuditPageReportsFilteredTotal(t *testing.T) {
	db := openTestStore(t)
	now := time.Now().UTC()
	for _, id := range []string{"one", "two"} {
		if _, err := db.Exec(`INSERT INTO api_keys(id, name, prefix, secret_hash, created_at) VALUES (?, ?, ?, ?, ?)`, id, id, "cts_"+id, []byte(id), now.Unix()); err != nil {
			t.Fatal(err)
		}
	}
	for _, record := range []AuditRecord{
		{OccurredAt: now, APIKeyID: "one", SourceIP: "127.0.0.1", Method: "POST", Path: "/a", StatusCode: 200},
		{OccurredAt: now.Add(-time.Second), APIKeyID: "one", SourceIP: "127.0.0.1", Method: "POST", Path: "/a", StatusCode: 502},
		{OccurredAt: now.Add(-2 * time.Second), APIKeyID: "two", SourceIP: "127.0.0.1", Method: "POST", Path: "/b", StatusCode: 200},
	} {
		if err := db.InsertAudit(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}
	page, err := db.QueryAuditPage(context.Background(), AuditFilter{APIKeyID: "one", Limit: 1, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 1 || page.Items[0].StatusCode != 200 {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestAPIKeyActivity24HAggregatesRequests(t *testing.T) {
	db := openTestStore(t)
	now := time.Now().UTC()
	for _, id := range []string{"one", "two"} {
		if _, err := db.Exec(`INSERT INTO api_keys(id, name, prefix, secret_hash, created_at) VALUES (?, ?, ?, ?, ?)`, id, id, "cts_"+id, []byte(id), now.Unix()); err != nil {
			t.Fatal(err)
		}
	}
	for _, record := range []AuditRecord{
		{OccurredAt: now.Add(-time.Hour), APIKeyID: "one", SourceIP: "127.0.0.1", Method: "POST", Path: "/a", StatusCode: 200, DurationMS: 100},
		{OccurredAt: now.Add(-time.Minute), APIKeyID: "one", SourceIP: "127.0.0.1", Method: "POST", Path: "/b", StatusCode: 502, DurationMS: 301},
		{OccurredAt: now.Add(-25 * time.Hour), APIKeyID: "one", SourceIP: "127.0.0.1", Method: "POST", Path: "/old", StatusCode: 500, DurationMS: 900},
	} {
		if err := db.InsertAudit(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}
	activity, err := db.APIKeyActivity24H(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	got := activity["one"]
	if got.Requests24H != 2 || got.Errors24H != 1 || got.AverageLatencyMS != 200 || got.LastStatusCode != 502 {
		t.Fatalf("unexpected activity: %+v", got)
	}
}
