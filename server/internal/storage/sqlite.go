package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/relayhub/relayhub/server/internal/models"
)

type SQLiteStore struct {
	db *sql.DB
}

func Open(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) init() error {
	schema := `
CREATE TABLE IF NOT EXISTS requests (
  request_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  api_key_id TEXT NOT NULL,
  session_key TEXT NOT NULL,
  logical_model TEXT NOT NULL,
  entry_protocol TEXT NOT NULL,
  route_strategy TEXT NOT NULL,
  matched_rule_ids TEXT NOT NULL,
  final_provider_id TEXT NOT NULL,
  final_model TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT NOT NULL,
  started_at TEXT NOT NULL,
  completed_at TEXT NOT NULL,
  logical_usage_json TEXT NOT NULL,
  physical_cost REAL NOT NULL,
  normalized_request TEXT NOT NULL,
  route_decision TEXT NOT NULL,
  response_preview TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS request_attempts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  request_id TEXT NOT NULL,
  attempt_index INTEGER NOT NULL,
  provider_id TEXT NOT NULL,
  provider_model TEXT NOT NULL,
  status TEXT NOT NULL,
  is_winner INTEGER NOT NULL,
  launch_mode TEXT NOT NULL,
  started_at TEXT NOT NULL,
  completed_at TEXT NOT NULL,
  cancelled_at TEXT NOT NULL,
  latency_ms INTEGER NOT NULL,
  usage_json TEXT NOT NULL,
  error TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS session_bindings (
  session_key TEXT NOT NULL,
  project_id TEXT NOT NULL,
  provider_id TEXT NOT NULL,
  provider_model TEXT NOT NULL,
  bound_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  PRIMARY KEY (session_key, project_id)
);`

	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) SaveRequest(ctx context.Context, record models.RequestRecord, attempts []models.AttemptRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	rollback := func(cause error) error {
		_ = tx.Rollback()
		return cause
	}

	matchedRules, err := json.Marshal(record.MatchedRuleIDs)
	if err != nil {
		return rollback(err)
	}
	logicalUsage, err := json.Marshal(record.LogicalUsage)
	if err != nil {
		return rollback(err)
	}
	norm, err := json.Marshal(record.NormalizedReq)
	if err != nil {
		return rollback(err)
	}
	decision, err := json.Marshal(record.RouteDecision)
	if err != nil {
		return rollback(err)
	}

	_, err = tx.ExecContext(ctx, `
INSERT OR REPLACE INTO requests (
  request_id, project_id, api_key_id, session_key, logical_model, entry_protocol, route_strategy,
  matched_rule_ids, final_provider_id, final_model, status, error, started_at, completed_at,
  logical_usage_json, physical_cost, normalized_request, route_decision, response_preview
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.RequestID, record.ProjectID, record.APIKeyID, record.SessionKey, record.LogicalModel,
		record.EntryProtocol, record.RouteStrategy, string(matchedRules), record.FinalProviderID,
		record.FinalModel, record.Status, record.Error, record.StartedAt.Format(time.RFC3339Nano),
		record.CompletedAt.Format(time.RFC3339Nano), string(logicalUsage), record.PhysicalCost,
		string(norm), string(decision), record.ResponsePreview,
	)
	if err != nil {
		return rollback(err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM request_attempts WHERE request_id = ?`, record.RequestID); err != nil {
		return rollback(err)
	}
	for _, attempt := range attempts {
		usageJSON, marshalErr := json.Marshal(attempt.Usage)
		if marshalErr != nil {
			return rollback(marshalErr)
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO request_attempts (
  request_id, attempt_index, provider_id, provider_model, status, is_winner, launch_mode,
  started_at, completed_at, cancelled_at, latency_ms, usage_json, error
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			attempt.RequestID, attempt.AttemptIndex, attempt.ProviderID, attempt.ProviderModel, attempt.Status,
			boolToInt(attempt.IsWinner), attempt.LaunchMode, attempt.StartedAt.Format(time.RFC3339Nano),
			attempt.CompletedAt.Format(time.RFC3339Nano), attempt.CancelledAt.Format(time.RFC3339Nano),
			attempt.LatencyMS, string(usageJSON), attempt.Error,
		)
		if err != nil {
			return rollback(err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) UpsertSessionBinding(ctx context.Context, binding models.SessionBinding) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO session_bindings (session_key, project_id, provider_id, provider_model, bound_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(session_key, project_id) DO UPDATE SET
  provider_id = excluded.provider_id,
  provider_model = excluded.provider_model,
  last_seen_at = excluded.last_seen_at`,
		binding.SessionKey, binding.ProjectID, binding.ProviderID, binding.ProviderModel,
		binding.BoundAt.Format(time.RFC3339Nano), binding.LastSeenAt.Format(time.RFC3339Nano),
	)
	return err
}

func (s *SQLiteStore) TouchSession(ctx context.Context, projectID, sessionKey string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE session_bindings SET last_seen_at = ? WHERE session_key = ? AND project_id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), sessionKey, projectID,
	)
	return err
}

func (s *SQLiteStore) GetSessionBinding(ctx context.Context, projectID, sessionKey string) (models.SessionBinding, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT session_key, project_id, provider_id, provider_model, bound_at, last_seen_at
FROM session_bindings WHERE session_key = ? AND project_id = ?`,
		sessionKey, projectID,
	)
	var binding models.SessionBinding
	var boundAt string
	var lastSeenAt string
	if err := row.Scan(&binding.SessionKey, &binding.ProjectID, &binding.ProviderID, &binding.ProviderModel, &boundAt, &lastSeenAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SessionBinding{}, false, nil
		}
		return models.SessionBinding{}, false, err
	}
	binding.BoundAt, _ = time.Parse(time.RFC3339Nano, boundAt)
	binding.LastSeenAt, _ = time.Parse(time.RFC3339Nano, lastSeenAt)
	return binding, true, nil
}

func (s *SQLiteStore) ListSessions(ctx context.Context) ([]models.SessionBinding, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT session_key, project_id, provider_id, provider_model, bound_at, last_seen_at
FROM session_bindings ORDER BY last_seen_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.SessionBinding{}
	for rows.Next() {
		var binding models.SessionBinding
		var boundAt string
		var lastSeenAt string
		if err := rows.Scan(&binding.SessionKey, &binding.ProjectID, &binding.ProviderID, &binding.ProviderModel, &boundAt, &lastSeenAt); err != nil {
			return nil, err
		}
		binding.BoundAt, _ = time.Parse(time.RFC3339Nano, boundAt)
		binding.LastSeenAt, _ = time.Parse(time.RFC3339Nano, lastSeenAt)
		out = append(out, binding)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ListRequests(ctx context.Context, limit int) ([]models.RequestRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT request_id, project_id, api_key_id, session_key, logical_model, entry_protocol, route_strategy,
       matched_rule_ids, final_provider_id, final_model, status, error, started_at, completed_at,
       logical_usage_json, physical_cost, normalized_request, route_decision, response_preview
FROM requests ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.RequestRecord{}
	for rows.Next() {
		item, scanErr := scanRequest(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) GetRequest(ctx context.Context, requestID string) (models.RequestRecord, []models.AttemptRecord, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT request_id, project_id, api_key_id, session_key, logical_model, entry_protocol, route_strategy,
       matched_rule_ids, final_provider_id, final_model, status, error, started_at, completed_at,
       logical_usage_json, physical_cost, normalized_request, route_decision, response_preview
FROM requests WHERE request_id = ?`, requestID)
	record, err := scanRequest(row)
	if err != nil {
		return models.RequestRecord{}, nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT request_id, attempt_index, provider_id, provider_model, status, is_winner, launch_mode,
       started_at, completed_at, cancelled_at, latency_ms, usage_json, error
FROM request_attempts WHERE request_id = ? ORDER BY attempt_index ASC`, requestID)
	if err != nil {
		return models.RequestRecord{}, nil, err
	}
	defer rows.Close()

	attempts := []models.AttemptRecord{}
	for rows.Next() {
		var item models.AttemptRecord
		var startedAt string
		var completedAt string
		var cancelledAt string
		var usageJSON string
		var isWinner int
		if err := rows.Scan(&item.RequestID, &item.AttemptIndex, &item.ProviderID, &item.ProviderModel, &item.Status, &isWinner,
			&item.LaunchMode, &startedAt, &completedAt, &cancelledAt, &item.LatencyMS, &usageJSON, &item.Error); err != nil {
			return models.RequestRecord{}, nil, err
		}
		item.IsWinner = isWinner == 1
		item.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
		item.CompletedAt, _ = time.Parse(time.RFC3339Nano, completedAt)
		item.CancelledAt, _ = time.Parse(time.RFC3339Nano, cancelledAt)
		_ = json.Unmarshal([]byte(usageJSON), &item.Usage)
		attempts = append(attempts, item)
	}

	return record, attempts, rows.Err()
}

func (s *SQLiteStore) UsageSummary(ctx context.Context) (models.UsageSummary, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
  COUNT(*) AS requests,
  SUM(CASE WHEN status = 'succeeded' THEN 1 ELSE 0 END) AS successes,
  SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failures,
  COALESCE(SUM(json_extract(logical_usage_json, '$.input_tokens')), 0) AS input_tokens,
  COALESCE(SUM(json_extract(logical_usage_json, '$.output_tokens')), 0) AS output_tokens,
  COALESCE(SUM(json_extract(logical_usage_json, '$.cost')), 0) AS logical_cost,
  COALESCE(SUM(physical_cost), 0) AS physical_cost
FROM requests`)
	var summary models.UsageSummary
	if err := row.Scan(&summary.Requests, &summary.Successes, &summary.Failures, &summary.InputTokens,
		&summary.OutputTokens, &summary.LogicalCost, &summary.PhysicalCost); err != nil {
		return models.UsageSummary{}, err
	}

	sessionsRow := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_bindings`)
	if err := sessionsRow.Scan(&summary.ActiveSessions); err != nil {
		return models.UsageSummary{}, err
	}
	summary.RaceExtraCost = summary.PhysicalCost - summary.LogicalCost
	if summary.RaceExtraCost < 0 {
		summary.RaceExtraCost = 0
	}
	return summary, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRequest(row scanner) (models.RequestRecord, error) {
	var item models.RequestRecord
	var matchedRules string
	var startedAt string
	var completedAt string
	var logicalUsage string
	var norm string
	var decision string
	if err := row.Scan(&item.RequestID, &item.ProjectID, &item.APIKeyID, &item.SessionKey, &item.LogicalModel,
		&item.EntryProtocol, &item.RouteStrategy, &matchedRules, &item.FinalProviderID, &item.FinalModel, &item.Status,
		&item.Error, &startedAt, &completedAt, &logicalUsage, &item.PhysicalCost, &norm, &decision, &item.ResponsePreview); err != nil {
		return models.RequestRecord{}, err
	}
	item.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
	item.CompletedAt, _ = time.Parse(time.RFC3339Nano, completedAt)
	_ = json.Unmarshal([]byte(matchedRules), &item.MatchedRuleIDs)
	_ = json.Unmarshal([]byte(logicalUsage), &item.LogicalUsage)
	_ = json.Unmarshal([]byte(norm), &item.NormalizedReq)
	_ = json.Unmarshal([]byte(decision), &item.RouteDecision)
	return item, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
