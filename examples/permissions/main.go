package main

import (
	"context"

	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

func main() {
	var engine *helios.Engine
	handlePermission := func(ctx context.Context, event contracts.RunEvent) error {
		if event.Type != contracts.EventPermissionAsked || event.Chunk == nil || event.Chunk.Permission == nil {
			return nil
		}
		return engine.SendPermissionResult(ctx, event.SessionID, event.Chunk.Permission.ID, helios.PermissionDecision{
			Allow:  true,
			Reason: "approved by host policy",
		})
	}
	_ = handlePermission
}
