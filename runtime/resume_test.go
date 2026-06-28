package runtime

import (
	"context"
	"testing"
)

func TestSessionRequestFromSnapshot(t *testing.T) {
	req, err := SessionRequestFromSnapshot(SessionSnapshot{
		SessionID:      "session-1",
		RunID:          "run-1",
		AgentID:        "agent-1",
		AgentType:      "test",
		AgentSessionID: "agent-session-1",
		Metadata:       map[string]any{"k": "v"},
	}, AgentSpec{})
	if err != nil {
		t.Fatalf("request from snapshot: %v", err)
	}
	if req.SessionID != "session-1" || req.ResumeSessionID != "agent-session-1" || req.Agent.Type != "test" || req.Agent.ID != "agent-1" {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestEngineResumeSessionFromSnapshot(t *testing.T) {
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
	handle, err := engine.ResumeSessionFromSnapshot(context.Background(), SessionSnapshot{
		SessionID:      "session-resume",
		AgentType:      "test",
		AgentSessionID: "agent-session-resume",
	}, AgentSpec{})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if handle.ID != "session-1" {
		t.Fatalf("unexpected handle: %+v", handle)
	}
}

func TestSessionRequestFromSnapshotRequiresAgentSessionID(t *testing.T) {
	if _, err := SessionRequestFromSnapshot(SessionSnapshot{SessionID: "s1"}, AgentSpec{Type: "test"}); err == nil {
		t.Fatalf("missing agent session id should fail")
	}
}
