package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

func TestFakeACPAgentCLI(t *testing.T) {
	if os.Getenv("HELIOS_FAKE_ACP") != "1" {
		t.Skip("helper process only")
	}
	runFakeACPAgent()
	os.Exit(0)
}

func TestBaseAdapterResidentSessionE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID: "host-session",
		Agent:     helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
		MCPServers: []helios.MCPServerSpec{{
			Name: "knowledge",
			Type: "http",
			URL:  "http://127.0.0.1:9000/mcp",
		}},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if handle.AgentSessionID != "fake-session-new" {
		t.Fatalf("agent session id = %q", handle.AgentSessionID)
	}

	var chunks []contracts.Chunk
	result, err := adapter.Prompt(ctx, helios.PromptRequest{
		SessionID: handle.ID,
		Input:     "hello",
	}, func(chunk contracts.Chunk) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if result.Output != "hello from fake" {
		t.Fatalf("output = %q", result.Output)
	}
	assertChunkTypes(t, chunks, contracts.ChunkThinking, contracts.ChunkText, contracts.ChunkToolUse, contracts.ChunkToolResult, contracts.ChunkUsage, contracts.ChunkStatus)

	usedResume, err := adapter.UsedNativeResume(ctx, handle.ID)
	if err != nil {
		t.Fatalf("used resume: %v", err)
	}
	if usedResume {
		t.Fatalf("new session should not use native resume")
	}
	if err := adapter.StopSession(ctx, handle.ID); err != nil {
		t.Fatalf("stop session: %v", err)
	}
}

func TestBaseAdapterResumeSessionE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID:       "host-session-resume",
		ResumeSessionID: "agent-existing",
		Agent:           helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
	})
	if err != nil {
		t.Fatalf("start resume session: %v", err)
	}
	if handle.AgentSessionID != "agent-existing" {
		t.Fatalf("agent session id = %q", handle.AgentSessionID)
	}
	usedResume, err := adapter.UsedNativeResume(ctx, handle.ID)
	if err != nil {
		t.Fatalf("used resume: %v", err)
	}
	if !usedResume {
		t.Fatalf("resume session should use native resume")
	}
	_ = adapter.StopSession(ctx, handle.ID)
}

func TestBaseAdapterDiagnosticsE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID: "host-session-diag",
		Agent:     helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer adapter.StopSession(context.Background(), handle.ID)
	diag, err := adapter.Diagnostics(ctx, handle.ID)
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	if diag.SessionID != handle.ID || diag.AgentSessionID != "fake-session-new" || diag.Status != helios.SessionRunning {
		t.Fatalf("unexpected diagnostics: %+v", diag)
	}
}

func TestBaseAdapterResumeFallsBackToLoadE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID:       "host-session-load",
		ResumeSessionID: "resume-fails",
		Agent:           helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
	})
	if err != nil {
		t.Fatalf("start load fallback session: %v", err)
	}
	if handle.AgentSessionID != "loaded-session" {
		t.Fatalf("agent session id = %q", handle.AgentSessionID)
	}
	if handle.Metadata["resumeStrategy"] != "load" || handle.Metadata["nativeResume"] != true {
		t.Fatalf("unexpected metadata: %+v", handle.Metadata)
	}
	result, err := adapter.Prompt(ctx, helios.PromptRequest{SessionID: handle.ID, Input: "hello"}, nil)
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if result.Output != "hello from fake" {
		t.Fatalf("output = %q", result.Output)
	}
	_ = adapter.StopSession(ctx, handle.ID)
}

func TestBaseAdapterResumeFallsBackToNewE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID:       "host-session-new-fallback",
		ResumeSessionID: "all-resume-fails",
		Agent:           helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
	})
	if err != nil {
		t.Fatalf("start new fallback session: %v", err)
	}
	if handle.AgentSessionID != "fake-session-new" {
		t.Fatalf("agent session id = %q", handle.AgentSessionID)
	}
	if handle.Metadata["resumeStrategy"] != "new" || handle.Metadata["nativeResume"] != false {
		t.Fatalf("unexpected metadata: %+v", handle.Metadata)
	}
	_ = adapter.StopSession(ctx, handle.ID)
}

