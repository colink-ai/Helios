package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

func TestConvertMCPServers(t *testing.T) {
	servers := ConvertMCPServers([]helios.MCPServerSpec{
		{Name: "search", Type: "http", URL: "http://127.0.0.1:9000/mcp", Headers: map[string]string{"Authorization": "Bearer token"}},
		{Name: "fs", Type: "stdio", Command: "mcp-fs", Args: []string{"."}, Env: map[string]string{"A": "B"}},
		{Name: "bad-http", Type: "http"},
		{Name: "unknown", Type: "weird"},
	})
	if len(servers) != 2 {
		t.Fatalf("servers len = %d, want 2: %+v", len(servers), servers)
	}
	first := servers[0].(map[string]any)
	if first["name"] != "search" || first["type"] != "http" || first["url"] == "" {
		t.Fatalf("unexpected http server: %+v", first)
	}
	second := servers[1].(map[string]any)
	if second["name"] != "fs" || second["type"] != "stdio" || second["command"] != "mcp-fs" {
		t.Fatalf("unexpected stdio server: %+v", second)
	}
}

func TestSupportsResume(t *testing.T) {
	if !supportsResume(map[string]any{"sessionResume": true}) {
		t.Fatalf("sessionResume should be supported")
	}
	if !supportsResume(map[string]any{"sessions": map[string]any{"resume": true}}) {
		t.Fatalf("nested sessions.resume should be supported")
	}
	if supportsResume(map[string]any{"sessions": map[string]any{"resume": false}}) {
		t.Fatalf("resume=false should not be supported")
	}
}

func TestSupportsLoad(t *testing.T) {
	if !supportsLoad(map[string]any{"sessionLoad": true}) {
		t.Fatalf("sessionLoad should be supported")
	}
	if !supportsLoad(map[string]any{"sessions": map[string]any{"load": true}}) {
		t.Fatalf("nested sessions.load should be supported")
	}
	if supportsLoad(map[string]any{"sessions": map[string]any{"load": false}}) {
		t.Fatalf("load=false should not be supported")
	}
}

func TestNormalizeCapabilities(t *testing.T) {
	capabilities := NormalizeCapabilities(helios.AgentSpec{
		Type:               "fake",
		Name:               "Fake",
		SupportsMultimodal: true,
	}, map[string]any{
		"sessionResume": true,
		"features": map[string]any{
			"usage":     true,
			"artifacts": true,
			"handoffs":  true,
		},
	})
	if capabilities.Protocol != "acp" || capabilities.AgentType != "fake" || capabilities.AgentName != "Fake" {
		t.Fatalf("unexpected identity: %+v", capabilities)
	}
	if !capabilities.ResidentSessions || !capabilities.OneShotRuns || !capabilities.NativeResume || !capabilities.Usage || !capabilities.Artifacts || !capabilities.Handoffs || !capabilities.Multimodal {
		t.Fatalf("unexpected capabilities: %+v", capabilities)
	}
}

func TestTakePendingElicitation(t *testing.T) {
	values := map[string]pendingElicitation{
		"first":  {request: "r1"},
		"second": {request: "r2"},
	}
	key, pending := takePendingElicitation(values, "second")
	if key != "second" || pending.request != "r2" {
		t.Fatalf("unexpected pending: %s %+v", key, pending)
	}
	key, pending = takePendingElicitation(values, "")
	if key == "" || pending.request == nil {
		t.Fatalf("expected fallback pending, got %s %+v", key, pending)
	}
}

func TestTakePendingPermission(t *testing.T) {
	values := map[string]pendingPermission{"p1": {request: "r1"}, "p2": {request: "r2"}}
	key, pending := takePendingPermission(values, "p2")
	if key != "p2" || pending.request != "r2" {
		t.Fatalf("unexpected pending: %s %+v", key, pending)
	}
	key, pending = takePendingPermission(values, "")
	if key == "" || pending.request == nil {
		t.Fatalf("expected fallback pending, got %s %+v", key, pending)
	}
}

func TestMonitorProcessRecordsExit(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 7")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	s := &session{cmd: cmd, status: helios.SessionRunning, waitDone: make(chan struct{})}
	go monitorProcess(s)
	<-s.waitDone
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.exited || s.exitErr == nil || s.status != helios.SessionFailed {
		t.Fatalf("unexpected session state: exited=%v err=%v status=%s", s.exited, s.exitErr, s.status)
	}
}

