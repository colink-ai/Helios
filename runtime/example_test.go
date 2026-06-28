package runtime_test

import (
	"context"
	"fmt"
	"os"

	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

func ExampleFileArtifactStore() {
	dir, _ := os.MkdirTemp("", "helios-artifacts-*")
	defer os.RemoveAll(dir)
	store := helios.NewFileArtifactStore(dir)
	artifact, _ := store.SaveArtifact(context.Background(), contracts.Artifact{
		SessionID: "session-1",
		Type:      contracts.ArtifactDocument,
		Name:      "summary.txt",
		Content:   "hello",
	})
	fmt.Println(artifact.Name)
	// Output: summary.txt
}

func ExamplePermissionDecision() {
	decision := helios.PermissionDecision{Allow: true, Reason: "approved by policy"}
	fmt.Println(decision.Allow)
	// Output: true
}

func ExampleSessionRequestFromSnapshot() {
	req, _ := helios.SessionRequestFromSnapshot(helios.SessionSnapshot{
		SessionID:      "session-1",
		AgentType:      "hermes",
		AgentSessionID: "agent-session-1",
	}, helios.AgentSpec{})
	fmt.Println(req.ResumeSessionID)
	// Output: agent-session-1
}

func ExampleTeamRunner() {
	runner := helios.NewTeamRunner(nil)
	_ = runner
	fmt.Println("team runner")
	// Output: team runner
}
