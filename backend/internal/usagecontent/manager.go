package usagecontent

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type job struct {
	ref      Reference
	apiKeyID int64
	input    bodyBuffer
	output   bodyBuffer
	bindings []binding
}

func (j *job) release(m *Manager) {
	j.input.release(m)
	j.output.release(m)
}

type Manager struct {
	store           Store
	config          Config
	queue           chan job
	done            chan struct{}
	closed          chan struct{}
	gate            sync.RWMutex
	closeOnce       sync.Once
	workerCancel    context.CancelFunc
	readers         chan struct{}
	stopping        atomic.Bool
	active          atomic.Int64
	buffered        atomic.Int64
	saved           atomic.Uint64
	dropped         atomic.Uint64
	truncated       atomic.Uint64
	writeFailures   atomic.Uint64
	cleanupFailures atomic.Uint64
	deleted         atomic.Uint64
	storageFull     atomic.Bool
}

func New(store Store, config Config) *Manager {
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if config.MaxBodyBytes > HardMaxBodyBytes {
		config.MaxBodyBytes = HardMaxBodyBytes
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{store: store, config: config, queue: make(chan job, QueueSize),
		done: make(chan struct{}), closed: make(chan struct{}), workerCancel: cancel, readers: make(chan struct{}, 2)}
	go m.run(ctx)
	return m
}

func (m *Manager) Enabled() bool { return m != nil && m.config.Enabled && !m.stopping.Load() }

func (m *Manager) reserve(n int64) bool {
	for {
		old := m.buffered.Load()
		if n < 0 || old+n > BufferLimit {
			return false
		}
		if m.buffered.CompareAndSwap(old, old+n) {
			return true
		}
	}
}

func (m *Manager) enqueue(j job) {
	m.gate.RLock()
	defer m.gate.RUnlock()
	if !m.stopping.Load() {
		select {
		case m.queue <- j:
			return
		default:
		}
	}
	j.release(m)
	m.dropped.Add(1)
}

func (m *Manager) Stats() Stats {
	if m == nil {
		return Stats{}
	}
	return Stats{Active: m.active.Load(), BufferedBytes: m.buffered.Load(), Queued: len(m.queue),
		Saved: m.saved.Load(), Dropped: m.dropped.Load(), Truncated: m.truncated.Load(),
		WriteFailures: m.writeFailures.Load(), CleanupFailures: m.cleanupFailures.Load(), Deleted: m.deleted.Load(), StorageFull: m.storageFull.Load()}
}

func (m *Manager) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		m.gate.Lock()
		m.stopping.Store(true)
		close(m.done)
		m.gate.Unlock()
	})
	select {
	case <-m.closed:
		return nil
	case <-ctx.Done():
		m.workerCancel()
		// The worker discards queued bodies after cancellation, releasing buffers.
		return ctx.Err()
	}
}

func (m *Manager) run(ctx context.Context) {
	defer close(m.closed)
	defer m.workerCancel()
	zw, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	if initializer, ok := m.store.(interface{ Initialize(context.Context) error }); ok {
		initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := initializer.Initialize(initCtx); err != nil {
			logger.LegacyPrintf("usage_content", "content storage initialization failed")
		}
		cancel()
	}
	// Cleanup starts even with capture disabled and without any gateway traffic.
	m.cleanup(ctx)
	for {
		select {
		case j := <-m.queue:
			m.write(ctx, zw, j)
		case <-ticker.C:
			m.cleanup(ctx)
		case <-m.done:
			for {
				select {
				case j := <-m.queue:
					m.write(ctx, zw, j)
				default:
					return
				}
			}
		}
	}
}

