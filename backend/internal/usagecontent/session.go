package usagecontent

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// bodyBuffer grows in small chunks. Both live requests and queued jobs count
// against one process-wide byte budget; reaching it never blocks forwarding.
type bodyBuffer struct {
	chunks    [][]byte
	length    int
	bytes     int64
	truncated bool
	omitted   string
}

func (b *bodyBuffer) append(m *Manager, p []byte) {
	b.bytes += int64(len(p))
	if b.omitted != "" || len(p) == 0 {
		return
	}
	for len(p) > 0 && b.length < m.config.MaxBodyBytes {
		if len(b.chunks) == 0 || len(b.chunks[len(b.chunks)-1]) == cap(b.chunks[len(b.chunks)-1]) {
			n := min(chunkSize, m.config.MaxBodyBytes-b.length)
			if !m.reserve(int64(n)) {
				b.truncated = true
				b.omitted = "buffer_capacity"
				return
			}
			b.chunks = append(b.chunks, make([]byte, 0, n))
		}
		last := len(b.chunks) - 1
		n := min(len(p), cap(b.chunks[last])-len(b.chunks[last]))
		b.chunks[last] = append(b.chunks[last], p[:n]...)
		b.length += n
		p = p[n:]
	}
	if len(p) > 0 {
		b.truncated = true
	}
}

func (b *bodyBuffer) release(m *Manager) {
	var reserved int64
	for _, p := range b.chunks {
		reserved += int64(cap(p))
	}
	m.buffered.Add(-reserved)
	b.chunks = nil
}

type binding struct {
	requestID string
}

type Session struct {
	mu        sync.Mutex
	m         *Manager
	apiKeyID  int64
	ref       Reference
	input     bodyBuffer
	output    bodyBuffer
	inputDone bool
	finished  bool
	closed    bool
	pending   int
	bindings  []binding
}

func (m *Manager) Begin(apiKeyID int64, clientRequestID, method, path, protocol, inputType string) *Session {
	if !m.Enabled() || apiKeyID <= 0 {
		return nil
	}
	if m.storageFull.Load() {
		m.dropped.Add(1)
		return nil
	}
	if m.active.Add(1) > MaxSessions {
		m.active.Add(-1)
		m.dropped.Add(1)
		return nil
	}
	now := time.Now().UTC()
	s := &Session{m: m, apiKeyID: apiKeyID, ref: Reference{
		CaptureID: uuid.NewString(), ClientRequestID: bounded(clientRequestID, 128),
		CreatedAt: now, ExpiresAt: now.Add(Retention),
		Metadata: Metadata{Method: bounded(method, 16), Path: bounded(path, 512), Protocol: protocol, RequestContentType: bounded(inputType, 128)},
	}}
	if !IsTextContentType(inputType) {
		s.input.omitted = "binary_content"
	}
	return s
}

func IsTextContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	return ct == "" || strings.HasPrefix(ct, "text/") || ct == "application/json" || strings.HasSuffix(ct, "+json") || ct == "application/x-ndjson"
}

func bounded(value string, n int) string {
	if len(value) <= n {
		return strings.Clone(value)
	}
	value = value[:n]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return strings.Clone(value)
}

// InputBytes observes a directly consumed, uncompressed request body.
func (s *Session) InputBytes(p []byte) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && !s.inputDone {
		s.input.append(s.m, p)
	}
}

// Input captures the first decoded/original body only. Later model rewrites,
// prereads and repeated helper reads cannot replace it.
func (s *Session) Input(p []byte, ct string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.inputDone {
		return
	}
	s.ref.RequestContentType = bounded(ct, 128)
	if s.input.bytes == 0 {
		s.input.append(s.m, p)
	}
	s.inputDone = true
}

func (s *Session) Output(p []byte, ct string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.finished {
		return
	}
	s.ref.ResponseContentType = bounded(ct, 128)
	if !IsTextContentType(ct) {
		s.output.omitted = "binary_content"
	}
	s.output.append(s.m, p)
}

// Acquire retains this session for a queued billing task. Release must also be
// called for abandoned/dropped tasks, and is safe to call more than once.
func (s *Session) Acquire() func() {
	if s == nil {
		return func() {}
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return func() {}
	}
	s.pending++
	s.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			s.pending--
			s.finalizeLocked()
			s.mu.Unlock()
		})
	}
}

func (s *Session) Bind(requestID string, apiKeyID int64) {
	if s == nil || requestID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.apiKeyID != apiKeyID {
		return
	}
	requestID = bounded(requestID, 128)
	for _, b := range s.bindings {
		if b.requestID == requestID {
			return
		}
	}
	if len(s.bindings) < 8 {
		s.bindings = append(s.bindings, binding{requestID: requestID})
	}
}

func (s *Session) Finish(status int, partial bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.finished = true
	s.ref.Status = status
	s.ref.Partial = partial
	s.finalizeLocked()
}

// The caller holds s.mu. Only bounded copies and a nonblocking enqueue happen
// here; all gzip and SQL work runs on the single background writer.
func (s *Session) finalizeLocked() {
	if !s.finished || s.pending != 0 || s.closed {
		return
	}
	s.closed = true
	s.ref.RequestBytes, s.ref.ResponseBytes = s.input.bytes, s.output.bytes
	s.ref.RequestStoredBytes, s.ref.ResponseStoredBytes = s.input.length, s.output.length
	s.ref.RequestTruncated, s.ref.ResponseTruncated = s.input.truncated, s.output.truncated
	s.ref.RequestOmitted, s.ref.ResponseOmitted = s.input.omitted, s.output.omitted
	if s.input.truncated || s.output.truncated {
		s.m.truncated.Add(1)
	}
	j := job{ref: s.ref, apiKeyID: s.apiKeyID, input: s.input, output: s.output, bindings: s.bindings}
	s.input.chunks, s.output.chunks, s.bindings = nil, nil, nil
	s.m.active.Add(-1)
	s.m.enqueue(j)
}
