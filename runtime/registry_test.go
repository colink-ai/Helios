package runtime

import (
	"context"
	"testing"
)

type testAdapter struct{}

func (testAdapter) StartSession(context.Context, SessionRequest) (*SessionHandle, error) {
	return &SessionHandle{ID: "session-1", Status: SessionRunning}, nil
}

func (testAdapter) Prompt(context.Context, PromptRequest, ChunkHandler) (*RunResult, error) {
	return &RunResult{Output: "ok"}, nil
}

func (testAdapter) StopSession(context.Context, string) error { return nil }

func (testAdapter) GetSessionStatus(context.Context, string) (SessionStatus, error) {
	return SessionRunning, nil
}

func (testAdapter) CheckHealth(context.Context, AgentSpec) error { return nil }

func TestRegistryCreateAndList(t *testing.T) {
	reg := NewRegistry()
	err := reg.Register(AdapterMeta{
		Type:        "test",
		Name:        "Test",
		Description: "test adapter",
		DefaultPath: "test-cli",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	})
	if err != nil {
		t.Fatalf("register adapter: %v", err)
	}
	if err := reg.Register(AdapterMeta{Type: "test", Factory: func(AgentSpec) (Adapter, error) {
		return testAdapter{}, nil
	}}); err == nil {
		t.Fatalf("duplicate register should fail")
	}
	adapter, err := reg.Create(AgentSpec{Type: "test"})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}
	if adapter == nil {
		t.Fatalf("adapter is nil")
	}
	types := reg.Types()
	if len(types) != 1 || types[0].Factory != nil || types[0].Type != "test" {
		t.Fatalf("unexpected types: %+v", types)
	}
}

func TestRegistryValidationAndSorting(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{}); err == nil {
		t.Fatalf("missing type should fail")
	}
	if err := reg.Register(AdapterMeta{Type: "bad"}); err == nil {
		t.Fatalf("missing factory should fail")
	}
	for _, typ := range []string{"zeta", "alpha"} {
		if err := reg.Register(AdapterMeta{Type: typ, Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		}}); err != nil {
			t.Fatalf("register %s: %v", typ, err)
		}
	}
	types := reg.Types()
	if len(types) != 2 || types[0].Type != "alpha" || types[1].Type != "zeta" {
		t.Fatalf("types not sorted: %+v", types)
	}
}
