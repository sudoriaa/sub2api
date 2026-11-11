package middleware

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/usagecontent"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageContentMiddlewareStore struct{ records chan usagecontent.Record }

func (s *usageContentMiddlewareStore) SaveBatch(_ context.Context, records []usagecontent.Record) error {
	for _, r := range records {
		s.records <- r
	}
	return nil
}
func (s *usageContentMiddlewareStore) References(context.Context, usagecontent.Query) ([]usagecontent.Reference, error) {
	return nil, nil
}
func (s *usageContentMiddlewareStore) Content(context.Context, usagecontent.Query) (*usagecontent.Record, error) {
	return nil, usagecontent.ErrNotFound
}
func (s *usageContentMiddlewareStore) DeleteExpired(context.Context, int) (int64, error) {
	return 0, nil
}

func decodeUsageBody(t *testing.T, raw []byte) string {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(raw))
	require.NoError(t, err)
	defer func() { _ = r.Close() }()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}

func TestUsageContentMiddlewarePreservesOriginalInputAndExactStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	input := []byte(`{"model":"original","input":"question"}`)
	output := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\ndata: [DONE]\n\n"
	for _, kind := range []string{"plain", "gzip", "preread"} {
		t.Run(kind, func(t *testing.T) {
			store := &usageContentMiddlewareStore{records: make(chan usagecontent.Record, 1)}
			m := usagecontent.New(store, usagecontent.Config{Enabled: true, MaxBodyBytes: 1024})
			defer func() { require.NoError(t, m.Close(context.Background())) }()
			r := gin.New()
			r.Use(ClientRequestID(), func(c *gin.Context) {
				c.Set(string(ContextKeyAPIKey), &service.APIKey{ID: 11})
				if kind == "preread" {
					c.Request.Body = pkghttputil.NewPrereadBody(input)
				}
				c.Next()
			}, UsageContent(m))
			r.POST("/v1/responses", func(c *gin.Context) {
				body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
				require.NoError(t, err)
				require.Equal(t, input, body)
				// A later gateway rewrite must never overwrite the original capture.
				c.Request.Body = pkghttputil.NewPrereadBody([]byte(`{"model":"mapped"}`))
				_, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
				require.NoError(t, err)
				c.Header("Content-Type", "text/event-stream")
				_, err = c.Writer.WriteString(output[:20])
				require.NoError(t, err)
				c.Writer.Flush()
				_, err = c.Writer.Write([]byte(output[20:]))
				require.NoError(t, err)
				usagecontent.FromContext(c.Request.Context()).Bind("billed-response", 11)
			})
			body := input
			if kind == "gzip" {
				var b bytes.Buffer
				w := gzip.NewWriter(&b)
				_, err := w.Write(body)
				require.NoError(t, err)
				require.NoError(t, w.Close())
				body = b.Bytes()
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/responses?api_key=QUERY-SECRET", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer AUTH-SECRET")
			req.Header.Set("Cookie", "COOKIE-SECRET")
			if kind == "gzip" {
				req.Header.Set("Content-Encoding", "gzip")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
			require.True(t, w.Flushed)
			require.Equal(t, output, w.Body.String())
			select {
			case record := <-store.records:
				require.Equal(t, string(input), decodeUsageBody(t, record.RequestGZIP))
				require.Equal(t, output, decodeUsageBody(t, record.ResponseGZIP))
				require.Equal(t, "billed-response", record.RequestID)
				require.Equal(t, w.Header().Get("X-Client-Request-ID"), record.ClientRequestID)
				raw, err := json.Marshal(record.Reference)
				require.NoError(t, err)
				for _, secret := range []string{"QUERY-SECRET", "AUTH-SECRET", "COOKIE-SECRET"} {
					require.NotContains(t, string(raw), secret)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("missing capture")
			}
		})
	}
}

func TestUsageContentMiddlewareTruncatesOnlyCaptureAndSupportsDirectJSONBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &usageContentMiddlewareStore{records: make(chan usagecontent.Record, 1)}
	m := usagecontent.New(store, usagecontent.Config{Enabled: true, MaxBodyBytes: 16})
	defer func() { require.NoError(t, m.Close(context.Background())) }()
	r := gin.New()
	r.Use(ClientRequestID(), func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), &service.APIKey{ID: 5}); c.Next() }, UsageContent(m))
	input := `{"input":"` + strings.Repeat("x", 100) + `"}`
	r.POST("/v1/messages", func(c *gin.Context) {
		var body map[string]string
		require.NoError(t, c.ShouldBindJSON(&body))
		c.String(200, body["input"])
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(input))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, strings.Repeat("x", 100), w.Body.String())
	select {
	case record := <-store.records:
		require.True(t, record.RequestTruncated)
		require.True(t, record.ResponseTruncated)
		require.Equal(t, input[:16], decodeUsageBody(t, record.RequestGZIP))
		require.EqualValues(t, len(input), record.RequestBytes)
	case <-time.After(5 * time.Second):
		t.Fatal("missing capture")
	}
}

func TestUsageContentMiddlewareDoesNotRecordUnauthenticatedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &usageContentMiddlewareStore{records: make(chan usagecontent.Record, 1)}
	m := usagecontent.New(store, usagecontent.Config{Enabled: true})
	r := gin.New()
	r.Use(UsageContent(m))
	r.POST("/v1/responses", func(c *gin.Context) { c.String(401, "unauthorized") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"input":"private"}`)))
	require.NoError(t, m.Close(context.Background()))
	require.Zero(t, m.Stats().Saved)
	require.Zero(t, m.Stats().BufferedBytes)
}
