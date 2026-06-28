package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/colink-ai/helios/contracts"
)

// EventSink receives normalized runtime events. Host applications can implement
// it to persist events into their own database schema.
type EventSink interface {
	OnRunEvent(ctx context.Context, event contracts.RunEvent) error
}

// EventSinkFunc adapts a function into an EventSink.
type EventSinkFunc func(context.Context, contracts.RunEvent) error

func (f EventSinkFunc) OnRunEvent(ctx context.Context, event contracts.RunEvent) error {
	return f(ctx, event)
}

// NoopEventSink discards all events.
type NoopEventSink struct{}

func (NoopEventSink) OnRunEvent(context.Context, contracts.RunEvent) error { return nil }

// SessionSnapshot is the SDK-level resume state. It is intentionally small and
// host applications may store richer records in their own schema.
type SessionSnapshot struct {
	SessionID      string         `json:"sessionId"`
	RunID          string         `json:"runId,omitempty"`
	AgentID        string         `json:"agentId,omitempty"`
	AgentType      string         `json:"agentType,omitempty"`
	AgentSessionID string         `json:"agentSessionId,omitempty"`
	Status         SessionStatus  `json:"status,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// SessionStore is optional resume metadata storage implemented by applications.
type SessionStore interface {
	SaveSession(ctx context.Context, snapshot SessionSnapshot) error
	LoadSession(ctx context.Context, sessionID string) (*SessionSnapshot, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// MemorySessionStore is a lightweight in-memory store for tests and embedded
// prototypes. It is not a durable database.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]SessionSnapshot
}

// NewMemorySessionStore creates an empty in-memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: map[string]SessionSnapshot{}}
}

func (s *MemorySessionStore) SaveSession(_ context.Context, snapshot SessionSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now().UTC()
	}
	s.sessions[snapshot.SessionID] = snapshot
	return nil
}

func (s *MemorySessionStore) LoadSession(_ context.Context, sessionID string) (*SessionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	return &snapshot, nil
}

func (s *MemorySessionStore) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}
