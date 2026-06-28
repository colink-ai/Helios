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
	})
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
}

func TestBuildEnv(t *testing.T) {
	workDir := t.TempDir()
	env := strings.Join(buildEnv(helios.SessionRequest{
		WorkDir: workDir,
		Agent:   helios.AgentSpec{DefaultModel: "qwen-plus"},
	}), "\n")
	for _, want := range []string{"OPENCODE_PURE=1", "OPENCODE_ENABLE_QUESTION_TOOL=1", "OPENCODE_CONFIG_DIR=", "OPENCODE_CONFIG_CONTENT="} {
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
