package admin

import (
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
	m := usagecontent.New(usagecontent.Config{Enabled: true})
	defer func() { require.NoError(t, m.Close(context.Background())) }()
	session := m.Begin(8, "server-id", "POST", "/v1/responses", "http", "application/json")
	session.Input([]byte(`{"input":"question"}`), "application/json")
	session.Output([]byte(`{"output_text":"answer"}`), "application/json")
	session.Bind("billing", 8)
	session.Finish(200, false)
	_, raw, release, err := m.Read(context.Background(), usagecontent.Query{APIKeyID: 8, RequestID: "billing"})
	require.NoError(t, err)
	require.NotNil(t, raw)
	release()
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
	// A valid capture ID must not cross API key scope.
	repo.log.APIKeyID = 9
	other := httptest.NewRecorder()
	r.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/usage/42/content?capture_id="+raw.CaptureID, nil))
	require.Contains(t, other.Body.String(), `"content":null`)
	repo.log.APIKeyID = 8
	require.Zero(t, m.Stats().ReadBytes, "HTTP handler must release after serializing")
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
