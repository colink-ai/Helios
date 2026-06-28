package main

import (
	"context"

	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

func main() {
	store := helios.NewFileArtifactStore("./runtime-artifacts")
	_, _ = store.SaveArtifact(context.Background(), contracts.Artifact{
		SessionID: "session-1",
		Type:      contracts.ArtifactDocument,
		Name:      "summary.txt",
		Content:   "hello from Helios",
	})
}
