package claude_code

import (
	"strings"
	"testing"

	helios "github.com/colink-ai/helios/runtime"
)

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

func TestOptionsAndNewAdapter(t *testing.T) {
	cfg := config{}
	WithCLIPath("custom-claude")(&cfg)
	if cfg.cliPath != "custom-claude" {
		t.Fatalf("cli path = %q", cfg.cliPath)
	}
	if adapter := NewAdapter(WithCLIPath("custom-claude")); adapter == nil {
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

func TestClaudeEnvShape(t *testing.T) {
	env := strings.Join(buildEnv(helios.SessionRequest{Agent: helios.AgentSpec{APIURL: "https://api.test", APIToken: "token"}}), "\n")
	if !strings.Contains(env, "ANTHROPIC_API_KEY=token") || !strings.Contains(env, "ANTHROPIC_BASE_URL=https://api.test") {
		t.Fatalf("unexpected env: %s", env)
	}
}

func TestClaudeEnvEmpty(t *testing.T) {
	if env := buildEnv(helios.SessionRequest{}); len(env) != 0 {
		t.Fatalf("env = %v", env)
	}
}
