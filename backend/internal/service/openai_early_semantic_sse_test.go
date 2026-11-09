package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type earlySemanticClockBody struct {
	origin  time.Time
	index   int
	pending string
	events  []string
}

func (r *earlySemanticClockBody) Read(p []byte) (int, error) {
	if r.pending == "" {
		events := []string{
			`{"type":"response.output_item.added","item":{"type":"reasoning","summary":[]}}`,
			`{"type":"response.output_text.delta","delta":"OK"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":2}}}`,
		}
		if r.events != nil {
			events = r.events
		}
		at := []time.Duration{100 * time.Millisecond, 3100 * time.Millisecond, 3200 * time.Millisecond}
		if r.index == len(events) {
			return 0, io.EOF
		}
		time.Sleep(at[r.index] - time.Since(r.origin))
		r.pending = "data: " + events[r.index] + "\n\n"
		r.index++
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
func (r *earlySemanticClockBody) Close() error { return nil }

type earlySemanticFlushRecorder struct {
	*httptest.ResponseRecorder
	origin time.Time
	flush  []time.Duration
}

func (r *earlySemanticFlushRecorder) Flush() {
	r.flush = append(r.flush, time.Since(r.origin))
	r.ResponseRecorder.Flush()
}

func TestOpenAIEarlySemanticSSEDoesNotWaitForVisibleFrame(t *testing.T) {
	setSemanticTTFTTestMode(t)
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"native", "native_guarded", "passthrough"} {
		for _, enabled := range []bool{false, true} {
			t.Run(path+map[bool]string{false: "/default", true: "/early"}[enabled], func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					origin := time.Now()
					rec := &earlySemanticFlushRecorder{ResponseRecorder: httptest.NewRecorder(), origin: origin}
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
					resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: &earlySemanticClockBody{origin: origin}}
					a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"openai_early_semantic_sse": enabled}}
					svc := &OpenAIGatewayService{cfg: &config.Config{}}
					if path == "native_guarded" {
						svc.cfg.Gateway.OpenAIFirstOutputTimeoutSeconds = 30
					}
					var first *int
					if path == "passthrough" {
						r, err := svc.handleStreamingResponsePassthrough(context.Background(), resp, c, a, origin, "test", "test")
						require.NoError(t, err)
						first = r.firstTokenMs
					} else {
						r, err := svc.handleStreamingResponse(context.Background(), resp, c, a, origin, "test", "test")
						require.NoError(t, err)
						first = r.firstTokenMs
					}
					require.NotNil(t, first)
					require.Equal(t, 100, *first, "semantic TTFT is not shortened or redefined")
					require.NotEmpty(t, rec.flush)
					want := 3100 * time.Millisecond
					if enabled {
						want = 100 * time.Millisecond
					}
					require.Equal(t, want, rec.flush[0])
					require.Equal(t, 3200*time.Millisecond, time.Since(origin), "model completion is unchanged")
					require.Contains(t, rec.Body.String(), `"delta":"OK"`)
				})
			})
		}
	}
}

func TestOpenAIEarlySemanticSSEIsStrictlyOptIn(t *testing.T) {
	require.False(t, (*Account)(nil).IsOpenAIEarlySemanticSSEEnabled())
	for _, tc := range []struct {
		platform, typ string
		flag          any
		want          bool
	}{
		{PlatformOpenAI, AccountTypeAPIKey, true, true},
		{PlatformOpenAI, AccountTypeAPIKey, false, false},
		{PlatformOpenAI, AccountTypeAPIKey, "true", false},
		{PlatformOpenAI, AccountTypeOAuth, true, false},
		{PlatformAnthropic, AccountTypeAPIKey, true, false},
	} {
		a := &Account{Platform: tc.platform, Type: tc.typ, Extra: map[string]any{"openai_early_semantic_sse": tc.flag}}
		require.Equal(t, tc.want, a.IsOpenAIEarlySemanticSSEEnabled())
	}
}

func TestOpenAIEarlySemanticSSEDoesNotReplayCommittedProgress(t *testing.T) {
	setSemanticTTFTTestMode(t)
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"native", "passthrough"} {
		t.Run(path, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				origin := time.Now()
				rec := &earlySemanticFlushRecorder{ResponseRecorder: httptest.NewRecorder(), origin: origin}
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: &earlySemanticClockBody{origin: origin, events: []string{
					`{"type":"response.output_item.added","item":{"type":"reasoning","summary":[]}}`,
					`{"type":"response.failed","response":{"error":{"code":"server_is_overloaded","type":"server_error","message":"server_is_overloaded"},"usage":{"input_tokens":10,"output_tokens":0}}}`,
				}}}
				a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"openai_early_semantic_sse": true}}
				svc := &OpenAIGatewayService{cfg: &config.Config{}}
				var err error
				if path == "passthrough" {
					_, err = svc.handleStreamingResponsePassthrough(context.Background(), resp, c, a, origin, "test", "test")
				} else {
					_, err = svc.handleStreamingResponse(context.Background(), resp, c, a, origin, "test", "test")
				}
				var replay *UpstreamFailoverError
				require.False(t, errors.As(err, &replay), "a committed semantic event must never silently replay")
				require.NotEmpty(t, rec.flush)
				require.Equal(t, 100*time.Millisecond, rec.flush[0])
				require.Equal(t, 1, bytes.Count(rec.Body.Bytes(), []byte(`"type":"response.output_item.added"`)))
				require.Contains(t, rec.Body.String(), `"type":"response.failed"`)
			})
		})
	}
}
