package open_code

import (
	"encoding/json"
	"strings"
	"testing"

	helios "github.com/colink-ai/helios/runtime"
)

func TestBuildConfigContent(t *testing.T) {
	content := buildConfigContent(helios.AgentSpec{
		DefaultModel: "qwen-plus",
		APIURL:       "https://model.test/v1",
		APIToken:     "secret",
	}, "")
	if content == "" {
		t.Fatalf("content is empty")
	}
	var cfg openCodeConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.Model != "helios/qwen-plus" {
		t.Fatalf("model = %q", cfg.Model)
	}
	provider := cfg.Provider[providerID]
	if provider.Options.APIKey != "secret" || provider.Options.BaseURL != "https://model.test/v1" {
		t.Fatalf("unexpected provider: %+v", provider)
	}
	if !provider.Models["qwen-plus"].Attachment {
		t.Fatalf("model should support attachments")
	}
	if cfg.Permission != nil {
		t.Fatalf("permission should not default to allow: %+v", cfg.Permission)
	}
}

func TestBuildConfigContentEmptyAndAPIOnly(t *testing.T) {
	if got := buildConfigContent(helios.AgentSpec{}, ""); got != "" {
		t.Fatalf("empty spec content = %q", got)
	}
	content := buildConfigContent(helios.AgentSpec{APIURL: "https://model.test/v1"}, "")
	if content == "" || !strings.Contains(content, "https://model.test/v1") {
		t.Fatalf("unexpected API-only content: %s", content)
	}
}

func TestBuildConfigContentPermissionMode(t *testing.T) {
	content := buildConfigContent(helios.AgentSpec{DefaultModel: "qwen-plus"}, "allow")
	var cfg openCodeConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.Permission != "allow" {
		t.Fatalf("permission = %#v", cfg.Permission)
	}
}

func TestBuildEnv(t *testing.T) {
	workDir := t.TempDir()
	env := strings.Join(buildEnv(helios.SessionRequest{
		WorkDir: workDir,
		Agent:   helios.AgentSpec{DefaultModel: "qwen-plus"},
	}, config{}), "\n")
	for _, want := range []string{"OPENCODE_PURE=1", "OPENCODE_ENABLE_QUESTION_TOOL=1", "OPENCODE_CONFIG_DIR=", "OPENCODE_CONFIG_CONTENT="} {
		if !strings.Contains(env, want) {
			t.Fatalf("env missing %q: %s", want, env)
		}
	}
}

func TestConfigDirPrecedence(t *testing.T) {
	if got := configDir(helios.SessionRequest{RuntimeHome: "/runtime", Agent: helios.AgentSpec{RuntimeHome: "/agent"}}); got != "/runtime/opencode" {
		t.Fatalf("runtime home dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{Agent: helios.AgentSpec{RuntimeHome: "/agent"}}); got != "/agent/opencode" {
		t.Fatalf("agent runtime home dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{WorkDir: "/work"}); got != "/work/.opencode" {
		t.Fatalf("work dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{Agent: helios.AgentSpec{WorkDir: "/agent-work"}}); got != "/agent-work/.opencode" {
		t.Fatalf("agent work dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{}); got != "" {
		t.Fatalf("empty dir = %q", got)
	}
}

func TestOptionsAndNewAdapter(t *testing.T) {
	cfg := config{}
	WithCLIPath("custom")(&cfg)
	WithHTTPPort(7777)(&cfg)
	WithPermissionMode("allow")(&cfg)
	if cfg.cliPath != "custom" || cfg.port != 7777 || cfg.permissionMode != "allow" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if adapter := NewAdapter(WithCLIPath("custom"), WithHTTPPort(7777)); adapter == nil {
		t.Fatalf("adapter is nil")
	}
}

func TestMetadataOptions(t *testing.T) {
	meta := map[string]any{"httpPort": "9191", "permission": "ask"}
	if value, ok := metadataInt(meta, "httpPort"); !ok || value != 9191 {
		t.Fatalf("unexpected httpPort: %d %v", value, ok)
	}
	if value, ok := metadataString(meta, "permission"); !ok || value != "ask" {
		t.Fatalf("unexpected permission: %q %v", value, ok)
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
	if err := Register(reg, WithCLIPath("default")); err != nil {
		t.Fatalf("register: %v", err)
	}
	adapter, err := reg.Create(helios.AgentSpec{Type: Type, CLIPath: "from-spec"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if adapter == nil {
		t.Fatalf("adapter is nil")
	}
}
