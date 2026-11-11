// Package usagecontent keeps bounded, short-lived client input/output details
// independently of billing and usage statistics. It never captures headers.
package usagecontent

import (
	"context"
	"errors"
	"time"
)

const (
	Retention           = 24 * time.Hour
	DefaultMaxBodyBytes = 2 << 20
	HardMaxBodyBytes    = 8 << 20
	BufferLimit         = 64 << 20
	QueueSize           = 256
	MaxSessions         = 512
	chunkSize           = 16 << 10
	batchSize           = 16
	batchBytes          = 8 << 20
	cleanupBatchSize    = 1000
	DefaultStorageMiB   = 1024
)

var (
	ErrNotFound     = errors.New("request content not found or expired")
	ErrBusy         = errors.New("request content reader is busy")
	ErrStorageLimit = errors.New("request content storage budget reached")
)

type Config struct {
	Enabled      bool
	MaxBodyBytes int
}

type Metadata struct {
	Method              string `json:"method"`
	Path                string `json:"path"`
	Protocol            string `json:"protocol"`
	Status              int    `json:"status"`
	Partial             bool   `json:"partial"`
	RequestContentType  string `json:"request_content_type"`
	ResponseContentType string `json:"response_content_type"`
	RequestBytes        int64  `json:"request_bytes"`
	ResponseBytes       int64  `json:"response_bytes"`
	RequestStoredBytes  int    `json:"request_stored_bytes"`
	ResponseStoredBytes int    `json:"response_stored_bytes"`
	RequestTruncated    bool   `json:"request_truncated"`
	ResponseTruncated   bool   `json:"response_truncated"`
	RequestOmitted      string `json:"request_omitted,omitempty"`
	ResponseOmitted     string `json:"response_omitted,omitempty"`
}

type Reference struct {
	CaptureID       string    `json:"capture_id"`
	RequestID       string    `json:"request_id"`
	ClientRequestID string    `json:"client_request_id"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	Metadata
}

type Detail struct {
	Reference
	RequestBody  string `json:"request_body"`
	ResponseBody string `json:"response_body"`
}

// Record is an internal persistence value. Bodies are gzip-compressed, not JSONB.
type Record struct {
	Reference
	APIKeyID     int64
	RequestGZIP  []byte
	ResponseGZIP []byte
}

type Query struct {
	APIKeyID        int64
	RequestID       string
	ClientRequestID string
	CaptureID       string
}

type Store interface {
	SaveBatch(context.Context, []Record) error
	References(context.Context, Query) ([]Reference, error)
	Content(context.Context, Query) (*Record, error)
	DeleteExpired(context.Context, int) (int64, error)
}

type Stats struct {
	Active          int64  `json:"active"`
	BufferedBytes   int64  `json:"buffered_bytes"`
	Queued          int    `json:"queued"`
	Saved           uint64 `json:"saved"`
	Dropped         uint64 `json:"dropped"`
	Truncated       uint64 `json:"truncated"`
	WriteFailures   uint64 `json:"write_failures"`
	CleanupFailures uint64 `json:"cleanup_failures"`
	Deleted         uint64 `json:"deleted"`
	StorageFull     bool   `json:"storage_full"`
}

type contextKey struct{}
type connectionKey struct{}

func WithSession(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, contextKey{}, session)
}

func WithConnection(ctx context.Context, connection *Connection) context.Context {
	return context.WithValue(ctx, connectionKey{}, connection)
}

// A billing task must copy the current session, not the mutable WS connection.
func FromContext(ctx context.Context) *Session {
	if ctx == nil {
		return nil
	}
	if s, ok := ctx.Value(contextKey{}).(*Session); ok {
		return s
	}
	if c := ConnectionFromContext(ctx); c != nil {
		return c.Current()
	}
	return nil
}

func ConnectionFromContext(ctx context.Context) *Connection {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(connectionKey{}).(*Connection)
	return c
}
