package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"cursor-tab-server/internal/store"
)

func TestSummaryAggregatesTwentyFourHourTraffic(t *testing.T) {
	database := openAuditTestStore(t)
	service := New(database)
	now := time.Date(2026, 8, 26, 15, 0, 0, 0, time.UTC)
	for index, record := range []store.AuditRecord{
		{OccurredAt: now.Add(-1 * time.Hour), SourceIP: "1", Method: "POST", Path: "/a", StatusCode: 200, DurationMS: 100, RequestBytes: 1, ResponseBytes: 1},
		{OccurredAt: now.Add(-2 * time.Hour), SourceIP: "2", Method: "POST", Path: "/a", StatusCode: 500, DurationMS: 301, RequestBytes: 1, ResponseBytes: 1},
		{OccurredAt: now.Add(-25 * time.Hour), SourceIP: "3", Method: "POST", Path: "/a", StatusCode: 200, DurationMS: 900, RequestBytes: 1, ResponseBytes: 1},
	} {
		record.ID = int64(index + 1)
		if err := service.Record(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}

	summary, err := service.Summary(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Requests24H != 2 || summary.Errors24H != 1 || summary.AverageLatencyMS != 200 || summary.SuccessRate != 50 {
		t.Fatalf("summary = %+v", summary)
	}
	if len(summary.StatusDistribution) != 2 {
		t.Fatalf("status distribution = %+v", summary.StatusDistribution)
	}
}

func openAuditTestStore(t *testing.T) *store.Store {
	t.Helper()
	database, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO api_keys (id, name, prefix, secret_hash, created_at) VALUES ('key', 'test', 'cts_test', ?, ?)`, []byte("hash"), time.Now().Unix()); err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	return database
}
