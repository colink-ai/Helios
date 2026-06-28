package main

import (
	"context"
	"testing"

	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

type fakePermissionSender struct {
	sessionID    string
	permissionID string
	decision     helios.PermissionDecision
	calls        int
}

func (s *fakePermissionSender) SendPermissionResult(_ context.Context, sessionID string, permissionID string, decision helios.PermissionDecision) error {
	s.sessionID = sessionID
	s.permissionID = permissionID
	s.decision = decision
	s.calls++
	return nil
}

func TestMainFunction(t *testing.T) {
	main()
}

func TestHandlePermission(t *testing.T) {
	sender := &fakePermissionSender{}
	err := handlePermission(context.Background(), sender, contracts.RunEvent{
		Type:      contracts.EventPermissionAsked,
		SessionID: "session-1",
		Chunk: &contracts.Chunk{Permission: &contracts.PermissionRequest{
			ID: "permission-1",
		}},
	})
	if err != nil {
		t.Fatalf("handle permission: %v", err)
	}
	if sender.calls != 1 || sender.sessionID != "session-1" || sender.permissionID != "permission-1" {
		t.Fatalf("unexpected sender state: %+v", sender)
	}
	if !sender.decision.Allow || sender.decision.Reason == "" {
		t.Fatalf("unexpected decision: %+v", sender.decision)
	}
}

func TestHandlePermissionIgnoresNonPermissionEvents(t *testing.T) {
	sender := &fakePermissionSender{}
	if err := handlePermission(context.Background(), sender, contracts.RunEvent{Type: contracts.EventChunk}); err != nil {
		t.Fatalf("ignore event: %v", err)
	}
	if sender.calls != 0 {
		t.Fatalf("sender should not be called")
	}
}
