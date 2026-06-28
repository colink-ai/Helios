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
