package usagecontent

import (
	"container/list"
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type job struct {
	ref           Reference
	apiKeyID      int64
	input, output bodyBuffer
	bindings      []binding
}

func (j *job) release(m *Manager) { j.input.release(m); j.output.release(m) }

type lookupKey struct {
	apiKeyID int64
	client   bool
	id       string
}
type windowEntry struct {
	job
	keys     []lookupKey
	element  *list.Element
	sequence uint64
	bytes    int64
	pins     int
}

// Manager owns a process-local FIFO of completed captures. Publishing moves
// immutable chunks, never compresses, copies a whole body, or accesses SQL.
type Manager struct {
	config                             Config
	mu                                 sync.Mutex
	window                             list.List
	index                              map[lookupKey]map[string]*windowEntry
	retainedBytes                      int64 // Includes evicted entries still pinned by HTTP readers.
	sequence                           uint64
	readers                            chan struct{}
	pool                               chan []byte
	pooled                             atomic.Int64
	readBytes                          atomic.Int64
	done, closed                       chan struct{}
	closeOnce                          sync.Once
	stopping                           atomic.Bool
	active, buffered                   atomic.Int64
	saved, dropped, truncated, deleted atomic.Uint64
}

func New(config Config) *Manager {
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = DefaultMaxBodyBytes
	}
	config.MaxBodyBytes = min(config.MaxBodyBytes, HardMaxBodyBytes)
	if config.MaxRecords <= 0 {
		config.MaxRecords = DefaultMaxRecords
	}
	config.MaxRecords = min(config.MaxRecords, HardMaxRecords)
	if config.MaxMemoryBytes <= 0 {
		config.MaxMemoryBytes = DefaultMemoryBytes
	}
	config.MaxMemoryBytes = min(config.MaxMemoryBytes, HardMemoryBytes)
	m := &Manager{config: config, index: make(map[lookupKey]map[string]*windowEntry), readers: make(chan struct{}, 2),
		pool: make(chan []byte, poolChunks), done: make(chan struct{}), closed: make(chan struct{})}
	go func() {
		defer close(m.closed)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.cleanup()
			case <-m.done:
				return
			}
		}
	}()
	return m
}
func (m *Manager) Enabled() bool { return m != nil && m.config.Enabled && !m.stopping.Load() }

