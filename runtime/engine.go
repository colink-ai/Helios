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
	registry    *Registry
	sink        EventSink
	store       SessionStore
	seq         int64
	mu          sync.RWMutex
	sessions    map[string]Adapter
	sessionOf   map[string]SessionRequest
	strictSink  bool
	strictStore bool
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

func WithStrictEventSink() EngineOption {
	return func(e *Engine) { e.strictSink = true }
}

func WithStrictSessionStore() EngineOption {
	return func(e *Engine) { e.strictStore = true }
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

// DetectCapabilities reports the capabilities of an agent runtime through its
// registered adapter.
func (e *Engine) DetectCapabilities(ctx context.Context, spec AgentSpec) (Capabilities, error) {
	adapter, err := e.registry.Create(spec)
	if err != nil {
		return Capabilities{}, err
	}
	if detector, ok := adapter.(CapabilityDetector); ok {
		capabilities, err := detector.DetectCapabilities(ctx, spec)
		if err != nil {
			return Capabilities{}, err
		}
		if capabilities.AgentType == "" {
			capabilities.AgentType = spec.Type
		}
		if capabilities.AgentName == "" {
			capabilities.AgentName = spec.Name
		}
		return capabilities, nil
	}
	return StaticCapabilities(spec, adapter), nil
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
	if err := e.emit(ctx, eventWith(req, contracts.EventSessionStarted, "")); err != nil && e.strictSink {
		e.cleanupStartedSession(ctx, handle.ID, adapter)
		return nil, err
	}
	if e.store != nil {
		err := e.store.SaveSession(ctx, SessionSnapshot{
			SessionID:      handle.ID,
			RunID:          handle.RunID,
			AgentID:        handle.AgentID,
			AgentType:      req.Agent.Type,
			AgentSessionID: handle.AgentSessionID,
			Status:         handle.Status,
			Metadata:       handle.Metadata,
			UpdatedAt:      time.Now().UTC(),
		})
		if err != nil && e.strictStore {
			e.cleanupStartedSession(ctx, handle.ID, adapter)
			return nil, err
		}
	}
	e.startSessionEventForwarder(ctx, handle.ID, req, adapter)
	return handle, nil
}

func (e *Engine) cleanupStartedSession(ctx context.Context, sessionID string, adapter Adapter) {
	_ = adapter.StopSession(ctx, sessionID)
	e.mu.Lock()
	delete(e.sessions, sessionID)
	delete(e.sessionOf, sessionID)
	e.mu.Unlock()
}

func (e *Engine) startSessionEventForwarder(ctx context.Context, sessionID string, req SessionRequest, adapter Adapter) {
	source, ok := adapter.(SessionEventSource)
	if !ok {
		return
	}
	events, err := source.SessionEvents(ctx, sessionID)
	if err != nil || events == nil {
		return
	}
	go func() {
		for event := range events {
			runtimeEvent := contracts.NewEvent(contracts.EventRuntimeError)
			runtimeEvent.RunID = req.RunID
			runtimeEvent.SessionID = sessionID
			runtimeEvent.AgentID = req.Agent.ID
			runtimeEvent.Error = event.Error
			runtimeEvent.Metadata = map[string]any{"adapterEventType": event.Type}
			for key, value := range event.Metadata {
				runtimeEvent.Metadata[key] = value
			}
			_ = e.emit(ctx, runtimeEvent)
		}
	}()
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
		if err := e.store.DeleteSession(ctx, sessionID); err != nil && e.strictStore {
			return err
		}
	}
	return e.emit(ctx, eventWith(sessionReq, contracts.EventSessionStopped, ""))
}

// SendPermissionResult answers a pending permission request for adapters that
// support host-driven permission decisions.
func (e *Engine) SendPermissionResult(ctx context.Context, sessionID string, permissionID string, decision PermissionDecision) error {
	e.mu.RLock()
	adapter, ok := e.sessions[sessionID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session %s is not active", sessionID)
	}
	sender, ok := adapter.(PermissionResultSender)
	if !ok {
		return fmt.Errorf("adapter for session %s does not support permission results", sessionID)
	}
	return sender.SendPermissionResult(ctx, sessionID, permissionID, decision)
}

func (e *Engine) PendingRequests(ctx context.Context, sessionID string) ([]PendingRequest, error) {
	e.mu.RLock()
	adapter, ok := e.sessions[sessionID]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %s is not active", sessionID)
	}
	inspector, ok := adapter.(PendingRequestInspector)
	if !ok {
		return nil, fmt.Errorf("adapter for session %s does not support pending request inspection", sessionID)
	}
	return inspector.PendingRequests(ctx, sessionID)
}

func (e *Engine) CancelPendingRequest(ctx context.Context, sessionID string, requestID string, reason string) error {
	e.mu.RLock()
	adapter, ok := e.sessions[sessionID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session %s is not active", sessionID)
	}
	inspector, ok := adapter.(PendingRequestInspector)
	if !ok {
		return fmt.Errorf("adapter for session %s does not support pending request cancellation", sessionID)
	}
	return inspector.CancelPendingRequest(ctx, sessionID, requestID, reason)
}