func TestBaseAdapterRunE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var chunks []contracts.Chunk
	result, err := adapter.Run(ctx, helios.RunRequest{
		RunID: "run-1",
		Agent: helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
		Input: "run",
	}, func(chunk contracts.Chunk) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.RunID != "run-1" || result.Output != "hello from fake" {
		t.Fatalf("unexpected result: %+v", result)
	}
	assertChunkTypes(t, chunks, contracts.ChunkThinking, contracts.ChunkText, contracts.ChunkToolUse, contracts.ChunkToolResult, contracts.ChunkUsage, contracts.ChunkStatus)
}

func TestBaseAdapterElicitationE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID: "host-session-elicit",
		Agent:     helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer adapter.StopSession(context.Background(), handle.ID)

	questionCh := make(chan contracts.Chunk, 1)
	resultCh := make(chan *helios.RunResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := adapter.Prompt(ctx, helios.PromptRequest{SessionID: handle.ID, Input: "please ask"}, func(chunk contracts.Chunk) {
			if chunk.Type == contracts.ChunkQuestion {
				questionCh <- chunk
			}
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	var question contracts.Chunk
	select {
	case question = <-questionCh:
	case err := <-errCh:
		t.Fatalf("prompt failed before question: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for question")
	}
	if question.ToolID != "tool-question" || len(question.Questions) != 1 || question.Questions[0].Options[0].Label != "A" {
		t.Fatalf("unexpected question: %+v", question)
	}
	if err := adapter.SendToolResult(ctx, handle.ID, question.ToolID, `{"question_0":"A"}`); err != nil {
		t.Fatalf("send tool result: %v", err)
	}

	select {
	case result := <-resultCh:
		if result.Output != "answer accepted" {
			t.Fatalf("output = %q", result.Output)
		}
	case err := <-errCh:
		t.Fatalf("prompt failed: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for prompt result")
	}
}

func TestBaseAdapterPermissionResultE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID: "host-session-permission",
		Agent:     helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer adapter.StopSession(context.Background(), handle.ID)

	permissionCh := make(chan contracts.Chunk, 1)
	resultCh := make(chan *helios.RunResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := adapter.Prompt(ctx, helios.PromptRequest{SessionID: handle.ID, Input: "please approve"}, func(chunk contracts.Chunk) {
			if chunk.Type == contracts.ChunkPermission {
				permissionCh <- chunk
			}
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	var permission contracts.Chunk
	select {
	case permission = <-permissionCh:
	case err := <-errCh:
		t.Fatalf("prompt failed before permission: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for permission")
	}
	if permission.Permission == nil || permission.Permission.Action != "shell" {
		t.Fatalf("unexpected permission: %+v", permission)
	}
	if err := adapter.SendPermissionResult(ctx, handle.ID, permission.ToolID, helios.PermissionDecision{Allow: true, Reason: "test"}); err != nil {
		t.Fatalf("send permission result: %v", err)
	}

	select {
	case result := <-resultCh:
		if result.Output != "permission accepted" {
			t.Fatalf("output = %q", result.Output)
		}
	case err := <-errCh:
		t.Fatalf("prompt failed: %v", err)
	case <-ctx.Done():
		t.Fatalf("timed out waiting for prompt result")
	}
}

func TestBaseAdapterDetectCapabilitiesE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	capabilities, err := adapter.DetectCapabilities(ctx, helios.AgentSpec{
		Type:               "fake",
		Name:               "Fake ACP",
		CLIPath:            os.Args[0],
		SupportsMultimodal: true,
	})
	if err != nil {
		t.Fatalf("detect capabilities: %v", err)
	}
	if capabilities.Protocol != "acp" || capabilities.AgentType != "fake" || capabilities.AgentName != "Fake ACP" {
		t.Fatalf("unexpected identity: %+v", capabilities)
	}
	if !capabilities.NativeResume || !capabilities.SessionLoad || !capabilities.Usage || !capabilities.Plans || !capabilities.Artifacts || !capabilities.Questions || !capabilities.Multimodal {
		t.Fatalf("unexpected capabilities: %+v", capabilities)
	}
	if capabilities.Metadata["protocolVersion"] != 1 {
		t.Fatalf("unexpected metadata: %+v", capabilities.Metadata)
	}
}

func TestBaseAdapterCheckHealthE2E(t *testing.T) {
	adapter := newFakeAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := adapter.CheckHealth(ctx, helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]}); err != nil {
		t.Fatalf("check health: %v", err)
	}
}

func TestBaseAdapterConfigureModelE2E(t *testing.T) {
	adapter := NewBaseAdapter(Config{
		CLIPath:              os.Args[0],
		StartupTimeout:       2 * time.Second,
		PromptTimeout:        2 * time.Second,
		ConfigureModelViaACP: true,
		ModelRef: func(req helios.SessionRequest) string {
			if req.Agent.DefaultModel != "" {
				return req.Agent.DefaultModel + "-from-ref"
			}
			return ""
		},
		BuildArgs: func(helios.SessionRequest) []string {
			return []string{"-test.run=TestFakeACPAgentCLI", "--"}
		},
		BuildEnv: func(helios.SessionRequest) []string {
			return []string{"HELIOS_FAKE_ACP=1"}
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID: "host-session-model",
		Agent:     helios.AgentSpec{Type: "fake", CLIPath: os.Args[0], DefaultModel: "qwen-test"},
	})
	if err != nil {
		t.Fatalf("start session with model config: %v", err)
	}
	if handle.AgentSessionID != "fake-session-new" {
		t.Fatalf("agent session id = %q", handle.AgentSessionID)
	}
	_ = adapter.StopSession(ctx, handle.ID)
}

func TestBaseAdapterStartSessionValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	missingCLI := NewBaseAdapter(Config{})
	if _, err := missingCLI.StartSession(ctx, helios.SessionRequest{SessionID: "missing-cli"}); err == nil {
		t.Fatalf("missing cli path should fail")
	}

	adapter := newFakeAdapter()
	handle, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID: "duplicate-session",
		Agent:     helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	defer adapter.StopSession(context.Background(), handle.ID)
	if _, err := adapter.StartSession(ctx, helios.SessionRequest{
		SessionID: "duplicate-session",
		Agent:     helios.AgentSpec{Type: "fake", CLIPath: os.Args[0]},
	}); err == nil {
		t.Fatalf("duplicate session should fail")
	}
}

func TestBaseAdapterConfigHelpers(t *testing.T) {
	adapter := NewBaseAdapter(Config{
		CLIPath:        "configured-cli",
		StartupTimeout: time.Second,
		PromptTimeout:  2 * time.Second,
		BuildArgs: func(req helios.SessionRequest) []string {
			return []string{"--session", req.SessionID}
		},
		BuildEnv: func(helios.SessionRequest) []string {
			return []string{"EXTRA_ENV=1"}
		},
	})
	req := helios.SessionRequest{
		SessionID: "session-1",
		Agent: helios.AgentSpec{
			CLIPath: "spec-cli",
			Env:     map[string]string{"AGENT_ENV": "2"},
		},
	}
	if got := adapter.cliPath(req.Agent); got != "configured-cli" {
		t.Fatalf("cliPath = %q", got)
	}
	if got := strings.Join(adapter.buildArgs(req), " "); got != "--session session-1" {
		t.Fatalf("buildArgs = %q", got)
	}
	env := strings.Join(adapter.buildEnv(req), "\n")
	if !strings.Contains(env, "AGENT_ENV=2") || !strings.Contains(env, "EXTRA_ENV=1") {
		t.Fatalf("buildEnv missing entries: %s", env)
	}
	if adapter.startupTimeout() != time.Second || adapter.promptTimeout() != 2*time.Second {
		t.Fatalf("custom timeouts not used")
	}

	defaults := NewBaseAdapter(Config{})
	if got := defaults.cliPath(helios.AgentSpec{CLIPath: "spec-cli"}); got != "spec-cli" {
		t.Fatalf("default cliPath = %q", got)
	}
	if args := defaults.buildArgs(helios.SessionRequest{}); args != nil {
		t.Fatalf("default args = %+v", args)
	}
	if defaults.startupTimeout() != defaultStartupTimeout || defaults.promptTimeout() != defaultPromptTimeout {
		t.Fatalf("default timeouts not used")
	}
}

func newFakeAdapter() *BaseAdapter {
	return NewBaseAdapter(Config{
		CLIPath:        os.Args[0],
		StartupTimeout: 2 * time.Second,
		PromptTimeout:  2 * time.Second,
		BuildArgs: func(helios.SessionRequest) []string {
			return []string{"-test.run=TestFakeACPAgentCLI", "--"}
		},
		BuildEnv: func(helios.SessionRequest) []string {
			return []string{"HELIOS_FAKE_ACP=1"}
		},
	})
}

func assertChunkTypes(t *testing.T, chunks []contracts.Chunk, want ...contracts.ChunkType) {
	t.Helper()
	got := make([]contracts.ChunkType, 0, len(chunks))
	for _, chunk := range chunks {
		got = append(got, chunk.Type)
	}
	if len(got) != len(want) {
		t.Fatalf("chunk types = %v, want %v; chunks=%+v", got, want, chunks)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chunk types = %v, want %v; chunks=%+v", got, want, chunks)
		}
	}
}

