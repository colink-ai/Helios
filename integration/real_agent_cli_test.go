//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/colink-ai/helios/adapters/all"
	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

func TestRealAgentCLIResidentSession(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	registry := helios.NewRegistry()
	if err := all.Register(registry); err != nil {
		t.Fatalf("register adapters: %v", err)
	}

	var chunks []contracts.Chunk
	engine := helios.NewEngine(registry, helios.WithEventSink(helios.EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		if event.Chunk != nil {
			chunks = append(chunks, *event.Chunk)
		}
		return nil
	})))
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	caps, err := engine.DetectCapabilities(ctx, cfg.agent)
	if err != nil {
		t.Fatalf("detect real agent capabilities: %v", err)
	}
	t.Logf("detected real agent type=%s protocol=%s resident=%v oneshot=%v resume=%v", caps.AgentType, caps.Protocol, caps.ResidentSessions, caps.OneShotRuns, caps.NativeResume)

	handle, err := engine.StartSession(ctx, helios.SessionRequest{
		SessionID:   helios.NewID("integration"),
		Agent:       cfg.agent,
		WorkDir:     cfg.workDir,
		RuntimeHome: cfg.runtimeHome,
	})
	if err != nil {
		t.Fatalf("start real agent session: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		if err := engine.StopSession(stopCtx, handle.ID); err != nil {
			t.Logf("stop real agent session: %v", err)
		}
	}()

	result, err := engine.Prompt(ctx, helios.PromptRequest{
		SessionID: handle.ID,
		Input:     cfg.prompt,
	})
	if err != nil {
		t.Fatalf("prompt real agent session: %v", err)
	}
	output := ""
	if result != nil {
		output = result.Output
	}
	if strings.TrimSpace(output) == "" {
		output = textFromChunks(chunks)
	}
	assertOutput(t, output, cfg.expectContains)
}

func TestRealAgentCLIOneShotRun(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	if !cfg.runOneShot {
		t.Skip("set HELIOS_RUN_ONESHOT=1 to validate Engine.Run against the real CLI")
	}
	registry := helios.NewRegistry()
	if err := all.Register(registry); err != nil {
		t.Fatalf("register adapters: %v", err)
	}
	engine := helios.NewEngine(registry)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	result, err := engine.Run(ctx, helios.RunRequest{
		Agent:       cfg.agent,
		Input:       cfg.prompt,
		WorkDir:     cfg.workDir,
		RuntimeHome: cfg.runtimeHome,
	})
	if err != nil {
		t.Fatalf("run real agent one-shot: %v", err)
	}
	if result == nil {
		t.Fatalf("one-shot result is nil")
	}
	assertOutput(t, result.Output, cfg.expectContains)
}

func TestRealAgentCLIMultimodalPrompt(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	if !cfg.runMultimodal {
		t.Skip("set HELIOS_RUN_MULTIMODAL=1 to validate image input against the real CLI")
	}
	registry := helios.NewRegistry()
	if err := all.Register(registry); err != nil {
		t.Fatalf("register adapters: %v", err)
	}
	var chunks []contracts.Chunk
	engine := helios.NewEngine(registry, helios.WithEventSink(helios.EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		if event.Chunk != nil {
			chunks = append(chunks, *event.Chunk)
		}
		return nil
	})))
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	handle, err := engine.StartSession(ctx, helios.SessionRequest{
		SessionID:   helios.NewID("integration-mm"),
		Agent:       cfg.agent,
		WorkDir:     cfg.workDir,
		RuntimeHome: cfg.runtimeHome,
	})
	if err != nil {
		t.Fatalf("start real multimodal session: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		if err := engine.StopSession(stopCtx, handle.ID); err != nil {
			t.Logf("stop real multimodal session: %v", err)
		}
	}()

	result, err := engine.Prompt(ctx, helios.PromptRequest{
		SessionID: handle.ID,
		Input:     cfg.multimodalPrompt,
		Images: []contracts.ImageContent{{
			MimeType: "image/png",
			Data:     redPixelPNGBase64,
		}},
	})
	if cfg.expectMultimodalFailure {
		if err != nil {
			t.Logf("multimodal prompt failed as expected: %v", err)
			return
		}
		output := ""
		if result != nil {
			output = result.Output
		}
		if strings.TrimSpace(output) == "" {
			output = textFromChunks(chunks)
		}
		if cfg.multimodalExpectContains != "" && strings.Contains(strings.ToLower(output), strings.ToLower(cfg.multimodalExpectContains)) {
			t.Fatalf("expected multimodal prompt not to satisfy visual expectation %q, got output %q", cfg.multimodalExpectContains, output)
		}
		t.Logf("multimodal prompt did not satisfy visual expectation as expected; output length=%d chunk summary=%s", len(output), chunkSummary(chunks))
		return
	}
	if err != nil {
		t.Fatalf("prompt real multimodal session: %v", err)
	}
	if result == nil {
		t.Fatalf("multimodal result is nil")
	}
	output := result.Output
	if strings.TrimSpace(output) == "" {
		output = textFromChunks(chunks)
	}
	t.Logf("multimodal chunk summary: %s", chunkSummary(chunks))
	assertOutput(t, output, cfg.multimodalExpectContains)
}