func compressBody(zw *gzip.Writer, b *bodyBuffer) ([]byte, error) {
	var buf bytes.Buffer
	zw.Reset(&buf)
	for _, p := range b.chunks {
		if _, err := zw.Write(p); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *Manager) write(ctx context.Context, zw *gzip.Writer, first job) {
	var records []Record
	n, size := 0, 0
	j := first
	for {
		n++
		if ctx.Err() == nil && !m.storageFull.Load() && time.Now().Before(j.ref.ExpiresAt) {
			request, requestErr := compressBody(zw, &j.input)
			response, responseErr := compressBody(zw, &j.output)
			if requestErr == nil && responseErr == nil {
				if len(j.bindings) == 0 {
					j.bindings = []binding{{}}
				}
				for _, b := range j.bindings {
					ref := j.ref
					ref.RequestID = b.requestID
					records = append(records, Record{Reference: ref, APIKeyID: j.apiKeyID, RequestGZIP: request, ResponseGZIP: response})
				}
				size += len(request) + len(response)
			} else {
				m.writeFailures.Add(1)
			}
		} else {
			m.dropped.Add(1)
		}
		j.release(m)
		if n >= batchSize || size >= batchBytes {
			break
		}
		select {
		case j = <-m.queue:
		default:
			goto persist
		}
	}
persist:
	if len(records) == 0 {
		return
	}
	for attempt := 0; attempt < 3; attempt++ {
		writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := m.store.SaveBatch(writeCtx, records)
		cancel()
		if err == nil {
			m.saved.Add(uint64(len(records)))
			return
		}
		if errors.Is(err, ErrStorageLimit) {
			m.storageFull.Store(true)
			m.dropped.Add(uint64(len(records)))
			return
		}
		if attempt < 2 && ctx.Err() == nil {
			timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
			}
			timer.Stop()
		}
	}
	m.writeFailures.Add(uint64(len(records)))
	// SQL errors can include values. Never send body data into application logs.
	logger.LegacyPrintf("usage_content", "background content write failed (%d records)", len(records))
}

func (m *Manager) cleanup(ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for i := 0; i < 10; i++ {
		n, err := m.store.DeleteExpired(cleanupCtx, cleanupBatchSize)
		if err != nil {
			m.cleanupFailures.Add(1)
			logger.LegacyPrintf("usage_content", "background content cleanup failed")
			return
		}
		m.deleted.Add(uint64(n))
		// Another instance may already have deleted expired rows. Re-open
		// admission after successful cleanup even when this batch deletes none;
		// the O(1), transactional storage guard still enforces the real budget.
		m.storageFull.Store(false)
		if n < cleanupBatchSize {
			return
		}
	}
}

// Read returns references without bodies plus one selected payload. The
// bounded reader semaphore and decompression cap protect the admin endpoint.
func (m *Manager) Read(ctx context.Context, query Query) ([]Reference, *Detail, error) {
	select {
	case m.readers <- struct{}{}:
		defer func() { <-m.readers }()
	default:
		return nil, nil, ErrBusy
	}
	refs, err := m.store.References(ctx, query)
	if err != nil || len(refs) == 0 {
		return refs, nil, err
	}
	if query.CaptureID == "" {
		query.CaptureID = refs[0].CaptureID
	}
	r, err := m.store.Content(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	if !time.Now().Before(r.ExpiresAt) {
		return nil, nil, ErrNotFound
	}
	request, err := decompressBody(r.RequestGZIP)
	if err != nil {
		return nil, nil, err
	}
	response, err := decompressBody(r.ResponseGZIP)
	if err != nil {
		return nil, nil, err
	}
	// Expiry may pass while decompressing. Never return an expired payload.
	if !time.Now().Before(r.ExpiresAt) {
		return nil, nil, ErrNotFound
	}
	return refs, &Detail{Reference: r.Reference, RequestBody: string(request), ResponseBody: string(response)}, nil
}

func decompressBody(raw []byte) ([]byte, error) {
	if len(raw) > HardMaxBodyBytes+64*1024 {
		return nil, fmt.Errorf("compressed content exceeds bound")
	}
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	out, err := io.ReadAll(io.LimitReader(reader, HardMaxBodyBytes+1))
	if err != nil || len(out) > HardMaxBodyBytes {
		return nil, fmt.Errorf("invalid or oversized content")
	}
	return out, nil
}