func runFakeACPAgent() {
	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	var pendingPromptID any
	waitingForElicitation := false
	waitingForPermission := false
	for scanner.Scan() {
		var req struct {
			ID     any             `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		if req.Method == "" && waitingForElicitation {
			waitingForElicitation = false
			emitFakeUpdate(writer, "agent_message_chunk", map[string]any{"content": map[string]any{"type": "text", "text": "answer accepted"}})
			writeFakeResult(writer, pendingPromptID, map[string]any{"stopReason": "end_turn"})
			pendingPromptID = nil
			continue
		}
		if req.Method == "" && waitingForPermission {
			waitingForPermission = false
			emitFakeUpdate(writer, "agent_message_chunk", map[string]any{"content": map[string]any{"type": "text", "text": "permission accepted"}})
			writeFakeResult(writer, pendingPromptID, map[string]any{"stopReason": "end_turn"})
			pendingPromptID = nil
			continue
		}
		switch req.Method {
		case "initialize":
			writeFakeResult(writer, req.ID, map[string]any{
				"protocolVersion": 1,
				"agentCapabilities": map[string]any{
					"sessionResume": true,
					"sessionLoad":   true,
					"features": map[string]any{
						"artifacts":   true,
						"elicitation": true,
						"mcpServers":  true,
						"permissions": true,
						"plan":        true,
						"tokenUsage":  true,
						"usage":       true,
						"vision":      true,
					},
				},
			})
		case "session/new":
			var params SessionParams
			_ = json.Unmarshal(req.Params, &params)
			writeFakeResult(writer, req.ID, map[string]any{"sessionId": "fake-session-new"})
		case "session/resume":
			var params SessionParams
			_ = json.Unmarshal(req.Params, &params)
			if params.SessionID == "resume-fails" || params.SessionID == "all-resume-fails" {
				writeFakeError(writer, req.ID, -32000, "resume failed")
				continue
			}
			writeFakeResult(writer, req.ID, map[string]any{"sessionId": params.SessionID})
		case "session/load":
			var params SessionParams
			_ = json.Unmarshal(req.Params, &params)
			emitFakeUpdate(writer, "agent_message_chunk", map[string]any{"content": map[string]any{"type": "text", "text": "history should be filtered"}})
			if params.SessionID == "all-resume-fails" {
				writeFakeError(writer, req.ID, -32000, "load failed")
				continue
			}
			writeFakeResult(writer, req.ID, map[string]any{"sessionId": "loaded-session"})
		case "session/set_config_option":
			writeFakeResult(writer, req.ID, map[string]any{"ok": true})
		case "session/prompt":
			var params PromptParams
			_ = json.Unmarshal(req.Params, &params)
			if fakePromptText(params) == "please ask" {
				pendingPromptID = req.ID
				waitingForElicitation = true
				writeFake(writer, map[string]any{
					"jsonrpc": "2.0",
					"id":      "elicit-1",
					"method":  "elicitation/create",
					"params": map[string]any{
						"mode":       "form",
						"sessionId":  params.SessionID,
						"toolCallId": "tool-question",
						"message":    "Choose",
						"requestedSchema": map[string]any{
							"properties": map[string]any{
								"question_0": map[string]any{
									"type":  "string",
									"title": "Choice",
									"oneOf": []any{map[string]any{
										"const": "A",
										"title": "A — Alpha",
									}},
								},
							},
						},
					},
				})
				continue
			}
			if fakePromptText(params) == "please approve" {
				pendingPromptID = req.ID
				waitingForPermission = true
				writeFake(writer, map[string]any{
					"jsonrpc": "2.0",
					"id":      "permission-1",
					"method":  "permission/request",
					"params": map[string]any{
						"permissionId": "perm-shell",
						"action":       "shell",
						"command":      "go test ./...",
						"reason":       "Run tests",
					},
				})
				continue
			}
			emitFakeUpdate(writer, "agent_thought_chunk", map[string]any{"content": map[string]any{"type": "text", "text": "thinking"}})
			emitFakeUpdate(writer, "agent_message_chunk", map[string]any{"content": map[string]any{"type": "text", "text": "hello from fake"}})
			emitFakeUpdate(writer, "tool_call", map[string]any{"toolCallId": "tool-1", "title": "search", "rawInput": map[string]any{"q": "helios"}})
			emitFakeUpdate(writer, "tool_call_update", map[string]any{"toolCallId": "tool-1", "status": "completed", "content": []any{map[string]any{"type": "text", "text": "tool result"}}})
			emitFakeUpdate(writer, "usage_update", map[string]any{"used": 7, "size": 100})
			emitFakeUpdate(writer, "plan", map[string]any{"entries": []any{map[string]any{"priority": 1, "status": "done", "content": "tested"}}})
			writeFakeResult(writer, req.ID, map[string]any{"stopReason": "end_turn"})
		case "session/end":
			writeFakeResult(writer, req.ID, map[string]any{"ok": true})
		default:
			writeFakeError(writer, req.ID, -32601, "unknown method "+req.Method)
		}
	}
}

func fakePromptText(params PromptParams) string {
	for _, block := range params.Prompt {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}

func emitFakeUpdate(writer *bufio.Writer, typ string, fields map[string]any) {
	fields["sessionUpdate"] = typ
	writeFake(writer, map[string]any{
		"jsonrpc": "2.0",
		"method":  "session/update",
		"params": map[string]any{
			"sessionId": "fake-session-new",
			"update":    fields,
		},
	})
}

func writeFakeResult(writer *bufio.Writer, id any, result any) {
	writeFake(writer, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func writeFakeError(writer *bufio.Writer, id any, code int, message string) {
	writeFake(writer, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}

func writeFake(writer *bufio.Writer, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(writer, string(data))
	writer.Flush()
}

func TestFakeHelperIsNotLeaking(t *testing.T) {
	if strings.Contains(strings.Join(os.Args, " "), "HELIOS_FAKE_ACP") {
		t.Fatalf("unexpected fake helper args: %v", os.Args)
	}
}
