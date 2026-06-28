package main

import (
	"context"

	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

type permissionSender interface {
	SendPermissionResult(ctx context.Context, sessionID string, permissionID string, decision helios.PermissionDecision) error
}

func main() {
	var engine *helios.Engine
	_ = handlePermission
	_ = engine
}

func handlePermission(ctx context.Context, sender permissionSender, event contracts.RunEvent) error {
	if event.Type != contracts.EventPermissionAsked || event.Chunk == nil || event.Chunk.Permission == nil {
		return nil
	}
	return sender.SendPermissionResult(ctx, event.SessionID, event.Chunk.Permission.ID, helios.PermissionDecision{
		Allow:  true,
		Reason: "approved by host policy",
	})
}
