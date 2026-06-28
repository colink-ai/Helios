package runtime

import (
	"context"
	"testing"
)

func TestMemorySessionStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemorySessionStore()
	if got, err := store.LoadSession(ctx, "missing"); err != nil || got != nil {
		t.Fatalf("missing session = %+v, %v", got, err)
	}
	snapshot := SessionSnapshot{SessionID: "s1", AgentType: "test", Status: SessionRunning}
	if err := store.SaveSession(ctx, snapshot); err != nil {
		t.Fatalf("save session: %v", err)
	}
	got, err := store.LoadSession(ctx, "s1")
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if got == nil || got.AgentType != "test" || got.UpdatedAt.IsZero() {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
	if err := store.DeleteSession(ctx, "s1"); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	got, err = store.LoadSession(ctx, "s1")
	if err != nil || got != nil {
		t.Fatalf("deleted session = %+v, %v", got, err)
	}
}
