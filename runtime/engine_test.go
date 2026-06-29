package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/colink-ai/helios/contracts"
)

type nativeRunAdapter struct {
	testAdapter
	fail bool
}

func (a nativeRunAdapter) Run(_ context.Context, _ RunRequest, onChunk ChunkHandler) (*RunResult, error) {
	if a.fail {
		return nil, fmt.Errorf("native failed")
	}
	onChunk(contracts.Chunk{Type: contracts.ChunkText, Content: "native chunk"})
	return &RunResult{Output: "native ok", SessionID: "native-session"}, nil
}

type failingPromptAdapter struct {
	testAdapter
}

func (failingPromptAdapter) Prompt(context.Context, PromptRequest, ChunkHandler) (*RunResult, error) {
	return nil, fmt.Errorf("prompt failed")
}

type stopFailAdapter struct {
	testAdapter
}

func (stopFailAdapter) StopSession(context.Context, string) error {
	return fmt.Errorf("stop failed")
}

type usagePromptAdapter struct {
	testAdapter
}

func (usagePromptAdapter) Prompt(context.Context, PromptRequest, ChunkHandler) (*RunResult, error) {
	return &RunResult{Output: "ok", Usage: &contracts.TokenUsage{InputTokens: 3, OutputTokens: 5}}, nil
}

type startFailAdapter struct {
	testAdapter
}

func (startFailAdapter) StartSession(context.Context, SessionRequest) (*SessionHandle, error) {
	return nil, fmt.Errorf("start failed")
}

type nilHandleAdapter struct {
	testAdapter
}

func (nilHandleAdapter) StartSession(context.Context, SessionRequest) (*SessionHandle, error) {
	return nil, nil
}

type diagnosticAdapter struct {
	testAdapter
}

func (diagnosticAdapter) Diagnostics(context.Context, string) (SessionDiagnostics, error) {
	return SessionDiagnostics{SessionID: "session-diag-provider", Status: SessionRunning, Metadata: map[string]any{"source": "provider"}}, nil
}

func TestEngineStartSessionEmitsAndStores(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	var events []contracts.RunEvent
	sink := EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		events = append(events, event)
		return nil
	})
	store := NewMemorySessionStore()
	engine := NewEngine(reg, WithEventSink(sink), WithSessionStore(store))

	handle, err := engine.StartSession(ctx, SessionRequest{
		RunID:     "run-1",
		SessionID: "session-1",
		Agent:     AgentSpec{ID: "agent-1", Type: "test"},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if handle.ID != "session-1" {
		t.Fatalf("handle id = %q", handle.ID)
	}
	if len(events) != 1 || events[0].Type != contracts.EventSessionStarted || events[0].Sequence != 1 {
		t.Fatalf("unexpected events: %+v", events)
	}
	if events[0].SchemaVersion != contracts.SemanticSchemaVersion {
		t.Fatalf("schema version = %q", events[0].SchemaVersion)
	}
	snapshot, err := store.LoadSession(ctx, "session-1")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot == nil || snapshot.AgentType != "test" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if err := engine.EmitChunk(ctx, "run-1", "session-1", "agent-1", contracts.Chunk{Type: contracts.ChunkText, Content: "hi"}); err != nil {
		t.Fatalf("emit chunk: %v", err)
	}
	if len(events) != 2 || events[1].Chunk.Content != "hi" || events[1].Sequence != 2 {
		t.Fatalf("unexpected chunk event: %+v", events)
	}
}

type failingStore struct {
	SessionStore
}

func (failingStore) SaveSession(context.Context, SessionSnapshot) error {
	return fmt.Errorf("store failed")
}

type cleanupAdapter struct {
	testAdapter
	stopped int
}

func (a *cleanupAdapter) StartSession(context.Context, SessionRequest) (*SessionHandle, error) {
	return &SessionHandle{ID: "cleanup-session", Status: SessionRunning}, nil
}

func (a *cleanupAdapter) StopSession(context.Context, string) error {
	a.stopped++
	return nil
}

func TestEngineStrictEventSink(t *testing.T) {
	ctx := context.Background()
	adapter := &cleanupAdapter{}
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return adapter, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg, WithStrictEventSink(), WithEventSink(EventSinkFunc(func(context.Context, contracts.RunEvent) error {
		return fmt.Errorf("sink failed")
	})))
	if _, err := engine.StartSession(ctx, SessionRequest{SessionID: "strict-sink", Agent: AgentSpec{Type: "test"}}); err == nil {
		t.Fatalf("strict sink should fail")
	}
	if adapter.stopped != 1 {
		t.Fatalf("expected cleanup stop, got %d", adapter.stopped)
	}
	if _, err := engine.Diagnostics(ctx, "cleanup-session"); err == nil {
		t.Fatalf("failed strict start should not leave active session")
	}
}