func TestSessionInspectorsAndPendingRequests(t *testing.T) {
	adapter := NewBaseAdapter(Config{})
	s := &session{
		id:             "session-1",
		agentSessionID: "agent-session-1",
		status:         helios.SessionRunning,
		nativeResume:   true,
		resumeStrategy: "resume",
		events:         make(chan helios.SessionRuntimeEvent),
		pendingElicitations: map[string]pendingElicitation{
			"tool-1": {request: "req-1", questions: []contracts.QuestionItem{{ID: "q1"}}, createdAt: time.Now().UTC()},
		},
		pendingPermissions: map[string]pendingPermission{
			"perm-1": {request: "req-2", createdAt: time.Now().UTC()},
		},
	}
	s.stderr.WriteString("stderr line\n")
	adapter.sessions["session-1"] = s

	status, err := adapter.GetSessionStatus(context.Background(), "session-1")
	if err != nil || status != helios.SessionRunning {
		t.Fatalf("status=%s err=%v", status, err)
	}
	agentSessionID, err := adapter.AgentSessionID(context.Background(), "session-1")
	if err != nil || agentSessionID != "agent-session-1" {
		t.Fatalf("agent session id=%q err=%v", agentSessionID, err)
	}
	events, err := adapter.SessionEvents(context.Background(), "session-1")
	if err != nil || events == nil {
		t.Fatalf("events=%v err=%v", events, err)
	}
	diag, err := adapter.Diagnostics(context.Background(), "session-1")
	if err != nil || diag.Stderr == "" || diag.Metadata["resumeStrategy"] != "resume" {
		t.Fatalf("diag=%+v err=%v", diag, err)
	}
	pending, err := adapter.PendingRequests(context.Background(), "session-1")
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
}

func TestHelperFallbacks(t *testing.T) {
	if parseSessionID([]byte(`bad`), "fallback") != "fallback" {
		t.Fatalf("bad session id should fallback")
	}
	if parseSessionID([]byte(`{"id":"abc"}`), "fallback") != "abc" {
		t.Fatalf("id should be parsed")
	}
	if supportsResume(nil) || supportsLoad(nil) {
		t.Fatalf("nil capabilities should not support resume/load")
	}
	if capabilityBool(map[string]any{"features": map[string]any{"x": true}}, "x") != true {
		t.Fatalf("nested capability not detected")
	}
	if stringFromAny(1) != "" {
		t.Fatalf("non-string should be empty")
	}
	blocks := promptBlocks(helios.PromptRequest{
		Input:  "hello",
		Images: []contracts.ImageContent{{MimeType: "image/png", Data: "data"}},
	})
	if len(blocks) != 2 || blocks[1].Type != "image" {
		t.Fatalf("unexpected blocks: %+v", blocks)
	}
}

func TestCaptureStderrAndText(t *testing.T) {
	s := &session{}
	captureStderr(io.NopCloser(bytes.NewBufferString("a\nb\n")), s)
	if got := s.stderrText(); got == "" {
		t.Fatalf("stderr text empty")
	}
}

func TestCancelPendingRequests(t *testing.T) {
	adapter := NewBaseAdapter(Config{})
	out := &writeBuffer{}
	s := &session{
		id: "session-1",
		pendingElicitations: map[string]pendingElicitation{
			"tool-1": {request: "r1", createdAt: time.Now().UTC()},
		},
		pendingPermissions: map[string]pendingPermission{
			"perm-1": {request: "r2", createdAt: time.Now().UTC()},
		},
		transport: newTransport(io.NopCloser(strings.NewReader("")), out, nil, nil),
	}
	adapter.sessions["session-1"] = s
	if err := adapter.CancelPendingRequest(context.Background(), "session-1", "tool-1", "no"); err != nil {
		t.Fatalf("cancel elicitation: %v", err)
	}
	if !strings.Contains(out.String(), `"decline"`) {
		t.Fatalf("unexpected response: %s", out.String())
	}
	if err := adapter.CancelPendingRequest(context.Background(), "session-1", "perm-1", "no"); err != nil {
		t.Fatalf("cancel permission: %v", err)
	}
	if !strings.Contains(out.String(), `"reject"`) {
		t.Fatalf("unexpected response: %s", out.String())
	}
	if err := adapter.CancelPendingRequest(context.Background(), "session-1", "missing", "no"); err == nil {
		t.Fatalf("missing pending should fail")
	}
}

