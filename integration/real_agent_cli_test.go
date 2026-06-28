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
	runMultimodalPrompt(t, cfg)
}

func TestRealAgentCLIAgentCoverage(t *testing.T) {
	if os.Getenv("HELIOS_INTEGRATION") != "1" {
		t.Skip("set HELIOS_INTEGRATION=1 to run real CLI integration tests")
	}
	if os.Getenv("HELIOS_RUN_AGENT_COVERAGE") != "1" {
		t.Skip("set HELIOS_RUN_AGENT_COVERAGE=1 to run the real CLI agent coverage scenarios")
	}
	apiKey := os.Getenv("HELIOS_API_KEY")
	if apiKey == "" && os.Getenv("HELIOS_ALLOW_EXISTING_AUTH") != "1" {
		t.Skip("set HELIOS_API_KEY, or HELIOS_ALLOW_EXISTING_AUTH=1 when CLIs should use existing local auth")
	}

	for _, scenario := range loadCoverageScenarios(t) {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			cfg := loadScenarioConfig(t, scenario)
			switch scenario.mode {
			case "text":
				runResidentSession(t, cfg)
			case "multimodal":
				runMultimodalPrompt(t, cfg)
			case "multimodal_fail":
				cfg.expectMultimodalFailure = true
				runMultimodalPrompt(t, cfg)
			default:
				t.Fatalf("unknown scenario mode %q", scenario.mode)
			}
		})
	}
}

