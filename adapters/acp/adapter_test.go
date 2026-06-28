package acp

import (
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
