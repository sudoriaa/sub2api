package usagecontent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func windowManager(t *testing.T, config Config) *Manager {
	t.Helper()
	config.Enabled = true
	m := New(config)
	t.Cleanup(func() { require.NoError(t, m.Close(context.Background())) })
	return m
}

func publish(m *Manager, id string, body string) {
	s := m.Begin(1, id, "POST", "/v1/responses", "http", "application/json")
	s.Input([]byte(body), "application/json")
	s.Bind(id, 1)
	s.Finish(200, false)
}

func TestMemoryWindowEvictsOldestAt101(t *testing.T) {
	m := windowManager(t, Config{})
	for i := 1; i <= 101; i++ {
		publish(m, fmt.Sprint(i), fmt.Sprint(i))
	}
	stats := m.Stats()
	require.Equal(t, 100, stats.Records)
	require.EqualValues(t, 101, stats.Saved)
	require.EqualValues(t, 1, stats.Deleted)
	require.Zero(t, stats.Dropped)
	_, d, err := readContent(m, context.Background(), Query{APIKeyID: 1, RequestID: "1"})
	require.NoError(t, err)
	require.Nil(t, d)
	for _, id := range []string{"2", "101"} {
		_, d, err = readContent(m, context.Background(), Query{APIKeyID: 1, RequestID: id})
		require.NoError(t, err)
		require.Equal(t, id, d.RequestBody)
	}
}

func TestMemoryWindowByteBudgetCountsAllocatedCapacity(t *testing.T) {
	m := windowManager(t, Config{MaxRecords: 100, MaxMemoryBytes: 2048, MaxBodyBytes: 1024})
	for _, id := range []string{"first", "second", "third"} {
		publish(m, id, "x")
	}
	require.Equal(t, 2, m.Stats().Records)
	require.EqualValues(t, 2048, m.Stats().RetainedBytes)
	require.Zero(t, m.Stats().BufferedBytes)
	_, d, err := readContent(m, context.Background(), Query{APIKeyID: 1, RequestID: "first"})
	require.NoError(t, err)
	require.Nil(t, d)
	// An individually oversized capture is dropped without destroying history.
	small := windowManager(t, Config{MaxMemoryBytes: 1024, MaxBodyBytes: 1024})
	publish(small, "keep", "keep")
	s := small.Begin(1, "oversized", "POST", "/responses", "http", "application/json")
	s.Input([]byte("x"), "application/json")
	s.Output([]byte("x"), "application/json")
	s.Finish(200, false)
	require.Equal(t, 1, small.Stats().Records)
	require.EqualValues(t, 1, small.Stats().Dropped)
}

func TestMemoryWindowPinnedEvictionAndReadBudget(t *testing.T) {
	m := windowManager(t, Config{MaxRecords: 1, MaxMemoryBytes: 1024, MaxBodyBytes: 1024})
	publish(m, "first", "private first")
	q := Query{APIKeyID: 1, RequestID: "first"}
	_, d, release, err := m.Read(context.Background(), q)
	require.NoError(t, err)
	_, _, release2, err := m.Read(context.Background(), q)
	require.NoError(t, err)
	_, _, release3, err := m.Read(context.Background(), q)
	require.ErrorIs(t, err, ErrBusy)
	release3()
	publish(m, "second", "second")
	require.Zero(t, m.Stats().Records)
	require.EqualValues(t, 1024, m.Stats().RetainedBytes, "evicted reader-pinned chunks remain charged")
	require.EqualValues(t, 1, m.Stats().Dropped)
	require.Equal(t, "private first", d.RequestBody)
	release()
	release()
	require.EqualValues(t, 1024, m.Stats().RetainedBytes)
	release2()
	require.Zero(t, m.Stats().RetainedBytes)
	require.Zero(t, m.Stats().ReadBytes)
	publish(m, "third", "third")
	require.Equal(t, 1, m.Stats().Records)
}

func TestMemoryWindowMultipleBindingsShareOnePayloadAndStayScoped(t *testing.T) {
	m := windowManager(t, Config{})
	s := m.Begin(1, "client", "POST", "/responses", "http", "application/json")
	s.Input([]byte("private"), "application/json")
	for i := 0; i < 8; i++ {
		s.Bind(fmt.Sprint(i), 1)
	}
	s.Finish(200, false)
	require.Equal(t, 1, m.Stats().Records)
	require.EqualValues(t, chunkSize, m.Stats().RetainedBytes)
	for i := 0; i < 8; i++ {
		_, d, err := readContent(m, context.Background(), Query{APIKeyID: 1, RequestID: fmt.Sprint(i)})
		require.NoError(t, err)
		require.Equal(t, "private", d.RequestBody)
		require.Equal(t, fmt.Sprint(i), d.RequestID)
	}
	for _, q := range []Query{{APIKeyID: 2, ClientRequestID: "client"}, {APIKeyID: 2, RequestID: "0"}, {APIKeyID: 1, RequestID: "other", CaptureID: s.ref.CaptureID}} {
		refs, d, err := readContent(m, context.Background(), q)
		require.NoError(t, err)
		require.Empty(t, refs)
		require.Nil(t, d)
	}
}

