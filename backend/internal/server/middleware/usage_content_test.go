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

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/usagecontent"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageContentMiddlewarePreservesOriginalInputAndExactStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	input := []byte(`{"model":"original","input":"question"}`)
	output := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\ndata: [DONE]\n\n"
	for _, kind := range []string{"plain", "gzip", "preread"} {
		t.Run(kind, func(t *testing.T) {
			m := usagecontent.New(usagecontent.Config{Enabled: true, MaxBodyBytes: 1024})
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
			_, record, release, readErr := m.Read(context.Background(), usagecontent.Query{APIKeyID: 11, RequestID: "billed-response"})
			require.NoError(t, readErr)
			require.NotNil(t, record)
			defer release()
			require.Equal(t, string(input), record.RequestBody)
			require.Equal(t, output, record.ResponseBody)
			require.Equal(t, "billed-response", record.RequestID)
			require.Equal(t, w.Header().Get("X-Client-Request-ID"), record.ClientRequestID)
			raw, err := json.Marshal(record.Reference)
			require.NoError(t, err)
			for _, secret := range []string{"QUERY-SECRET", "AUTH-SECRET", "COOKIE-SECRET"} {
				require.NotContains(t, string(raw), secret)
			}
		})
	}
}

func TestUsageContentMiddlewareTruncatesOnlyCaptureAndSupportsDirectJSONBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := usagecontent.New(usagecontent.Config{Enabled: true, MaxBodyBytes: 16})
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
	_, record, release, readErr := m.Read(context.Background(), usagecontent.Query{APIKeyID: 5, ClientRequestID: w.Header().Get("X-Client-Request-ID")})
	require.NoError(t, readErr)
	require.NotNil(t, record)
	defer release()
	require.True(t, record.RequestTruncated)
	require.True(t, record.ResponseTruncated)
	require.Equal(t, input[:16], record.RequestBody)
	require.EqualValues(t, len(input), record.RequestBytes)
}

type shortUsageContentWriter struct {
	gin.ResponseWriter
	called bool
}

func (w *shortUsageContentWriter) WriteString(string) (int, error) {
	w.called = true
	return 3, io.ErrClosedPipe
}

func TestUsageContentWriterPreservesStringShortWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := usagecontent.New(usagecontent.Config{Enabled: true})
	defer m.Close(context.Background())
	s := m.Begin(1, "short", "POST", "/responses", "http", "application/json")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	underlying := &shortUsageContentWriter{ResponseWriter: c.Writer}
	w := &usageContentWriter{ResponseWriter: underlying, session: s}
	n, err := w.WriteString("abc-not-delivered")
	require.Equal(t, 3, n)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.True(t, underlying.called)
	s.Finish(200, w.failed)
	_, d, release, err := m.Read(context.Background(), usagecontent.Query{APIKeyID: 1, ClientRequestID: "short"})
	defer release()
	require.NoError(t, err)
	require.Equal(t, "abc", d.ResponseBody)
	require.EqualValues(t, 3, d.ResponseBytes)
	require.True(t, d.Partial)
}

func TestUsageContentMiddlewareDoesNotRecordUnauthenticatedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := usagecontent.New(usagecontent.Config{Enabled: true})
	r := gin.New()
	r.Use(UsageContent(m))
	r.POST("/v1/responses", func(c *gin.Context) { c.String(401, "unauthorized") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"input":"private"}`)))
	require.NoError(t, m.Close(context.Background()))
	require.Zero(t, m.Stats().Saved)
	require.Zero(t, m.Stats().BufferedBytes)
}
