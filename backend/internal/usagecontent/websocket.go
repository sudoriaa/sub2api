package usagecontent

import "sync"

// Connection is a small WS turn router, not a connection-long payload buffer.
// Billing tasks copy a Session before the next response.create can replace it.
type Connection struct {
	mu       sync.Mutex
	m        *Manager
	apiKeyID int64
	clientID string
	path     string
	current  *Session
	turn     int
	turns    map[int]*Session
	holds    map[int]func()
	closed   bool
}

func NewConnection(m *Manager, apiKeyID int64, clientID, path string) *Connection {
	return &Connection{m: m, apiKeyID: apiKeyID, clientID: bounded(clientID, 128), path: bounded(path, 512), turns: map[int]*Session{}, holds: map[int]func(){}}
}

// InitialInput is called before the first client frame is normalized/mapped.
func (c *Connection) InitialInput(payload []byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed && c.current == nil {
		c.beginLocked(1, payload)
	}
}

func (c *Connection) BeginTurn(turn int, payload []byte) *Session {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	if c.turn == turn && c.current != nil && !c.current.isFinished() {
		return c.current
	}
	return c.beginLocked(turn, payload)
}

func (c *Connection) beginLocked(turn int, payload []byte) *Session {
	// A following frame can be admitted just after terminal delivery but
	// before the previous AfterTurn callback binds its usage. Keep that old
	// session alive; never prematurely mark a successful turn as partial.
	if old := c.turns[turn]; old != nil {
		old.Finish(101, true)
		if done := c.holds[turn]; done != nil {
			done()
		}
	}
	s := c.m.Begin(c.apiKeyID, c.clientID, "GET", c.path, "websocket", "application/json")
	s.Input(payload, "application/json")
	c.current, c.turn = s, turn
	c.turns[turn] = s
	c.holds[turn] = s.Acquire()
	for key, old := range c.turns {
		if len(c.turns) > 4 && key != turn && old.isFinished() && c.holds[key] == nil {
			delete(c.turns, key)
		}
	}
	return s
}

func (s *Session) isFinished() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finished
}

func (c *Connection) Current() *Session {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *Connection) ForTurn(turn int) *Session {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.turns[turn]
}

// TakeTurn transfers the in-flight turn reference to its completion callback.
// Release after Finish and after queuing billing tasks (which acquire their
// own references). The callback's session cannot be changed by another frame.
func (c *Connection) TakeTurn(turn int) (*Session, func()) {
	if c == nil {
		return nil, func() {}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s, done := c.turns[turn], c.holds[turn]
	delete(c.holds, turn)
	if done == nil {
		done = func() {}
	}
	return s, done
}

func (c *Connection) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	for _, s := range c.turns {
		s.Finish(101, true)
	}
	for _, done := range c.holds {
		done()
	}
	c.turns = nil
	c.holds = nil
	c.current = nil
}

// OutputFrame preserves delivered frame order as SSE data events for one turn.
func (s *Session) OutputFrame(payload []byte) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished || s.closed {
		return
	}
	s.ref.ResponseContentType = "text/event-stream; profile=websocket-frames"
	s.output.append(s.m, []byte("data: "))
	s.output.append(s.m, payload)
	s.output.append(s.m, []byte("\n\n"))
}
