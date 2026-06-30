package hermes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	helios "github.com/colink-ai/helios/runtime"
	"gopkg.in/yaml.v3"
)

func TestRenderConfig(t *testing.T) {
	cfg, err := renderConfig(nil, helios.AgentSpec{
		DefaultModel: "qwen-plus",
		APIURL:       "https://example.test/v1",
	}, []helios.MCPServerSpec{
		{Name: "knowledge", Type: "http", URL: "http://127.0.0.1:9000/mcp", Headers: map[string]string{"Authorization": "Bearer x"}},
		{Name: "fs", Type: "stdio", Command: "mcp-fs", Args: []string{"."}},
	})
	if err != nil {
		t.Fatalf("render config: %v", err)
	}
	for _, want := range []string{
		`default: qwen-plus`,
		`provider: custom`,
		`base_url: https://example.test/v1`,
		`knowledge:`,
		`url: http://127.0.0.1:9000/mcp`,
		`command: mcp-fs`,
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
	if !strings.Contains(string(data), `default: glm-test`) {
		t.Fatalf("unexpected config:\n%s", string(data))
	}
	env := strings.Join(buildEnv(home, spec), "\n")
	for _, want := range []string{"HERMES_HOME=", "HERMES_INFERENCE_PROVIDER=custom", "CUSTOM_BASE_URL=https://model.test/v1", "OPENAI_API_KEY=secret"} {
		if !strings.Contains(env, want) {
			t.Fatalf("env missing %q: %s", want, env)
		}
	}
}

func TestWriteConfigPreservesExistingAndAppliesMutator(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(configPath, []byte("existing:\n  enabled: true\nmcp_servers:\n  stale:\n    enabled: true\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	err := writeConfig(home, helios.AgentSpec{DefaultModel: "glm-test"}, nil, func(cfg map[string]any) {
		cfg["memory"] = map[string]any{"memory_enabled": false}
		cfg["skills"] = map[string]any{"guard_agent_created": true}
	})
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config should be yaml: %v\n%s", err, data)
	}
	if cfg["existing"].(map[string]any)["enabled"] != true {
		t.Fatalf("existing config should be preserved: %+v", cfg)
	}
	if _, ok := cfg["mcp_servers"]; ok {
		t.Fatalf("stale mcp servers should be removed when no servers are provided: %+v", cfg["mcp_servers"])
	}
	if cfg["memory"].(map[string]any)["memory_enabled"] != false {
		t.Fatalf("mutator config missing: %+v", cfg)
	}
}

func TestWriteConfigSkipsUnchangedFile(t *testing.T) {
	home := t.TempDir()
	spec := helios.AgentSpec{DefaultModel: "glm-test"}
	if err := writeConfig(home, spec, nil); err != nil {
		t.Fatalf("write config: %v", err)
	}
	info, err := os.Stat(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if err := writeConfig(home, spec, nil); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}
	infoAfter, err := os.Stat(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatalf("stat config after: %v", err)
	}
	if !infoAfter.ModTime().Equal(info.ModTime()) {
		t.Fatalf("unchanged config should not be rewritten")
	}
}

func TestRuntimeHomePrecedence(t *testing.T) {
	req := helios.SessionRequest{
		RuntimeHome: "request-home",
		Agent:       helios.AgentSpec{RuntimeHome: "agent-home"},
	}
	if got := runtimeHome(req); got != "request-home" {
		t.Fatalf("runtimeHome = %q", got)
	}
	req.RuntimeHome = ""
	if got := runtimeHome(req); got != "agent-home" {
		t.Fatalf("runtimeHome agent fallback = %q", got)
	}
	req.Agent.RuntimeHome = ""
	if got := runtimeHome(req); got != "" {
		t.Fatalf("runtimeHome empty = %q", got)
	}
	req.WorkDir = "work"
	req.RuntimeConfigMode = helios.RuntimeConfigUser
	if got := runtimeHome(req); got != "" {
		t.Fatalf("user config mode should not set runtime home, got %q", got)
	}
}

func TestConfigDirPrecedence(t *testing.T) {
	req := helios.SessionRequest{
		ConfigDir:   "request-config",
		RuntimeHome: "request-home",
		Agent:       helios.AgentSpec{ConfigDir: "agent-config", RuntimeHome: "agent-home"},
	}
	if got := runtimeHome(req); got != "request-config" {
		t.Fatalf("config dir should win over runtime home, got %q", got)
	}
	req.ConfigDir = ""
	if got := runtimeHome(req); got != "agent-config" {
		t.Fatalf("agent config dir should win over runtime home, got %q", got)
	}
	req.Agent.ConfigDir = ""
	if got := runtimeHome(req); got != "request-home" {
		t.Fatalf("runtime home fallback = %q", got)
	}
}

func TestOptionsAndHelpers(t *testing.T) {
	if NewAdapter(WithCLIPath("custom-hermes")) == nil {
		t.Fatalf("adapter is nil")
	}
	if got := abs(""); got != "" {
		t.Fatalf("abs empty = %q", got)
	}
	if got := abs("relative-home"); !filepath.IsAbs(got) {
		t.Fatalf("abs relative = %q", got)
	}
	if got := quoteKey(""); got != `""` {
		t.Fatalf("quoteKey empty = %q", got)
	}
	if got := quoteKey("simple-key"); got != "simple-key" {
		t.Fatalf("quoteKey simple = %q", got)
	}
	if got := quoteKey("needs space"); got != `"needs space"` {
		t.Fatalf("quoteKey space = %q", got)
	}
	if got := quote("line\nquote\""); got != `"line\nquote\""` {
		t.Fatalf("quote = %q", got)
	}
	if got := quote("carriage\rtab\t"); got != `"carriage\rtab\t"` {
		t.Fatalf("quote control chars = %q", got)
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

func TestRegisterSpecCLIOverride(t *testing.T) {
	reg := helios.NewRegistry()
	if err := Register(reg, WithCLIPath("default-hermes")); err != nil {
		t.Fatalf("register: %v", err)
	}
	adapter, err := reg.Create(helios.AgentSpec{Type: Type, CLIPath: "spec-hermes"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if adapter == nil {
		t.Fatalf("adapter is nil")
	}
}
