package usagecontent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T, maxBytes int) *Manager {
	t.Helper()
	m := New(Config{Enabled: true, MaxBodyBytes: maxBytes})
	t.Cleanup(func() { require.NoError(t, m.Close(context.Background())) })
	return m
}
func readContent(m *Manager, ctx context.Context, q Query) ([]Reference, *Detail, error) {
	refs, detail, release, err := m.Read(ctx, q)
	defer release()
	return refs, detail, err
}

func waitSaved(t *testing.T, m *Manager, n uint64) {
	t.Helper()
	require.Eventually(t, func() bool { return m.Stats().Saved >= n }, 5*time.Second, time.Millisecond)
}

func TestInputOutputLifecycleAndBindingAfterHTTPFinished(t *testing.T) {
	m := newTestManager(t, 1024)
	s := m.Begin(7, "server-client-id", "POST", "/v1/responses", "http", "application/json")
	input := []byte(`{"input":"original question","instructions":"system"}`)
	s.Input(input, "application/json")
	copy(input, []byte(strings.Repeat("X", len(input))))
	s.Input([]byte(`{"input":"rewritten"}`), "application/json")
	s.Output([]byte("data: {\"delta\":\"hello\"}\n\n"), "text/event-stream")
	s.Output([]byte("data: [DONE]\n\n"), "text/event-stream")
	release := s.Acquire()
	s.Finish(200, false)
	require.Zero(t, m.Stats().Saved, "must wait for the queued billing identity")
	s.Bind("billing-id", 999) // An unrelated key cannot bind this user's content.
	s.Bind("billing-id", 7)
	s.Bind("billing-id", 7)
	release()
	release()
	waitSaved(t, m, 1)
	refs, detail, err := readContent(m, context.Background(), Query{APIKeyID: 7, RequestID: "billing-id"})
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, `{"input":"original question","instructions":"system"}`, detail.RequestBody)
	require.Equal(t, "data: {\"delta\":\"hello\"}\n\ndata: [DONE]\n\n", detail.ResponseBody)
	require.Equal(t, Retention, detail.ExpiresAt.Sub(detail.CreatedAt))
	require.Zero(t, m.Stats().BufferedBytes)
	require.Zero(t, m.Stats().Active)
	refs, detail, err = readContent(m, context.Background(), Query{APIKeyID: 999, RequestID: "billing-id"})
	require.NoError(t, err)
	require.Empty(t, refs)
	require.Nil(t, detail)
}

func TestUnbilledErrorsAndAbandonedBillingTaskKeepObservedContent(t *testing.T) {
	m := newTestManager(t, 1024)
	s := m.Begin(9, "trusted-error-id", "POST", "/v1/messages", "http", "application/json")
	s.Input([]byte(`{"messages":[{"role":"user","content":"question"}]}`), "application/json")
	s.Output([]byte(`{"error":{"message":"upstream unavailable"}}`), "application/json")
	abandon := s.Acquire()
	s.Finish(502, true)
	abandon()
	waitSaved(t, m, 1)
	_, detail, err := readContent(m, context.Background(), Query{APIKeyID: 9, ClientRequestID: "trusted-error-id"})
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.Equal(t, 502, detail.Status)
	require.True(t, detail.Partial)
	require.Contains(t, detail.ResponseBody, "upstream unavailable")
}