func TestHandleRequestElicitationDeclinesInvalidRequests(t *testing.T) {
	adapter := NewBaseAdapter(Config{})
	out := &writeBuffer{}
	s := &session{transport: newTransport(io.NopCloser(strings.NewReader("")), out, nil, nil)}

	adapter.handleRequest(s, "bad-json", "elicitation/create", json.RawMessage(`{`))
	adapter.handleRequest(s, "bad-mode", "elicitation/create", json.RawMessage(`{"mode":"freeform","requestedSchema":{"properties":{"question_0":{"type":"string"}}}}`))
	adapter.handleRequest(s, "no-questions", "elicitation/create", json.RawMessage(`{"mode":"form","requestedSchema":{"properties":{"other":{"type":"string"}}}}`))
	if got := out.String(); strings.Count(got, `"decline"`) != 3 {
		t.Fatalf("expected three declines, got: %s", got)
	}
	if len(s.pendingElicitations) != 0 {
		t.Fatalf("invalid elicitations should not be pending: %+v", s.pendingElicitations)
	}
}

func TestHandleRequestPermissionFallbackAndUnknownMethod(t *testing.T) {
	adapter := NewBaseAdapter(Config{})
	out := &writeBuffer{}
	var chunks []contracts.Chunk
	s := &session{transport: newTransport(io.NopCloser(strings.NewReader("")), out, nil, nil)}
	s.onChunk = func(chunk contracts.Chunk) {
		chunks = append(chunks, chunk)
	}

	adapter.handleRequest(s, "perm-id", "permission/request", json.RawMessage(`{"action":"shell","command":"go test","reason":"run tests"}`))
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkPermission || chunks[0].Permission.ID != "permission-perm-id" {
		t.Fatalf("unexpected permission chunk: %+v", chunks)
	}
	if len(s.pendingPermissions) != 1 {
		t.Fatalf("permission should be pending: %+v", s.pendingPermissions)
	}

	adapter.handleRequest(s, "unknown-id", "unknown/method", json.RawMessage(`{}`))
	if !strings.Contains(out.String(), `"method not found"`) {
		t.Fatalf("unknown method should get JSON-RPC error: %s", out.String())
	}
}

func TestDirectAdapterErrorHelpers(t *testing.T) {
	adapter := NewBaseAdapter(Config{})
	if err := adapter.CheckHealth(context.Background(), helios.AgentSpec{}); err == nil {
		t.Fatalf("check health without cli should fail")
	}
	if _, err := adapter.DetectCapabilities(context.Background(), helios.AgentSpec{}); err == nil {
		t.Fatalf("detect without cli should fail")
	}
	s := &session{}
	if err := adapter.configureModel(context.Background(), helios.SessionRequest{}, s); err != nil {
		t.Fatalf("empty model config should be a no-op: %v", err)
	}
}

func TestProtocolAndTransportHelpers(t *testing.T) {
	req := NewRequest(1, "method", map[string]any{"x": 1})
	if req.JSONRPC != "2.0" || req.Method != "method" {
		t.Fatalf("unexpected request: %+v", req)
	}
	sessionParams, err := json.Marshal(SessionParams{MCPServers: []any{}})
	if err != nil {
		t.Fatalf("marshal session params: %v", err)
	}
	if !strings.Contains(string(sessionParams), `"mcpServers":[]`) {
		t.Fatalf("empty mcpServers must be serialized for strict ACP CLIs: %s", string(sessionParams))
	}
	for _, value := range []any{float64(1), 2, int64(3), "four", map[string]any{"x": 1}} {
		if idKey(value) == "" {
			t.Fatalf("empty id key for %#v", value)
		}
	}
	out := &writeBuffer{}
	tp := newTransport(io.NopCloser(strings.NewReader("")), out, nil, nil)
	if err := tp.write(func() {}); err == nil {
		t.Fatalf("unmarshalable write should fail")
	}
	if err := tp.sendResponse("id", func() {}, nil); err == nil {
		t.Fatalf("unmarshalable response should fail")
	}
}

type writeBuffer struct {
	bytes.Buffer
}

func (w *writeBuffer) Close() error { return nil }

func TestJSONNumberHelpers(t *testing.T) {
	values := map[string]any{"i": json.Number("7"), "f": json.Number("2.5")}
	if int64Value(values, "i") != 7 || floatValue(values, "f") != 2.5 {
		t.Fatalf("unexpected values")
	}
}