func runResidentSession(t *testing.T, cfg integrationConfig) {
	t.Helper()
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
	defer stopSession(t, engine, handle.ID)

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

func runMultimodalPrompt(t *testing.T, cfg integrationConfig) {
	t.Helper()
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
	defer stopSession(t, engine, handle.ID)

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

func stopSession(t *testing.T, engine *helios.Engine, sessionID string) {
	t.Helper()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer stopCancel()
	if err := engine.StopSession(stopCtx, sessionID); err != nil {
		t.Logf("stop real agent session: %v", err)
	}
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

type coverageScenario struct {
	name     string
	agent    string
	protocol string
	model    string
	mode     string
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
		Metadata:     agentMetadata(agentType),
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

func loadCoverageScenarios(t *testing.T) []coverageScenario {
	t.Helper()
	if raw := os.Getenv("HELIOS_AGENT_COVERAGE_SCENARIOS"); raw != "" {
		scenarios := []coverageScenario{}
		for _, item := range splitCSV(raw) {
			parts := strings.Split(item, ":")
			if len(parts) != 5 {
				t.Fatalf("HELIOS_AGENT_COVERAGE_SCENARIOS item %q must be name:agent:protocol:model:mode", item)
			}
			scenarios = append(scenarios, coverageScenario{
				name:     strings.TrimSpace(parts[0]),
				agent:    strings.TrimSpace(parts[1]),
				protocol: strings.TrimSpace(parts[2]),
				model:    strings.TrimSpace(parts[3]),
				mode:     strings.TrimSpace(parts[4]),
			})
		}
		return scenarios
	}

	textModel := envDefault("HELIOS_TEXT_MODEL", os.Getenv("HELIOS_MODEL"))
	multimodalModel := os.Getenv("HELIOS_MULTIMODAL_MODEL")
	textOnlyModel := os.Getenv("HELIOS_TEXT_ONLY_MODEL")
	if textModel == "" {
		t.Fatalf("HELIOS_TEXT_MODEL or HELIOS_MODEL is required for agent coverage scenarios")
	}
	agents := []string{"hermes", "open_code", "claude_code", "open_claw"}
	scenarios := make([]coverageScenario, 0, len(agents)*3)
	for _, agent := range agents {
		protocol := defaultProtocol(agent)
		scenarios = append(scenarios, coverageScenario{
			name:  agent + "_text",
			agent: agent, protocol: protocol, model: textModel, mode: "text",
		})
		if multimodalModel != "" {
			scenarios = append(scenarios, coverageScenario{
				name:  agent + "_multimodal_supported",
				agent: agent, protocol: protocol, model: multimodalModel, mode: "multimodal",
			})
		}
		if textOnlyModel != "" {
			scenarios = append(scenarios, coverageScenario{
				name:  agent + "_multimodal_unsupported",
				agent: agent, protocol: protocol, model: textOnlyModel, mode: "multimodal_fail",
			})
		}
	}
	return scenarios
}

func defaultProtocol(agentType string) string {
	switch agentType {
	case "claude_code":
		return "anthropic"
	default:
		return "openai"
	}
}

func loadScenarioConfig(t *testing.T, scenario coverageScenario) integrationConfig {
	t.Helper()
	cliPath := envDefault(agentEnvKey(scenario.agent, "CLI"), defaultCLI(scenario.agent))
	if cliPath == "" {
		t.Fatalf("CLI path is required for scenario %s agent %q", scenario.name, scenario.agent)
	}
	if _, err := exec.LookPath(cliPath); err != nil {
		t.Fatalf("real agent CLI %q is not executable or not on PATH: %v", cliPath, err)
	}
	apiURL := protocolURL(t, scenario.protocol)
	apiKey := os.Getenv("HELIOS_API_KEY")
	if apiKey == "" && os.Getenv("HELIOS_ALLOW_EXISTING_AUTH") != "1" {
		t.Skip("set HELIOS_API_KEY, or HELIOS_ALLOW_EXISTING_AUTH=1 when CLIs should use existing local auth")
	}
	timeout := integrationTimeout(t)
	workDir := filepath.Join(t.TempDir(), "work")
	runtimeHome := filepath.Join(t.TempDir(), "runtime-home")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("create scenario workdir: %v", err)
	}
	agent := helios.AgentSpec{
		ID:           "integration-agent-" + scenario.name,
		Type:         scenario.agent,
		Name:         "Real CLI Integration " + scenario.name,
		CLIPath:      cliPath,
		DefaultModel: scenario.model,
		APIURL:       apiURL,
		APIToken:     apiKey,
		RuntimeHome:  runtimeHome,
		WorkDir:      workDir,
		Metadata:     agentMetadata(scenario.agent),
	}
	t.Logf("running coverage scenario=%s agent=%s cli=%s protocol=%s model=%s apiURL_set=%v", scenario.name, scenario.agent, cliPath, scenario.protocol, scenario.model, apiURL != "")
	return integrationConfig{
		agent:                    agent,
		workDir:                  workDir,
		runtimeHome:              runtimeHome,
		prompt:                   envDefault("HELIOS_PROMPT", "Reply with exactly: helios-ok"),
		expectContains:           envDefault("HELIOS_EXPECT_CONTAINS", "helios-ok"),
		multimodalPrompt:         envDefault("HELIOS_MULTIMODAL_PROMPT", "The attached image is a single-color square. Reply with exactly one lowercase English word for its color."),
		multimodalExpectContains: envDefault("HELIOS_MULTIMODAL_EXPECT_CONTAINS", "red"),
		timeout:                  timeout,
		expectMultimodalFailure:  scenario.mode == "multimodal_fail",
	}
}

func protocolURL(t *testing.T, protocol string) string {
	t.Helper()
	switch protocol {
	case "openai":
		return envDefault("HELIOS_OPENAI_API_URL", os.Getenv("HELIOS_API_URL"))
	case "anthropic":
		return envDefault("HELIOS_ANTHROPIC_API_URL", os.Getenv("HELIOS_API_URL"))
	default:
		t.Fatalf("unknown protocol %q", protocol)
		return ""
	}
}

func integrationTimeout(t *testing.T) time.Duration {
	t.Helper()
	timeout := 2 * time.Minute
	if raw := os.Getenv("HELIOS_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			t.Fatalf("HELIOS_TIMEOUT_SECONDS must be a positive integer, got %q", raw)
		}
		timeout = time.Duration(seconds) * time.Second
	}
	return timeout
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

func agentMetadata(agentType string) map[string]any {
	if agentType != "open_claw" {
		return nil
	}
	metadata := map[string]any{}
	if value := os.Getenv("HELIOS_OPEN_CLAW_GATEWAY_URL"); value != "" {
		metadata["gatewayURL"] = value
	}
	if value := os.Getenv("HELIOS_OPEN_CLAW_GATEWAY_PORT"); value != "" {
		metadata["gatewayPort"] = value
	}
	if value := os.Getenv("HELIOS_OPEN_CLAW_GATEWAY_TOKEN"); value != "" {
		metadata["gatewayToken"] = value
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func agentEnvKey(agentType string, suffix string) string {
	return "HELIOS_" + strings.ToUpper(agentType) + "_" + suffix
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
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
