package audit

import (
	"context"
	"fmt"
	"time"

	"cursor-tab-server/internal/store"
)

type Record = store.AuditRecord
type Query = store.AuditFilter
type Page = store.AuditPage

type Summary struct {
	Requests24H        int64         `json:"requests_24h"`
	Errors24H          int64         `json:"errors_24h"`
	AverageLatencyMS   int64         `json:"average_latency_ms"`
	SuccessRate        float64       `json:"success_rate"`
	ActiveKeys24H      int64         `json:"active_keys_24h"`
	StatusDistribution []StatusCount `json:"status_distribution"`
}

type StatusCount struct {
	StatusCode int   `json:"status_code"`
	Count      int64 `json:"count"`
}

type Service struct{ store *store.Store }

func New(database *store.Store) *Service { return &Service{store: database} }

func (s *Service) Record(ctx context.Context, record Record) error {
	if record.OccurredAt.IsZero() {
		record.OccurredAt = time.Now().UTC()
	}
	if err := ValidateErrorKind(record.ErrorKind); err != nil {
		return err
	}
	return s.store.InsertAudit(ctx, record)
}

func (s *Service) Query(ctx context.Context, filter Query) ([]Record, error) {
	page, err := s.QueryPage(ctx, filter)
	return page.Items, err
}

func (s *Service) QueryPage(ctx context.Context, filter Query) (Page, error) {
	if filter.Limit < 1 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		return Page{}, fmt.Errorf("offset cannot be negative")
	}
	return s.store.QueryAuditPage(ctx, filter)
}

func (s *Service) Summary(ctx context.Context, now time.Time) (Summary, error) {
	since := now.Add(-24 * time.Hour)
	summary := Summary{StatusDistribution: []StatusCount{}}
	err := s.store.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END), 0), COALESCE(CAST(AVG(duration_ms) AS INTEGER), 0), COUNT(DISTINCT api_key_id) FROM audit_logs WHERE occurred_at > ?`, since.Unix()).Scan(&summary.Requests24H, &summary.Errors24H, &summary.AverageLatencyMS, &summary.ActiveKeys24H)
	if err != nil {
		return Summary{}, err
	}
	if summary.Requests24H > 0 {
		summary.SuccessRate = float64(summary.Requests24H-summary.Errors24H) * 100 / float64(summary.Requests24H)
	}
	rows, err := s.store.QueryContext(ctx, `SELECT status_code, COUNT(*) AS total FROM audit_logs WHERE occurred_at > ? GROUP BY status_code ORDER BY total DESC, status_code`, since.Unix())
	if err != nil {
		return Summary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var status StatusCount
		if err := rows.Scan(&status.StatusCode, &status.Count); err != nil {
			return Summary{}, err
		}
		summary.StatusDistribution = append(summary.StatusDistribution, status)
	}
	return summary, rows.Err()
}

func (s *Service) ActivityByKey(ctx context.Context, now time.Time) (map[string]store.APIKeyActivity, error) {
	return s.store.APIKeyActivity24H(ctx, now)
}

func (s *Service) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return s.store.DeleteAuditBefore(ctx, before)
}

func ValidateErrorKind(value string) error {
	switch value {
	case "", "unauthorized", "rate_limited", "upstream_failure", "request_too_large", "internal_error":
		return nil
	default:
		return fmt.Errorf("unsupported audit error kind")
	}
}
