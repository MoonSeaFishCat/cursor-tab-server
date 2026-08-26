package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type AuditRecord struct {
	ID            int64     `json:"id"`
	OccurredAt    time.Time `json:"occurred_at"`
	APIKeyID      string    `json:"api_key_id,omitempty"`
	SourceIP      string    `json:"source_ip"`
	Method        string    `json:"method"`
	Path          string    `json:"path"`
	StatusCode    int       `json:"status_code"`
	DurationMS    int64     `json:"duration_ms"`
	RequestBytes  int64     `json:"request_bytes"`
	ResponseBytes int64     `json:"response_bytes"`
	ErrorKind     string    `json:"error_kind,omitempty"`
}

type AuditFilter struct {
	APIKeyID   string
	Path       string
	StatusCode int
	Limit      int
	Offset     int
}

type AuditPage struct {
	Items []AuditRecord
	Total int64
}

type APIKeyActivity struct {
	APIKeyID         string `json:"api_key_id"`
	Requests24H      int64  `json:"requests_24h"`
	Errors24H        int64  `json:"errors_24h"`
	AverageLatencyMS int64  `json:"average_latency_ms"`
	LastStatusCode   int    `json:"last_status_code"`
}

func (s *Store) InsertAudit(ctx context.Context, record AuditRecord) error {
	_, err := s.ExecContext(ctx, `INSERT INTO audit_logs (occurred_at, api_key_id, source_ip, method, path, status_code, duration_ms, request_bytes, response_bytes, error_kind) VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?)`, record.OccurredAt.UTC().Unix(), record.APIKeyID, record.SourceIP, record.Method, record.Path, record.StatusCode, record.DurationMS, record.RequestBytes, record.ResponseBytes, record.ErrorKind)
	return err
}

func auditWhere(filter AuditFilter) (string, []any) {
	var clauses []string
	var args []any
	if filter.APIKeyID != "" {
		clauses, args = append(clauses, "api_key_id = ?"), append(args, filter.APIKeyID)
	}
	if filter.Path != "" {
		clauses, args = append(clauses, "path = ?"), append(args, filter.Path)
	}
	if filter.StatusCode != 0 {
		clauses, args = append(clauses, "status_code = ?"), append(args, filter.StatusCode)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (s *Store) QueryAudit(ctx context.Context, filter AuditFilter) ([]AuditRecord, error) {
	page, err := s.QueryAuditPage(ctx, filter)
	return page.Items, err
}

// QueryAuditPage returns a page of matching audit records and the unpaginated
// total so administrative clients can render accurate pagination controls.
func (s *Store) QueryAuditPage(ctx context.Context, filter AuditFilter) (AuditPage, error) {
	where, args := auditWhere(filter)
	var page AuditPage
	if err := s.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`+where, args...).Scan(&page.Total); err != nil {
		return AuditPage{}, err
	}
	queryArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	rows, err := s.QueryContext(ctx, `SELECT id, occurred_at, COALESCE(api_key_id, ''), source_ip, method, path, status_code, duration_ms, request_bytes, response_bytes, error_kind FROM audit_logs`+where+` ORDER BY occurred_at DESC, id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return AuditPage{}, err
	}
	defer rows.Close()
	page.Items = make([]AuditRecord, 0)
	for rows.Next() {
		var record AuditRecord
		var occurredAt int64
		if err := rows.Scan(&record.ID, &occurredAt, &record.APIKeyID, &record.SourceIP, &record.Method, &record.Path, &record.StatusCode, &record.DurationMS, &record.RequestBytes, &record.ResponseBytes, &record.ErrorKind); err != nil {
			return AuditPage{}, err
		}
		record.OccurredAt = time.Unix(occurredAt, 0).UTC()
		page.Items = append(page.Items, record)
	}
	if err := rows.Err(); err != nil {
		return AuditPage{}, err
	}
	return page, nil
}

// APIKeyActivity24H aggregates the last 24 hours of traffic by API key.
func (s *Store) APIKeyActivity24H(ctx context.Context, now time.Time) (map[string]APIKeyActivity, error) {
	rows, err := s.QueryContext(ctx, `SELECT api_key_id, COUNT(*), COALESCE(SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END), 0), COALESCE(CAST(AVG(duration_ms) AS INTEGER), 0),
		(SELECT status_code FROM audit_logs latest WHERE latest.api_key_id = audit_logs.api_key_id ORDER BY occurred_at DESC, id DESC LIMIT 1)
		FROM audit_logs WHERE occurred_at > ? AND api_key_id IS NOT NULL GROUP BY api_key_id`, now.UTC().Add(-24*time.Hour).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	activity := make(map[string]APIKeyActivity)
	for rows.Next() {
		var value APIKeyActivity
		if err := rows.Scan(&value.APIKeyID, &value.Requests24H, &value.Errors24H, &value.AverageLatencyMS, &value.LastStatusCode); err != nil {
			return nil, err
		}
		activity[value.APIKeyID] = value
	}
	return activity, rows.Err()
}

func (s *Store) DeleteAuditBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.ExecContext(ctx, `DELETE FROM audit_logs WHERE occurred_at < ?`, before.UTC().Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

var _ = sql.ErrNoRows
