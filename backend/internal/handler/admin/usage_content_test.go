package admin

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/usagecontent"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageContentAdminRepo struct {
	service.UsageLogRepository
	log   *service.UsageLog
	reads int
}

func (r *usageContentAdminRepo) GetByID(context.Context, int64) (*service.UsageLog, error) {
	r.reads++
	if r.log == nil {
		return nil, service.ErrUsageLogNotFound
	}
	return r.log, nil
}

type usageContentAdminStore struct {
	record *usagecontent.Record
	query  usagecontent.Query
}

func (s *usageContentAdminStore) SaveBatch(context.Context, []usagecontent.Record) error { return nil }
func (s *usageContentAdminStore) DeleteExpired(context.Context, int) (int64, error)      { return 0, nil }
func (s *usageContentAdminStore) References(_ context.Context, q usagecontent.Query) ([]usagecontent.Reference, error) {
	s.query = q
	if s.record == nil || !time.Now().Before(s.record.ExpiresAt) {
		return []usagecontent.Reference{}, nil
	}
	return []usagecontent.Reference{s.record.Reference}, nil
}
func (s *usageContentAdminStore) Content(_ context.Context, q usagecontent.Query) (*usagecontent.Record, error) {
	s.query = q
	return s.record, nil
}

func TestUsageContentRequiresFullAdminBeforeReadingAnyRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{"user", "observer", "scoped", "missing-role"} {
		t.Run(mode, func(t *testing.T) {
			repo := &usageContentAdminRepo{}
			h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				role := service.RoleAdmin
				if mode == "user" {
					role = service.RoleUser
				}
				if mode != "missing-role" {
					c.Set(string(middleware.ContextKeyUserRole), role)
				}
				if mode == "observer" {
					c.Request = c.Request.WithContext(service.WithObserverScope(c.Request.Context(), []int64{1}))
				}
				if mode == "scoped" {
					c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{KeyID: "limited"}))
				}
				c.Next()
			})
			r.GET("/usage/:id/content", h.Content)
			r.GET("/usage/errors/:id/content", h.ErrorContent)
			for _, path := range []string{"/usage/42/content", "/usage/errors/42/content"} {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				require.Equal(t, http.StatusForbidden, w.Code)
				require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			}
			require.Zero(t, repo.reads)
		})
	}
}

func TestUsageContentAdminLoadsOnlySelectedKeyScopedPayloadWithNoStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	zip := func(body string) []byte {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		_, err := w.Write([]byte(body))
		require.NoError(t, err)
		require.NoError(t, w.Close())
		return b.Bytes()
	}
	raw := &usagecontent.Record{APIKeyID: 8, Reference: usagecontent.Reference{CaptureID: "00000000-0000-0000-0000-000000000001", RequestID: "billing", ClientRequestID: "server-id", CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}, RequestGZIP: zip(`{"input":"question"}`), ResponseGZIP: zip(`{"output_text":"answer"}`)}
	store := &usageContentAdminStore{record: raw}
	m := usagecontent.New(store, usagecontent.Config{Enabled: true})
	defer func() { require.NoError(t, m.Close(context.Background())) }()
	repo := &usageContentAdminRepo{log: &service.UsageLog{ID: 42, APIKeyID: 8, RequestID: "billing", CreatedAt: now}}
	h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
	h.Contents = m
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin); c.Next() })
	r.GET("/usage/:id/content", h.Content)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/usage/42/content?capture_id="+raw.CaptureID, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	require.EqualValues(t, 8, store.query.APIKeyID)
	require.Equal(t, "billing", store.query.RequestID)
	require.Equal(t, raw.CaptureID, store.query.CaptureID)
	var envelope struct {
		Data struct {
			Content        usagecontent.Detail `json:"content"`
			RetentionHours int                 `json:"retention_hours"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, 24, envelope.Data.RetentionHours)
	require.Equal(t, `{"input":"question"}`, envelope.Data.Content.RequestBody)
	require.Equal(t, `{"output_text":"answer"}`, envelope.Data.Content.ResponseBody)
	for _, path := range []string{"/usage/not-a-number/content", "/usage/-1/content", "/usage/42/content?capture_id=invalid"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func TestUsageContentExpiredAndHistoricalRequestsRemainHonest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, age := range []time.Duration{time.Hour, 25 * time.Hour} {
		repo := &usageContentAdminRepo{log: &service.UsageLog{ID: 1, APIKeyID: 1, RequestID: "missing", CreatedAt: time.Now().Add(-age)}}
		h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin); c.Next() })
		r.GET("/usage/:id/content", h.Content)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/usage/1/content", nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), `"content":null`)
		if age > 24*time.Hour {
			require.Contains(t, w.Body.String(), `"empty_reason":"expired"`)
		} else {
			require.Contains(t, w.Body.String(), `"empty_reason":"not_recorded"`)
		}
	}
}
