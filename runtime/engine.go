package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/colink-ai/helios/contracts"
)

// Engine wires adapters, event emission, and optional session persistence.
type Engine struct {
	registry  *Registry
	sink      EventSink
	store     SessionStore
	seq       int64
	mu        sync.RWMutex
	sessions  map[string]Adapter
	sessionOf map[string]SessionRequest
}

// EngineOption configures an Engine.
type EngineOption func(*Engine)

func WithEventSink(sink EventSink) EngineOption {
	return func(e *Engine) {
		if sink != nil {
			e.sink = sink
		}
	}
}

func WithSessionStore(store SessionStore) EngineOption {
	return func(e *Engine) { e.store = store }
}

// NewEngine creates a runtime engine.
func NewEngine(registry *Registry, opts ...EngineOption) *Engine {
	if registry == nil {
		registry = NewRegistry()
	}
	e := &Engine{
		registry:  registry,
		sink:      NoopEventSink{},
		sessions:  map[string]Adapter{},
		sessionOf: map[string]SessionRequest{},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// StartSession creates an adapter session and emits normalized events.
func (e *Engine) StartSession(ctx context.Context, req SessionRequest) (*SessionHandle, error) {
	adapter, err := e.registry.Create(req.Agent)
	if err != nil {
		return nil, err
	}
	handle, err := adapter.StartSession(ctx, req)
	if err != nil {
		_ = e.emit(ctx, eventWith(req, contracts.EventRunFailed, err.Error()))
		return nil, err
	}
	if handle == nil {
		return nil, fmt.Errorf("adapter returned nil session handle")
	}
	if handle.ID == "" {
		handle.ID = req.SessionID
	}
	if handle.Status == "" {
		handle.Status = SessionRunning
	}
	e.mu.Lock()
	e.sessions[handle.ID] = adapter
	req.SessionID = handle.ID
	e.sessionOf[handle.ID] = req
	e.mu.Unlock()
	_ = e.emit(ctx, eventWith(req, contracts.EventSessionStarted, ""))
	if e.store != nil {
		_ = e.store.SaveSession(ctx, SessionSnapshot{
			SessionID:      handle.ID,
			RunID:          handle.RunID,
			AgentID:        handle.AgentID,
			AgentType:      req.Agent.Type,
			AgentSessionID: handle.AgentSessionID,
			Status:         handle.Status,
			Metadata:       handle.Metadata,
			UpdatedAt:      time.Now().UTC(),
		})
	}
	return handle, nil
}

// Prompt sends input to an active session and emits each normalized chunk.
func (e *Engine) Prompt(ctx context.Context, req PromptRequest) (*RunResult, error) {
	e.mu.RLock()
	adapter, ok := e.sessions[req.SessionID]
	sessionReq := e.sessionOf[req.SessionID]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %s is not active", req.SessionID)
	}
	wrapped := func(chunk contracts.Chunk) {
		_ = e.EmitChunk(ctx, sessionReq.RunID, req.SessionID, sessionReq.Agent.ID, chunk)
	}
	result, err := adapter.Prompt(ctx, req, wrapped)
	if err != nil {
		_ = e.emit(ctx, eventWith(sessionReq, contracts.EventRunFailed, err.Error()))
		return nil, err
	}
	if result != nil && result.Usage != nil {
		event := contracts.NewEvent(contracts.EventUsageReported)
		event.RunID = sessionReq.RunID
		event.SessionID = req.SessionID
		event.AgentID = sessionReq.Agent.ID
		event.Usage = result.Usage
		_ = e.emit(ctx, event)
	}
	return result, nil
}

// StopSession stops an active session and removes process-local runtime state.
func (e *Engine) StopSession(ctx context.Context, sessionID string) error {
	e.mu.RLock()
	adapter, ok := e.sessions[sessionID]
	sessionReq := e.sessionOf[sessionID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session %s is not active", sessionID)
	}
	if err := adapter.StopSession(ctx, sessionID); err != nil {
		return err
	}
	e.mu.Lock()
	delete(e.sessions, sessionID)
	delete(e.sessionOf, sessionID)
	e.mu.Unlock()
	if e.store != nil {
		_ = e.store.DeleteSession(ctx, sessionID)
	}
	return e.emit(ctx, eventWith(sessionReq, contracts.EventSessionStopped, ""))
}

// Run executes a one-shot request. Native RunAdapter implementations are used
// directly; other adapters are driven through a temporary session.
func (e *Engine) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	adapter, err := e.registry.Create(req.Agent)
	if err != nil {
		return nil, err
	}
	runID := req.RunID
	if runID == "" {
		runID = NewID("run")
		req.RunID = runID
	}
	started := contracts.NewEvent(contracts.EventRunStarted)
	started.RunID = runID
	started.AgentID = req.Agent.ID
	_ = e.emit(ctx, started)

	if native, ok := adapter.(RunAdapter); ok {
		result, err := native.Run(ctx, req, func(chunk contracts.Chunk) {
			_ = e.EmitChunk(ctx, runID, "", req.Agent.ID, chunk)
		})
		if err != nil {
			_ = e.emit(ctx, contracts.RunEvent{Type: contracts.EventRunFailed, RunID: runID, AgentID: req.Agent.ID, Error: err.Error(), Timestamp: time.Now().UTC()})
			return nil, err
		}
		_ = e.emit(ctx, contracts.RunEvent{Type: contracts.EventRunCompleted, RunID: runID, AgentID: req.Agent.ID, Timestamp: time.Now().UTC()})
		return result, nil
	}

	sessionID := NewID("session")
	handle, err := e.StartSession(ctx, SessionRequest{
		RunID:       runID,
		SessionID:   sessionID,
		Agent:       req.Agent,
		WorkDir:     req.WorkDir,
		RuntimeHome: req.RuntimeHome,
		MCPServers:  req.MCPServers,
		Metadata:    req.Metadata,
	})
	if err != nil {
		return nil, err
	}
	result, promptErr := e.Prompt(ctx, PromptRequest{
		SessionID: handle.ID,
		Input:     req.Input,
		Images:    req.Images,
		Metadata:  req.Metadata,
	})
	stopErr := e.StopSession(ctx, handle.ID)
	if promptErr != nil {
		return nil, promptErr
	}
	if stopErr != nil {
		return nil, stopErr
	}
	_ = e.emit(ctx, contracts.RunEvent{Type: contracts.EventRunCompleted, RunID: runID, SessionID: handle.ID, AgentID: req.Agent.ID, Timestamp: time.Now().UTC()})
	return result, nil
}

// EmitChunk forwards a chunk to the configured event sink.
func (e *Engine) EmitChunk(ctx context.Context, runID, sessionID, agentID string, chunk contracts.Chunk) error {
	event := contracts.NewEvent(contracts.EventChunk)
	event.RunID = runID
	event.SessionID = sessionID
	event.AgentID = agentID
	event.Chunk = &chunk
	return e.emit(ctx, event)
}

func (e *Engine) emit(ctx context.Context, event contracts.RunEvent) error {
	event.Sequence = atomic.AddInt64(&e.seq, 1)
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	return e.sink.OnRunEvent(ctx, event)
}

func eventWith(req SessionRequest, typ contracts.EventType, errText string) contracts.RunEvent {
	event := contracts.NewEvent(typ)
	event.RunID = req.RunID
	event.SessionID = req.SessionID
	event.AgentID = req.Agent.ID
	event.Error = errText
	return event
}