func TestMemoryWindowExpiryIsIndependentOfFIFOAndRestart(t *testing.T) {
	m := windowManager(t, Config{})
	publish(m, "live", "live")
	publish(m, "expired", "expired")
	m.mu.Lock()
	m.window.Back().Value.(*windowEntry).ref.ExpiresAt = time.Now().Add(-time.Second)
	m.mu.Unlock()
	_, d, err := readContent(m, context.Background(), Query{APIKeyID: 1, RequestID: "expired"})
	require.NoError(t, err)
	require.Nil(t, d, "must reject before periodic cleanup")
	m.cleanup()
	require.Equal(t, 1, m.Stats().Records)
	_, d, err = readContent(m, context.Background(), Query{APIKeyID: 1, RequestID: "live"})
	require.NoError(t, err)
	require.Equal(t, "live", d.RequestBody)
	fresh := windowManager(t, Config{})
	_, d, err = readContent(fresh, context.Background(), Query{APIKeyID: 1, RequestID: "live"})
	require.NoError(t, err)
	require.Nil(t, d)
}

func TestMemoryWindowCloseAndCancellationReleaseResources(t *testing.T) {
	m := windowManager(t, Config{})
	publish(m, "one", "private")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, done, err := m.Read(ctx, Query{APIKeyID: 1, RequestID: "one"})
	require.ErrorIs(t, err, context.Canceled)
	done()
	_, d, done, err := m.Read(context.Background(), Query{APIKeyID: 1, RequestID: "one"})
	require.NoError(t, err)
	require.NoError(t, m.Close(context.Background()))
	require.False(t, m.Enabled())
	require.Equal(t, "private", d.RequestBody)
	require.Greater(t, m.Stats().RetainedBytes, int64(0))
	done()
	require.Zero(t, m.Stats().RetainedBytes)
	require.Zero(t, m.Stats().PooledBytes)
	require.Zero(t, m.Stats().ReadBytes)
}

func TestMemoryWindowConcurrentPublishReadEvict(t *testing.T) {
	m := windowManager(t, Config{MaxRecords: 10, MaxMemoryBytes: 256 << 10})
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				id := fmt.Sprintf("%d-%d", n, i)
				publish(m, id, id)
				_, d, done, err := m.Read(context.Background(), Query{APIKeyID: 1, RequestID: id})
				if err != nil && err != ErrBusy {
					t.Error(err)
				}
				if d != nil && d.RequestBody != id {
					t.Errorf("mixed capture: %q != %q", d.RequestBody, id)
				}
				done()
			}
		}(n)
	}
	wg.Wait()
	require.LessOrEqual(t, m.Stats().Records, 10)
	require.LessOrEqual(t, m.Stats().RetainedBytes, int64(256<<10))
	require.LessOrEqual(t, m.Stats().PooledBytes, int64(poolChunks*chunkSize))
	require.Zero(t, m.Stats().ReadBytes)
	require.Zero(t, m.Stats().BufferedBytes)
}

func TestMemoryWindowReusedChunksNeverExposeOldTail(t *testing.T) {
	m := windowManager(t, Config{MaxRecords: 1})
	publish(m, "old", strings.Repeat("secret", 2000))
	publish(m, "evict", "evict")
	publish(m, "new", "x")
	_, d, err := readContent(m, context.Background(), Query{APIKeyID: 1, RequestID: "new"})
	require.NoError(t, err)
	require.Equal(t, "x", d.RequestBody)
}

func TestMemoryWindowStreamMetadataAndTruncationDoNotAllocatePerChunk(t *testing.T) {
	m := windowManager(t, Config{MaxBodyBytes: 1})
	s := m.Begin(1, "stream", "POST", "/responses", "http", "application/json")
	p := []byte("hello")
	s.Output(p, "text/event-stream")
	require.Zero(t, testing.AllocsPerRun(1000, func() { s.Output(p, "text/event-stream") }))
	require.Zero(t, testing.AllocsPerRun(1000, func() { s.OutputString("hello", "text/event-stream") }))
	s.Finish(200, false)
	_, d, err := readContent(m, context.Background(), Query{APIKeyID: 1, ClientRequestID: "stream"})
	require.NoError(t, err)
	require.Equal(t, "h", d.ResponseBody)
	require.True(t, d.ResponseTruncated)
}

func BenchmarkUsageContentLargeStream(b *testing.B) {
	m := New(Config{Enabled: true, MaxRecords: 1})
	defer m.Close(context.Background())
	input := bytes.Repeat([]byte("x"), 2<<20)
	output := bytes.Repeat([]byte("y"), 512)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := m.Begin(1, "bench", "POST", "/responses", "http", "application/json")
		s.Input(input, "application/json")
		for j := 0; j < (2<<20)/len(output); j++ {
			s.Output(output, "text/event-stream")
		}
		s.Finish(200, false)
	}
}