func TestBodyLimitsAndBinaryOmissionDoNotAffectOriginalBytes(t *testing.T) {
	m := newTestManager(t, 32)
	s := m.Begin(1, "small", "POST", "/v1/chat/completions", "http", "application/json")
	s.Input([]byte(strings.Repeat("q", 100)), "application/json")
	s.Output([]byte(strings.Repeat("a", 90)), "text/event-stream")
	s.Finish(200, false)
	waitSaved(t, m, 1)
	_, d, err := readContent(m, context.Background(), Query{APIKeyID: 1, ClientRequestID: "small"})
	require.NoError(t, err)
	require.Len(t, d.RequestBody, 32)
	require.Len(t, d.ResponseBody, 32)
	require.EqualValues(t, 100, d.RequestBytes)
	require.EqualValues(t, 90, d.ResponseBytes)
	require.True(t, d.RequestTruncated)
	require.True(t, d.ResponseTruncated)
	require.EqualValues(t, 1, m.Stats().Truncated)
	s = m.Begin(1, "binary", "POST", "/v1/images/edits", "http", "multipart/form-data; boundary=test")
	s.Input([]byte("image-binary"), "multipart/form-data")
	s.Output([]byte("audio-binary"), "audio/mpeg")
	s.Finish(200, false)
	waitSaved(t, m, 2)
	_, d, err = readContent(m, context.Background(), Query{APIKeyID: 1, ClientRequestID: "binary"})
	require.NoError(t, err)
	require.Empty(t, d.RequestBody)
	require.Empty(t, d.ResponseBody)
	require.Equal(t, "binary_content", d.RequestOmitted)
	require.Equal(t, "binary_content", d.ResponseOmitted)
}

func TestProcessBufferCapacityFailsOpenWithoutLeaks(t *testing.T) {
	m := newTestManager(t, 1024)
	s := m.Begin(1, "overload", "POST", "/v1/responses", "http", "application/json")
	m.buffered.Store(BufferLimit)
	require.Nil(t, m.Begin(1, "skipped", "POST", "/v1/responses", "http", "application/json"))
	s.Input([]byte("still forwarded by caller"), "application/json")
	s.Finish(200, false)
	_, d, err := readContent(m, context.Background(), Query{APIKeyID: 1, ClientRequestID: "overload"})
	require.NoError(t, err)
	require.Equal(t, "buffer_capacity", d.RequestOmitted)
	require.True(t, d.RequestTruncated)
	require.Empty(t, d.RequestBody)
	require.EqualValues(t, BufferLimit, m.Stats().BufferedBytes)
	m.buffered.Store(0)
}

func TestConcurrentRequestsRemainSeparateAndReleaseBuffers(t *testing.T) {
	m := newTestManager(t, 1024)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := m.Begin(int64(i+1), fmt.Sprint(i), "POST", "/v1/responses", "http", "application/json")
			s.Input([]byte(fmt.Sprint(i)), "application/json")
			done := s.Acquire()
			s.Finish(200, false)
			s.Bind(fmt.Sprint(i), int64(i+1))
			done()
		}(i)
	}
	wg.Wait()
	waitSaved(t, m, 100)
	require.Zero(t, m.Stats().BufferedBytes)
	require.Zero(t, m.Stats().Dropped)
	for i := 0; i < 100; i++ {
		_, d, err := readContent(m, context.Background(), Query{APIKeyID: int64(i + 1), RequestID: fmt.Sprint(i)})
		require.NoError(t, err)
		require.Equal(t, fmt.Sprint(i), d.RequestBody)
	}
}

func TestWebSocketTurnsKeepOriginalFirstFrameAndDistinctBillingSessions(t *testing.T) {
	m := newTestManager(t, 1024)
	c := NewConnection(m, 3, "ws-client", "/v1/responses")
	c.InitialInput([]byte(`{"type":"response.create","input":"original first"}`))
	s1 := c.BeginTurn(1, []byte(`{"input":"mapped first"}`))
	s1.OutputFrame([]byte(`{"type":"response.output_text.delta","delta":"first"}`))
	s1.Bind("first-response", 3)
	finishBilling := s1.Acquire()
	s1.Finish(101, false)
	s2 := c.BeginTurn(2, []byte(`{"type":"response.create","input":"second"}`))
	require.NotSame(t, s1, s2)
	parent := WithSession(context.Background(), s1)
	require.Same(t, s1, FromContext(parent), "later turns must not retarget queued billing tasks")
	s2.OutputFrame([]byte(`{"type":"response.output_text.delta","delta":"second"}`))
	s2.Bind("second-response", 3)
	s2.Finish(101, false)
	finishBilling()
	c.Close()
	waitSaved(t, m, 2)
	_, d, err := readContent(m, context.Background(), Query{APIKeyID: 3, RequestID: "first-response"})
	require.NoError(t, err)
	require.Contains(t, d.RequestBody, "original first")
	require.NotContains(t, d.ResponseBody, "second")
	require.False(t, d.Partial)
	_, d, err = readContent(m, context.Background(), Query{APIKeyID: 3, RequestID: "second-response"})
	require.NoError(t, err)
	require.Contains(t, d.RequestBody, "second")
	require.NotContains(t, d.ResponseBody, "first")
}

