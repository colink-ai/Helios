package hermes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	helios "github.com/colink-ai/helios/runtime"
)

func TestRenderConfig(t *testing.T) {
	cfg := renderConfig(helios.AgentSpec{
		DefaultModel: "qwen-plus",
		APIURL:       "https://example.test/v1",
	}, []helios.MCPServerSpec{
		{Name: "knowledge", Type: "http", URL: "http://127.0.0.1:9000/mcp", Headers: map[string]string{"Authorization": "Bearer x"}},
		{Name: "fs", Type: "stdio", Command: "mcp-fs", Args: []string{"."}},
	})
	for _, want := range []string{
		`default: "qwen-plus"`,
		`provider: custom`,
		`base_url: "https://example.test/v1"`,
		`knowledge:`,
		`url: "http://127.0.0.1:9000/mcp"`,
		`command: "mcp-fs"`,
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config missing %q:\n%s", want, cfg)
		}
	}
}

func TestWriteConfigAndEnv(t *testing.T) {
	home := t.TempDir()
	spec := helios.AgentSpec{
		DefaultModel: "glm-test",
		APIURL:       "https://model.test/v1",
		APIToken:     "secret",
	}
	if err := writeConfig(home, spec, nil); err != nil {
		t.Fatalf("write config: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), `default: "glm-test"`) {
		t.Fatalf("unexpected config:\n%s", string(data))
	}
	env := strings.Join(buildEnv(home, spec), "\n")
	for _, want := range []string{"HERMES_HOME=", "HERMES_INFERENCE_PROVIDER=custom", "CUSTOM_BASE_URL=https://model.test/v1", "OPENAI_API_KEY=secret"} {
		if !strings.Contains(env, want) {
			t.Fatalf("env missing %q: %s", want, env)
		}
	}
}

func TestRegister(t *testing.T) {
	reg := helios.NewRegistry()
	if err := Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	adapter, err := reg.Create(helios.AgentSpec{Type: Type})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if adapter == nil {
		t.Fatalf("adapter is nil")
	}
}
