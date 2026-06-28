package runtime

import (
	"context"
	"testing"

	"github.com/colink-ai/helios/contracts"
)

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

type permissionAdapter struct {
	testAdapter
	decision PermissionDecision
}

func (a *permissionAdapter) SendPermissionResult(_ context.Context, _ string, _ string, decision PermissionDecision) error {
	a.decision = decision
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