func TestWebSocketNextFrameBeforePreviousBillingDoesNotFinalizeWrongTurn(t *testing.T) {
	m := newTestManager(t, 1024)
	c := NewConnection(m, 5, "rapid-ws", "/v1/responses")
	s1 := c.BeginTurn(1, []byte(`{"input":"first"}`))
	// Snapshot before downstream Write; a fast client can start its next turn
	// immediately after that write returns, before the completion callback.
	delivered := c.Current()
	s2 := c.BeginTurn(2, []byte(`{"input":"second"}`))
	delivered.OutputFrame([]byte(`{"type":"response.completed","response":{"id":"first-response"}}`))
	require.False(t, s1.isFinished())
	require.Zero(t, m.Stats().Saved)
	s, done := c.TakeTurn(1)
	require.Same(t, s1, s)
	s.Bind("first-response", 5)
	s.Finish(101, false)
	done()
	s, done = c.TakeTurn(2)
	require.Same(t, s2, s)
	s.OutputFrame([]byte(`{"type":"response.completed","response":{"id":"second-response"}}`))
	s.Bind("second-response", 5)
	s.Finish(101, false)
	done()
	c.Close()
	waitSaved(t, m, 2)
	_, first, err := readContent(m, context.Background(), Query{APIKeyID: 5, RequestID: "first-response"})
	require.NoError(t, err)
	require.False(t, first.Partial)
	require.Contains(t, first.ResponseBody, "first-response")
	require.NotContains(t, first.ResponseBody, "second-response")
	_, second, err := readContent(m, context.Background(), Query{APIKeyID: 5, RequestID: "second-response"})
	require.NoError(t, err)
	require.False(t, second.Partial)
	require.NotContains(t, second.ResponseBody, "first-response")
}

func TestMetadataNeverContainsHeaders(t *testing.T) {
	m := newTestManager(t, 1024)
	s := m.Begin(1, "internal-correlation", "POST", "/v1/responses", "http", "application/json")
	s.Input([]byte(`{"input":"hello"}`), "application/json")
	s.Finish(200, false)
	waitSaved(t, m, 1)
	_, d, err := readContent(m, context.Background(), Query{APIKeyID: 1, ClientRequestID: "internal-correlation"})
	require.NoError(t, err)
	raw, err := json.Marshal(d.Metadata)
	require.NoError(t, err)
	require.NotContains(t, strings.ToLower(string(raw)), "authorization")
	require.NotContains(t, strings.ToLower(string(raw)), "cookie")
}

func BenchmarkUsageContentCapture(b *testing.B) {
	// Includes bounded publication and FIFO eviction. No gzip or SQL.
	m := New(Config{Enabled: true, MaxRecords: 1})
	defer m.Close(context.Background())
	input, output := bytes.Repeat([]byte("x"), 16<<10), bytes.Repeat([]byte("y"), 4<<10)
	b.ReportAllocs()
	b.SetBytes(int64(len(input) + len(output)))
	for i := 0; i < b.N; i++ {
		s := m.Begin(1, "benchmark", "POST", "/v1/responses", "http", "application/json")
		s.Input(input, "application/json")
		s.Output(output, "text/event-stream")
		s.Finish(200, false)
	}
}