type integrationConfig struct {
	agent                    helios.AgentSpec
	workDir                  string
	runtimeHome              string
	prompt                   string
	expectContains           string
	multimodalPrompt         string
	multimodalExpectContains string
	timeout                  time.Duration
	runOneShot               bool
	runMultimodal            bool
	expectMultimodalFailure  bool
}

func loadIntegrationConfig(t *testing.T) integrationConfig {
	t.Helper()
	if os.Getenv("HELIOS_INTEGRATION") != "1" {
		t.Skip("set HELIOS_INTEGRATION=1 to run real CLI integration tests")
	}

	agentType := envDefault("HELIOS_AGENT_TYPE", "open_code")
	cliPath := envDefault("HELIOS_AGENT_CLI", defaultCLI(agentType))
	if cliPath == "" {
		t.Fatalf("HELIOS_AGENT_CLI is required for agent type %q", agentType)
	}
	if _, err := exec.LookPath(cliPath); err != nil {
		t.Fatalf("real agent CLI %q is not executable or not on PATH: %v", cliPath, err)
	}

	apiKey := os.Getenv("HELIOS_API_KEY")
	if apiKey == "" && os.Getenv("HELIOS_ALLOW_EXISTING_AUTH") != "1" {
		t.Skip("set HELIOS_API_KEY, or HELIOS_ALLOW_EXISTING_AUTH=1 when the CLI should use existing local auth")
	}

	workDir := os.Getenv("HELIOS_WORKDIR")
	if workDir == "" {
		workDir = t.TempDir()
	}
	runtimeHome := os.Getenv("HELIOS_RUNTIME_HOME")
	if runtimeHome == "" {
		runtimeHome = filepath.Join(t.TempDir(), "runtime-home")
	}

	timeout := 2 * time.Minute
	if raw := os.Getenv("HELIOS_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			t.Fatalf("HELIOS_TIMEOUT_SECONDS must be a positive integer, got %q", raw)
		}
		timeout = time.Duration(seconds) * time.Second
	}

	prompt := envDefault("HELIOS_PROMPT", "Reply with exactly: helios-ok")
	expect := envDefault("HELIOS_EXPECT_CONTAINS", "helios-ok")
	multimodalPrompt := envDefault("HELIOS_MULTIMODAL_PROMPT", "The attached image is a single-color square. Reply with exactly one lowercase English word for its color.")
	multimodalExpect := envDefault("HELIOS_MULTIMODAL_EXPECT_CONTAINS", "red")
	agent := helios.AgentSpec{
		ID:           "integration-agent",
		Type:         agentType,
		Name:         "Real CLI Integration Agent",
		CLIPath:      cliPath,
		DefaultModel: os.Getenv("HELIOS_MODEL"),
		APIURL:       os.Getenv("HELIOS_API_URL"),
		APIToken:     apiKey,
		RuntimeHome:  runtimeHome,
		WorkDir:      workDir,
	}
	t.Logf("running real CLI integration agent=%s cli=%s model=%s apiURL_set=%v workDir=%s runtimeHome=%s", agent.Type, agent.CLIPath, agent.DefaultModel, agent.APIURL != "", workDir, runtimeHome)
	return integrationConfig{
		agent:                    agent,
		workDir:                  workDir,
		runtimeHome:              runtimeHome,
		prompt:                   prompt,
		expectContains:           expect,
		multimodalPrompt:         multimodalPrompt,
		multimodalExpectContains: multimodalExpect,
		timeout:                  timeout,
		runOneShot:               os.Getenv("HELIOS_RUN_ONESHOT") == "1",
		runMultimodal:            os.Getenv("HELIOS_RUN_MULTIMODAL") == "1",
		expectMultimodalFailure:  os.Getenv("HELIOS_EXPECT_MULTIMODAL_FAILURE") == "1",
	}
}

func defaultCLI(agentType string) string {
	switch agentType {
	case "hermes":
		return "hermes"
	case "open_code":
		return "opencode"
	case "claude_code":
		return "claude-agent-acp"
	case "open_claw":
		return "openclaw"
	default:
		return ""
	}
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func assertOutput(t *testing.T, output string, expect string) {
	t.Helper()
	output = strings.TrimSpace(output)
	if output == "" {
		t.Fatalf("real agent output is empty")
	}
	if expect != "" && !strings.Contains(output, expect) {
		t.Fatalf("real agent output %q does not contain expected text %q", output, expect)
	}
}

func textFromChunks(chunks []contracts.Chunk) string {
	parts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk.Type == contracts.ChunkText && chunk.Content != "" {
			parts = append(parts, chunk.Content)
		}
	}
	return fmt.Sprint(strings.Join(parts, ""))
}

func chunkSummary(chunks []contracts.Chunk) string {
	parts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		parts = append(parts, fmt.Sprintf("%s:%d", chunk.Type, len(chunk.Content)))
	}
	return strings.Join(parts, ",")
}

const redPixelPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAIAAACQkWg2AAAAF0lEQVR4nGP4z8BAEiJN9aiGUQ1DSgMAkPn/Afnh+ngAAAAASUVORK5CYII="