func (m *Manager) reserve(n int64) bool {
	for {
		old := m.buffered.Load()
		if m.stopping.Load() || n < 0 || old+n > BufferLimit {
			return false
		}
		if m.buffered.CompareAndSwap(old, old+n) {
			return true
		}
	}
}
func (m *Manager) chunk(n int) []byte {
	if n == chunkSize {
		select {
		case p := <-m.pool:
			m.pooled.Add(-int64(cap(p)))
			return p[:0]
		default:
		}
	}
	return make([]byte, 0, n)
}
func (m *Manager) recycle(chunks [][]byte) {
	for _, p := range chunks {
		if cap(p) != chunkSize || m.stopping.Load() {
			continue
		}
		// Only overwritten logical bytes are exposed; readers pin immutable chunks.
		// This internal pool is strictly bounded, unlike sync.Pool.
		m.pooled.Add(int64(cap(p)))
		select {
		case m.pool <- p[:0]:
		default:
			m.pooled.Add(-int64(cap(p)))
		}
	}
}
func (m *Manager) publish(j job) {
	size := j.input.capacity() + j.output.capacity()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopping.Load() || !time.Now().Before(j.ref.ExpiresAt) || size > m.config.MaxMemoryBytes {
		j.release(m)
		m.dropped.Add(1)
		return
	}
	for m.window.Len() > 0 && (m.window.Len() >= m.config.MaxRecords || m.retainedBytes+size > m.config.MaxMemoryBytes) {
		m.evictLocked(m.window.Front().Value.(*windowEntry))
	}
	// Slow readers can pin the budget even after eviction. Never grow beyond
	// the limit or wait for a reader on the gateway completion path.
	if m.retainedBytes+size > m.config.MaxMemoryBytes {
		j.release(m)
		m.dropped.Add(1)
		return
	}
	m.sequence++
	e := &windowEntry{job: j, sequence: m.sequence, bytes: size}
	e.keys = append(e.keys, lookupKey{j.apiKeyID, true, j.ref.ClientRequestID})
	for _, b := range j.bindings {
		e.keys = append(e.keys, lookupKey{j.apiKeyID, false, b.requestID})
	}
	e.element = m.window.PushBack(e)
	for _, key := range e.keys {
		if key.id == "" {
			continue
		}
		if m.index[key] == nil {
			m.index[key] = make(map[string]*windowEntry)
		}
		m.index[key][e.ref.CaptureID] = e
	}
	m.retainedBytes += size
	m.buffered.Add(-size) // Transfer ownership, without releasing/reallocating chunks.
	m.saved.Add(1)
}
func (m *Manager) evictLocked(e *windowEntry) {
	if e.element == nil {
		return
	}
	m.window.Remove(e.element)
	e.element = nil
	for _, key := range e.keys {
		delete(m.index[key], e.ref.CaptureID)
		if len(m.index[key]) == 0 {
			delete(m.index, key)
		}
	}
	m.deleted.Add(1)
	if e.pins == 0 {
		m.freeLocked(e)
	}
}
func (m *Manager) freeLocked(e *windowEntry) {
	m.retainedBytes -= e.bytes
	m.recycle(e.input.chunks)
	m.recycle(e.output.chunks)
	e.input.chunks, e.output.chunks = nil, nil
}
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	// TTL follows capture start, not completion, so expiry is not FIFO ordered.
	for p := m.window.Front(); p != nil; {
		next := p.Next()
		e := p.Value.(*windowEntry)
		if !now.Before(e.ref.ExpiresAt) {
			m.evictLocked(e)
		}
		p = next
	}
}
func (m *Manager) Stats() Stats {
	if m == nil {
		return Stats{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return Stats{Active: m.active.Load(), BufferedBytes: m.buffered.Load(), Saved: m.saved.Load(), Dropped: m.dropped.Load(),
		Truncated: m.truncated.Load(), Deleted: m.deleted.Load(), Records: m.window.Len(), RetainedBytes: m.retainedBytes,
		PooledBytes: m.pooled.Load(), ReadBytes: m.readBytes.Load(), MaxRecords: m.config.MaxRecords, MaxMemoryBytes: m.config.MaxMemoryBytes}
}
func (m *Manager) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.stopping.Store(true)
		for m.window.Len() > 0 {
			m.evictLocked(m.window.Front().Value.(*windowEntry))
		}
		for {
			select {
			case p := <-m.pool:
				m.pooled.Add(-int64(cap(p)))
			default:
				goto drained
			}
		}
	drained:
		close(m.done)
		m.mu.Unlock()
	})
	select {
	case <-m.closed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Read pins one entry and a reader slot until release, INCLUDING HTTP JSON
// serialization. Evicted/pinned bytes remain charged; at most two body copies
// (each bounded by two MaxBodyBytes) can be served concurrently. Call release
// even on cancellation. No body copy occurs under the window lock.
func (m *Manager) Read(ctx context.Context, query Query) ([]Reference, *Detail, func(), error) {
	noop := func() {}
	if err := ctx.Err(); err != nil {
		return nil, nil, noop, err
	}
	select {
	case m.readers <- struct{}{}:
	default:
		return nil, nil, noop, ErrBusy
	}
	key := lookupKey{apiKeyID: query.APIKeyID, id: query.RequestID}
	if query.ClientRequestID != "" {
		key.client, key.id = true, query.ClientRequestID
	}
	m.mu.Lock()
	var candidates []*windowEntry
	now := time.Now()
	if query.APIKeyID > 0 && key.id != "" {
		for _, e := range m.index[key] {
			if now.Before(e.ref.ExpiresAt) {
				candidates = append(candidates, e)
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].sequence > candidates[j].sequence })
	var chosen *windowEntry
	refs := make([]Reference, 0, min(20, len(candidates)))
	for i, e := range candidates {
		ref := e.ref
		if !key.client {
			ref.RequestID = key.id
		} else if len(e.bindings) > 0 {
			ref.RequestID = e.bindings[0].requestID
		}
		if i < 20 {
			refs = append(refs, ref)
		}
		if query.CaptureID == e.ref.CaptureID || (query.CaptureID == "" && i == 0) {
			chosen = e
		}
	}
	if chosen == nil {
		m.mu.Unlock()
		<-m.readers
		return refs, nil, noop, nil
	}
	chosen.pins++
	ref := chosen.ref
	if !key.client {
		ref.RequestID = key.id
	} else if len(chosen.bindings) > 0 {
		ref.RequestID = chosen.bindings[0].requestID
	}
	readSize := int64(chosen.input.length + chosen.output.length)
	m.readBytes.Add(readSize)
	m.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			m.mu.Lock()
			chosen.pins--
			if chosen.pins == 0 && chosen.element == nil {
				m.freeLocked(chosen)
			}
			m.mu.Unlock()
			m.readBytes.Add(-readSize)
			<-m.readers
		})
	}
	detail := &Detail{Reference: ref, RequestBody: chosen.input.text(), ResponseBody: chosen.output.text()}
	if err := ctx.Err(); err != nil {
		release()
		return nil, nil, noop, err
	}
	if !time.Now().Before(ref.ExpiresAt) {
		release()
		return nil, nil, noop, ErrNotFound
	}
	return refs, detail, release, nil
}
