package usagecontent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SQLStore struct {
	DB              *sql.DB
	MaxStorageBytes int64
}

// Initialize reconciles the small budget counter only on startup, including
// after restoring a backup that deliberately excludes conversation payloads.
// Normal writes/cleanup use O(1) increments, not repeated SUM/table scans.
func (s *SQLStore) Initialize(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO usage_content_budget(id,used_bytes) VALUES(1,0) ON CONFLICT (id) DO NOTHING`); err != nil {
		return err
	}
	var used int64
	if err := tx.QueryRowContext(ctx, `SELECT used_bytes FROM usage_content_budget WHERE id=1 FOR UPDATE`).Scan(&used); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE usage_content_budget SET used_bytes=(SELECT COALESCE(SUM(storage_bytes),0) FROM usage_request_contents) WHERE id=1`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLStore) SaveBatch(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	const columns = 11
	values, args := make([]string, 0, len(records)), make([]any, 0, columns*len(records))
	var needed int64
	for _, r := range records {
		if !time.Now().Before(r.ExpiresAt) {
			continue
		}
		meta, err := json.Marshal(r.Metadata)
		if err != nil {
			return err
		}
		p := len(args)
		// Include a conservative allowance for row/index overhead, not just gzip.
		storageBytes := len(meta) + len(r.RequestGZIP) + len(r.ResponseGZIP) + 512
		needed += int64(storageBytes)
		values = append(values, fmt.Sprintf("($%d::uuid,$%d::varchar,$%d::bigint,$%d::varchar,$%d::timestamptz,$%d::timestamptz,$%d::jsonb,$%d::bytea,$%d::bytea,$%d::int,$%d::int)", p+1, p+2, p+3, p+4, p+5, p+6, p+7, p+8, p+9, p+10, p+11))
		args = append(args, r.CaptureID, r.RequestID, r.APIKeyID, r.ClientRequestID, r.CreatedAt, r.ExpiresAt, string(meta), r.RequestGZIP, r.ResponseGZIP, 1, storageBytes)
	}
	if len(values) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var used int64
	if err := tx.QueryRowContext(ctx, `SELECT used_bytes FROM usage_content_budget WHERE id=1 FOR UPDATE`).Scan(&used); err != nil {
		return err
	}
	budget := s.MaxStorageBytes
	if budget <= 0 {
		budget = DefaultStorageMiB << 20
	}
	if used+needed > budget {
		return ErrStorageLimit
	}
	rows, err := tx.QueryContext(ctx, `INSERT INTO usage_request_contents
 (capture_id,billing_request_id,api_key_id,client_request_id,created_at,expires_at,metadata,request_body,response_body,format_version,storage_bytes)
 SELECT v.* FROM (VALUES `+strings.Join(values, ",")+`) AS v(capture_id,billing_request_id,api_key_id,client_request_id,created_at,expires_at,metadata,request_body,response_body,format_version,storage_bytes)
 WHERE v.expires_at > NOW()
 ON CONFLICT (capture_id,billing_request_id) DO NOTHING RETURNING storage_bytes`, args...)
	if err != nil {
		return err
	}
	var inserted int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			_ = rows.Close()
			return err
		}
		inserted += n
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE usage_content_budget SET used_bytes=used_bytes+$1 WHERE id=1`, inserted); err != nil {
		return err
	}
	return tx.Commit()
}

// Identities always include the API key ID. A client-supplied correlation
// header alone is never trusted to join two users' conversations.
func contentWhere(q Query) (string, []any) {
	args := []any{q.APIKeyID}
	where := "api_key_id=$1 AND expires_at > NOW()"
	if q.ClientRequestID != "" {
		args = append(args, q.ClientRequestID)
		where += " AND client_request_id=$2"
	} else {
		args = append(args, q.RequestID)
		where += " AND billing_request_id=$2"
	}
	return where, args
}

func (s *SQLStore) References(ctx context.Context, q Query) ([]Reference, error) {
	where, args := contentWhere(q)
	rows, err := s.DB.QueryContext(ctx, `SELECT capture_id,billing_request_id,client_request_id,created_at,expires_at,metadata FROM usage_request_contents WHERE `+where+` ORDER BY created_at DESC,capture_id DESC LIMIT 20`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Reference{}
	seen := map[string]bool{}
	for rows.Next() {
		var r Reference
		var meta []byte
		if err := rows.Scan(&r.CaptureID, &r.RequestID, &r.ClientRequestID, &r.CreatedAt, &r.ExpiresAt, &meta); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(meta, &r.Metadata); err != nil {
			return nil, err
		}
		if seen[r.CaptureID] {
			continue
		}
		seen[r.CaptureID] = true
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLStore) Content(ctx context.Context, q Query) (*Record, error) {
	where, args := contentWhere(q)
	args = append(args, q.CaptureID)
	var r Record
	var meta []byte
	err := s.DB.QueryRowContext(ctx, `SELECT capture_id,billing_request_id,client_request_id,created_at,expires_at,metadata,request_body,response_body FROM usage_request_contents WHERE `+where+` AND capture_id=$3::uuid ORDER BY billing_request_id LIMIT 1`, args...).Scan(
		&r.CaptureID, &r.RequestID, &r.ClientRequestID, &r.CreatedAt, &r.ExpiresAt, &meta, &r.RequestGZIP, &r.ResponseGZIP)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(meta, &r.Metadata); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *SQLStore) DeleteExpired(ctx context.Context, limit int) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var used int64
	if err := tx.QueryRowContext(ctx, `SELECT used_bytes FROM usage_content_budget WHERE id=1 FOR UPDATE`).Scan(&used); err != nil {
		return 0, err
	}
	var n, freed int64
	err = tx.QueryRowContext(ctx, `WITH deleted AS (DELETE FROM usage_request_contents WHERE (capture_id,billing_request_id) IN (
 SELECT capture_id,billing_request_id FROM usage_request_contents
	WHERE expires_at <= NOW() ORDER BY expires_at LIMIT $1 FOR UPDATE SKIP LOCKED) RETURNING storage_bytes)
 SELECT COUNT(*),COALESCE(SUM(storage_bytes),0) FROM deleted`, limit).Scan(&n, &freed)
	if err != nil {
		return 0, err
	}
	if freed > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE usage_content_budget SET used_bytes=GREATEST(0,used_bytes-$1) WHERE id=1`, freed); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}
