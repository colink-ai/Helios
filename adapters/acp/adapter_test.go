package acp

import (
	"os/exec"
	"testing"

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