func TestEngineStrictSessionStore(t *testing.T) {
	ctx := context.Background()
	adapter := &cleanupAdapter{}
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return adapter, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg, WithStrictSessionStore(), WithSessionStore(failingStore{}))
	if _, err := engine.StartSession(ctx, SessionRequest{SessionID: "strict-store", Agent: AgentSpec{Type: "test"}}); err == nil {
		t.Fatalf("strict store should fail")
	}
	if adapter.stopped != 1 {
		t.Fatalf("expected cleanup stop, got %d", adapter.stopped)
	}
}

func TestEnginePromptAndStop(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	var events []contracts.RunEvent
	engine := NewEngine(reg, WithEventSink(EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		events = append(events, event)
		return nil
	})))
	handle, err := engine.StartSession(ctx, SessionRequest{
		RunID:     "run-2",
		SessionID: "session-2",
		Agent:     AgentSpec{ID: "agent-2", Type: "test"},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	result, err := engine.Prompt(ctx, PromptRequest{SessionID: handle.ID, Input: "hello"})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if result.Output != "ok" {
		t.Fatalf("output = %q", result.Output)
	}
	if err := engine.StopSession(ctx, handle.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(events) != 2 || events[1].Type != contracts.EventSessionStopped {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestEnginePromptReportsUsage(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "usage",
		Factory: func(AgentSpec) (Adapter, error) {
			return usagePromptAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	var events []contracts.RunEvent
	engine := NewEngine(reg, WithEventSink(EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		events = append(events, event)
		return nil
	})))
	handle, err := engine.StartSession(ctx, SessionRequest{SessionID: "usage-session", Agent: AgentSpec{Type: "usage"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := engine.Prompt(ctx, PromptRequest{SessionID: handle.ID, Input: "hello"}); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if len(events) != 2 || events[1].Type != contracts.EventUsageReported || events[1].Usage.OutputTokens != 5 {
		t.Fatalf("usage event not reported: %+v", events)
	}
}

func TestEnginePromptFailure(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "fail-prompt",
		Factory: func(AgentSpec) (Adapter, error) {
			return failingPromptAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg)
	handle, err := engine.StartSession(ctx, SessionRequest{SessionID: "prompt-fail", Agent: AgentSpec{Type: "fail-prompt"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := engine.Prompt(ctx, PromptRequest{SessionID: handle.ID, Input: "hello"}); err == nil {
		t.Fatalf("prompt should fail")
	}
}

func TestEngineStartSessionFailures(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "start-fail",
		Factory: func(AgentSpec) (Adapter, error) {
			return startFailAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register start fail: %v", err)
	}
	if err := reg.Register(AdapterMeta{
		Type: "nil-handle",
		Factory: func(AgentSpec) (Adapter, error) {
			return nilHandleAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register nil handle: %v", err)
	}
	var events []contracts.RunEvent
	engine := NewEngine(reg, WithEventSink(EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		events = append(events, event)
		return nil
	})))
	if _, err := engine.StartSession(ctx, SessionRequest{RunID: "run-start-fail", Agent: AgentSpec{Type: "start-fail"}}); err == nil {
		t.Fatalf("start failure should be returned")
	}
	if len(events) != 1 || events[0].Type != contracts.EventRunFailed || events[0].Error != "start failed" {
		t.Fatalf("run failed event not emitted: %+v", events)
	}
	if _, err := engine.StartSession(ctx, SessionRequest{Agent: AgentSpec{Type: "nil-handle"}}); err == nil {
		t.Fatalf("nil handle should fail")
	}
}

func TestEngineRunNative(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "native",
		Factory: func(AgentSpec) (Adapter, error) {
			return nativeRunAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	var events []contracts.RunEvent
	engine := NewEngine(reg, WithEventSink(EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		events = append(events, event)
		return nil
	})))
	result, err := engine.Run(ctx, RunRequest{RunID: "run-native", Agent: AgentSpec{Type: "native"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Output != "native ok" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(events) != 5 {
		t.Fatalf("unexpected events: %+v", events)
	}
	if events[1].Type != contracts.EventSessionStarted || events[1].SessionID != "native-session" {
		t.Fatalf("native run should emit session start with session id: %+v", events)
	}
	if events[2].Chunk.Content != "native chunk" || events[2].SessionID != "native-session" {
		t.Fatalf("native chunk should include session id: %+v", events)
	}
	if events[3].Type != contracts.EventSessionStopped || events[4].Type != contracts.EventRunCompleted || events[4].SessionID != "native-session" {
		t.Fatalf("native run should emit stop and completion with session id: %+v", events)
	}
}

func TestEngineRunNativeFailure(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "native-fail",
		Factory: func(AgentSpec) (Adapter, error) {
			return nativeRunAdapter{fail: true}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg)
	if _, err := engine.Run(ctx, RunRequest{Agent: AgentSpec{Type: "native-fail"}}); err == nil {
		t.Fatalf("native run should fail")
	}
}

func TestEngineRunStopFailure(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "stop-fail",
		Factory: func(AgentSpec) (Adapter, error) {
			return stopFailAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg)
	if _, err := engine.Run(ctx, RunRequest{Agent: AgentSpec{Type: "stop-fail"}, Input: "hello"}); err == nil {
		t.Fatalf("run should fail on stop")
	}
}

type permissionAdapter struct {
	testAdapter
	decision PermissionDecision
}

func (a *permissionAdapter) SendPermissionResult(_ context.Context, _ string, _ string, decision PermissionDecision) error {
	a.decision = decision
	return nil
}

type eventSourceAdapter struct {
	testAdapter
	events chan SessionRuntimeEvent
}

func (a *eventSourceAdapter) SessionEvents(context.Context, string) (<-chan SessionRuntimeEvent, error) {
	return a.events, nil
}

type pendingAdapter struct {
	testAdapter
	canceled string
}

func (a *pendingAdapter) PendingRequests(context.Context, string) ([]PendingRequest, error) {
	return []PendingRequest{{ID: "p1", Kind: PendingRequestPermission}}, nil
}

func (a *pendingAdapter) CancelPendingRequest(_ context.Context, _ string, requestID string, _ string) error {
	a.canceled = requestID
	return nil
}

func TestEngineSendPermissionResult(t *testing.T) {
	ctx := context.Background()
	adapter := &permissionAdapter{}
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "permission",
		Factory: func(AgentSpec) (Adapter, error) {
			return adapter, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg)
	handle, err := engine.StartSession(ctx, SessionRequest{SessionID: "session-perm", Agent: AgentSpec{Type: "permission"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := engine.SendPermissionResult(ctx, handle.ID, "p1", PermissionDecision{Allow: true, Reason: "ok"}); err != nil {
		t.Fatalf("send permission: %v", err)
	}
	if !adapter.decision.Allow || adapter.decision.Reason != "ok" {
		t.Fatalf("unexpected decision: %+v", adapter.decision)
	}
}

func TestEnginePendingRequests(t *testing.T) {
	ctx := context.Background()
	adapter := &pendingAdapter{}
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "pending",
		Factory: func(AgentSpec) (Adapter, error) {
			return adapter, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg)
	handle, err := engine.StartSession(ctx, SessionRequest{SessionID: "pending-session", Agent: AgentSpec{Type: "pending"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	pending, err := engine.PendingRequests(ctx, handle.ID)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != "p1" {
		t.Fatalf("unexpected pending: %+v", pending)
	}
	if err := engine.CancelPendingRequest(ctx, handle.ID, "p1", "timeout"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if adapter.canceled != "p1" {
		t.Fatalf("unexpected canceled id: %s", adapter.canceled)
	}
}

func TestEngineForwardsSessionRuntimeEvents(t *testing.T) {
	ctx := context.Background()
	adapter := &eventSourceAdapter{events: make(chan SessionRuntimeEvent, 1)}
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "event-source",
		Factory: func(AgentSpec) (Adapter, error) {
			return adapter, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	received := make(chan contracts.RunEvent, 4)
	engine := NewEngine(reg, WithEventSink(EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		received <- event
		return nil
	})))
	handle, err := engine.StartSession(ctx, SessionRequest{SessionID: "session-events", Agent: AgentSpec{Type: "event-source"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	adapter.events <- SessionRuntimeEvent{SessionID: handle.ID, Type: "process.exited", Error: "exit 1"}
	close(adapter.events)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-received:
			if event.Type == contracts.EventRuntimeError {
				if event.Error != "exit 1" || event.Metadata["adapterEventType"] != "process.exited" {
					t.Fatalf("unexpected runtime event: %+v", event)
				}
				return
			}
		case <-deadline:
			t.Fatalf("runtime event not forwarded")
		}
	}
}

func TestEngineDiagnosticsFallback(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg)
	handle, err := engine.StartSession(ctx, SessionRequest{SessionID: "session-diag", Agent: AgentSpec{Type: "test"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	diag, err := engine.Diagnostics(ctx, handle.ID)
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	if diag.SessionID != handle.ID || diag.Status != SessionRunning {
		t.Fatalf("unexpected diagnostics: %+v", diag)
	}
}

func TestEngineDiagnosticsProvider(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "diag-provider",
		Factory: func(AgentSpec) (Adapter, error) {
			return diagnosticAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg)
	handle, err := engine.StartSession(ctx, SessionRequest{SessionID: "session-diag-provider", Agent: AgentSpec{Type: "diag-provider"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	diag, err := engine.Diagnostics(ctx, handle.ID)
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	if diag.Metadata["source"] != "provider" {
		t.Fatalf("provider diagnostics not used: %+v", diag)
	}
}

func TestEngineEmitChunkUsesSemanticEventTypes(t *testing.T) {
	ctx := context.Background()
	var events []contracts.RunEvent
	engine := NewEngine(nil, WithEventSink(EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		events = append(events, event)
		return nil
	})))

	chunks := []contracts.Chunk{
		{Type: contracts.ChunkToolUse, ToolID: "t1"},
		{Type: contracts.ChunkInputJSONDelta, ToolID: "t1", PartialJSON: "{}"},
		{Type: contracts.ChunkToolResult, ToolID: "t1"},
		{Type: contracts.ChunkToolResult, ToolID: "t2", IsError: true},
		{Type: contracts.ChunkQuestion},
		{Type: contracts.ChunkPermission, Permission: &contracts.PermissionRequest{ID: "p1"}},
		{Type: contracts.ChunkUsage, Usage: &contracts.TokenUsage{InputTokens: 1}},
		{Type: contracts.ChunkStatus, Plan: []contracts.PlanEntry{{Content: "plan"}}},
		{Type: contracts.ChunkArtifact, Artifact: &contracts.Artifact{Name: "file", Type: contracts.ArtifactOther}},
		{Type: contracts.ChunkHandoff, Handoff: &contracts.Handoff{Target: contracts.HandoffTarget{Type: "human"}}},
		{Type: contracts.ChunkError, Content: "boom"},
	}
	for _, chunk := range chunks {
		if err := engine.EmitChunk(ctx, "run-1", "session-1", "agent-1", chunk); err != nil {
			t.Fatalf("emit chunk: %v", err)
		}
	}
	want := []contracts.EventType{
		contracts.EventToolStarted,
		contracts.EventToolInputDelta,
		contracts.EventToolCompleted,
		contracts.EventToolFailed,
		contracts.EventQuestionAsked,
		contracts.EventPermissionAsked,
		contracts.EventUsageReported,
		contracts.EventPlanUpdated,
		contracts.EventArtifactCreated,
		contracts.EventHandoffCreated,
		contracts.EventRuntimeError,
	}
	if len(events) != len(want) {
		t.Fatalf("events len = %d, want %d: %+v", len(events), len(want), events)
	}
	for i := range want {
		if events[i].Type != want[i] {
			t.Fatalf("event types = %+v, want %v at %d", events, want[i], i)
		}
	}
	if events[8].Artifact == nil || events[9].Handoff == nil || events[10].Error != "boom" {
		t.Fatalf("semantic payloads were not attached: %+v", events)
	}
}

type detectingAdapter struct {
	testAdapter
}

func (detectingAdapter) DetectCapabilities(context.Context, AgentSpec) (Capabilities, error) {
	return Capabilities{Protocol: "test", NativeResume: true}, nil
}

type failingDetector struct {
	testAdapter
}

func (failingDetector) DetectCapabilities(context.Context, AgentSpec) (Capabilities, error) {
	return Capabilities{}, fmt.Errorf("detect failed")
}

func TestEngineDetectCapabilities(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "detecting",
		Factory: func(AgentSpec) (Adapter, error) {
			return detectingAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register detecting: %v", err)
	}
	engine := NewEngine(reg)
	capabilities, err := engine.DetectCapabilities(ctx, AgentSpec{Type: "detecting", Name: "Detector"})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if capabilities.AgentType != "detecting" || capabilities.AgentName != "Detector" || !capabilities.NativeResume {
		t.Fatalf("unexpected capabilities: %+v", capabilities)
	}
}

func TestEngineDetectCapabilitiesStaticFallback(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register test: %v", err)
	}
	engine := NewEngine(reg)
	capabilities, err := engine.DetectCapabilities(ctx, AgentSpec{Type: "test", SupportsMultimodal: true})
	if err != nil {
		t.Fatalf("detect fallback: %v", err)
	}
	if capabilities.AgentType != "test" || !capabilities.ResidentSessions || capabilities.OneShotRuns || !capabilities.Multimodal {
		t.Fatalf("unexpected fallback capabilities: %+v", capabilities)
	}
}

func TestEngineDetectCapabilitiesFailure(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "detect-fail",
		Factory: func(AgentSpec) (Adapter, error) {
			return failingDetector{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg)
	if _, err := engine.DetectCapabilities(ctx, AgentSpec{Type: "detect-fail"}); err == nil {
		t.Fatalf("detect failure should be returned")
	}
}

func TestEngineUnsupportedSessionExtensions(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	engine := NewEngine(reg)
	handle, err := engine.StartSession(ctx, SessionRequest{SessionID: "plain-session", Agent: AgentSpec{Type: "test"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := engine.SendPermissionResult(ctx, handle.ID, "p1", PermissionDecision{Allow: true}); err == nil {
		t.Fatalf("plain adapter should not accept permission result")
	}
	if _, err := engine.PendingRequests(ctx, handle.ID); err == nil {
		t.Fatalf("plain adapter should not inspect pending requests")
	}
	if err := engine.CancelPendingRequest(ctx, handle.ID, "p1", "test"); err == nil {
		t.Fatalf("plain adapter should not cancel pending requests")
	}
	for _, fn := range []func() error{
		func() error { return engine.SendPermissionResult(ctx, "missing", "p1", PermissionDecision{}) },
		func() error { _, err := engine.PendingRequests(ctx, "missing"); return err },
		func() error { return engine.CancelPendingRequest(ctx, "missing", "p1", "test") },
		func() error { _, err := engine.Diagnostics(ctx, "missing"); return err },
	} {
		if err := fn(); err == nil {
			t.Fatalf("missing session should fail")
		}
	}
}