// Diagnostics returns adapter-level session diagnostics when available.
func (e *Engine) Diagnostics(ctx context.Context, sessionID string) (SessionDiagnostics, error) {
	e.mu.RLock()
	adapter, ok := e.sessions[sessionID]
	e.mu.RUnlock()
	if !ok {
		return SessionDiagnostics{}, fmt.Errorf("session %s is not active", sessionID)
	}
	if provider, ok := adapter.(DiagnosticProvider); ok {
		return provider.Diagnostics(ctx, sessionID)
	}
	status, err := adapter.GetSessionStatus(ctx, sessionID)
	if err != nil {
		return SessionDiagnostics{}, err
	}
	return SessionDiagnostics{SessionID: sessionID, Status: status}, nil
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
	if err := e.emit(ctx, started); err != nil && e.strictSink {
		return nil, err
	}

	if native, ok := adapter.(RunAdapter); ok {
		sessionID := NewID("session")
		sessionReq := SessionRequest{
			RunID:             runID,
			SessionID:         sessionID,
			Agent:             req.Agent,
			WorkDir:           req.WorkDir,
			RuntimeConfigMode: req.RuntimeConfigMode,
			RuntimeHome:       req.RuntimeHome,
			ConfigDir:         req.ConfigDir,
			MCPServers:        req.MCPServers,
			Metadata:          req.Metadata,
		}
		if err := e.emit(ctx, eventWith(sessionReq, contracts.EventSessionStarted, "")); err != nil && e.strictSink {
			return nil, err
		}
		result, err := native.Run(ctx, req, func(chunk contracts.Chunk) {
			_ = e.EmitChunk(ctx, runID, sessionID, req.Agent.ID, chunk)
		})
		if err != nil {
			_ = e.emit(ctx, eventWith(sessionReq, contracts.EventSessionStopped, ""))
			_ = e.emit(ctx, contracts.RunEvent{Type: contracts.EventRunFailed, RunID: runID, SessionID: sessionID, AgentID: req.Agent.ID, Error: err.Error(), Timestamp: time.Now().UTC()})
			return nil, err
		}
		if result == nil {
			result = &RunResult{}
		}
		if result.RunID == "" {
			result.RunID = runID
		}
		if result.SessionID != "" && result.SessionID != sessionID && result.AgentSessionID == "" {
			result.AgentSessionID = result.SessionID
		}
		result.SessionID = sessionID
		if err := e.emit(ctx, eventWith(sessionReq, contracts.EventSessionStopped, "")); err != nil && e.strictSink {
			return nil, err
		}
		if err := e.emit(ctx, contracts.RunEvent{Type: contracts.EventRunCompleted, RunID: runID, SessionID: sessionID, AgentID: req.Agent.ID, Timestamp: time.Now().UTC()}); err != nil && e.strictSink {
			return nil, err
		}
		return result, nil
	}

	sessionID := NewID("session")
	handle, err := e.StartSession(ctx, SessionRequest{
		RunID:             runID,
		SessionID:         sessionID,
		Agent:             req.Agent,
		WorkDir:           req.WorkDir,
		RuntimeConfigMode: req.RuntimeConfigMode,
		RuntimeHome:       req.RuntimeHome,
		ConfigDir:         req.ConfigDir,
		MCPServers:        req.MCPServers,
		Metadata:          req.Metadata,
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
	if err := e.emit(ctx, contracts.RunEvent{Type: contracts.EventRunCompleted, RunID: runID, SessionID: handle.ID, AgentID: req.Agent.ID, Timestamp: time.Now().UTC()}); err != nil && e.strictSink {
		return nil, err
	}
	return result, nil
}

// EmitChunk forwards a chunk to the configured event sink.
func (e *Engine) EmitChunk(ctx context.Context, runID, sessionID, agentID string, chunk contracts.Chunk) error {
	event := contracts.NewEvent(eventTypeForChunk(chunk))
	event.RunID = runID
	event.SessionID = sessionID
	event.AgentID = agentID
	event.Chunk = &chunk
	event.Artifact = chunk.Artifact
	event.Handoff = chunk.Handoff
	event.Usage = chunk.Usage
	if chunk.Type == contracts.ChunkError {
		event.Error = chunk.Content
	}
	return e.emit(ctx, event)
}

func eventTypeForChunk(chunk contracts.Chunk) contracts.EventType {
	switch chunk.Type {
	case contracts.ChunkToolUse:
		return contracts.EventToolStarted
	case contracts.ChunkInputJSONDelta:
		return contracts.EventToolInputDelta
	case contracts.ChunkToolResult:
		if chunk.IsError {
			return contracts.EventToolFailed
		}
		return contracts.EventToolCompleted
	case contracts.ChunkQuestion:
		return contracts.EventQuestionAsked
	case contracts.ChunkPermission:
		return contracts.EventPermissionAsked
	case contracts.ChunkUsage:
		return contracts.EventUsageReported
	case contracts.ChunkStatus:
		if len(chunk.Plan) > 0 {
			return contracts.EventPlanUpdated
		}
	case contracts.ChunkArtifact:
		return contracts.EventArtifactCreated
	case contracts.ChunkHandoff:
		return contracts.EventHandoffCreated
	case contracts.ChunkError:
		return contracts.EventRuntimeError
	}
	return contracts.EventChunk
}

func (e *Engine) emit(ctx context.Context, event contracts.RunEvent) error {
	event.Sequence = atomic.AddInt64(&e.seq, 1)
	if event.SchemaVersion == "" {
		event.SchemaVersion = contracts.SemanticSchemaVersion
	}
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
