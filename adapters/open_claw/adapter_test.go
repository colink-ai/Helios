package open_claw

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
	WithCLIPath("claw")(&cfg)
	WithGatewayURL("ws://gateway")(&cfg)
	WithGatewayPort(1234)(&cfg)
	WithToken("token")(&cfg)
	WithGatewayLauncher(testLauncher{})(&cfg)
	if cfg.cliPath != "claw" || cfg.gatewayURL != "ws://gateway" || cfg.gatewayPort != 1234 || cfg.token != "token" || cfg.launcher == nil {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if adapter := NewAdapter(WithCLIPath("claw"), WithGatewayPort(1234)); adapter == nil {
		t.Fatalf("adapter is nil")
	}
}

func TestBuildArgsDefaults(t *testing.T) {
	args := buildArgs(config{gatewayPort: 26888}, helios.SessionRequest{})
	got := strings.Join(args, " ")
	if !strings.Contains(got, "ws://127.0.0.1:26888") || !strings.Contains(got, "agent:main:session-") {
		t.Fatalf("unexpected args: %v", args)
	}
	env := strings.Join(buildEnv(config{gatewayPort: 0}, helios.SessionRequest{}), "\n")
	if env != "" {
		t.Fatalf("unexpected env: %s", env)
	}
}

func TestBuildArgsResumeSession(t *testing.T) {
	args := buildArgs(config{gatewayPort: 26888}, helios.SessionRequest{SessionID: "new", ResumeSessionID: "resume"})
	got := strings.Join(args, " ")
	if !strings.Contains(got, "agent:main:resume") || strings.Contains(got, "agent:main:new") {
		t.Fatalf("unexpected resume args: %v", args)
	}
	args = buildArgs(config{gatewayPort: 26888}, helios.SessionRequest{ResumeSessionID: "agent:main:stored"})
	got = strings.Join(args, " ")
	if !strings.Contains(got, "agent:main:stored") || strings.Contains(got, "agent:main:agent:") {
		t.Fatalf("unexpected full resume args: %v", args)
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

func TestMetadataOptions(t *testing.T) {
	meta := map[string]any{
		"gatewayURL":   "ws://gateway",
		"gatewayPort":  19999,
		"gatewayToken": "token",
	}
	if value, ok := metadataString(meta, "gatewayURL"); !ok || value != "ws://gateway" {
		t.Fatalf("unexpected gatewayURL: %q %v", value, ok)
	}
	if value, ok := metadataInt(meta, "gatewayPort"); !ok || value != 19999 {
		t.Fatalf("unexpected gatewayPort: %d %v", value, ok)
	}
	if value, ok := metadataString(meta, "gatewayToken"); !ok || value != "token" {
		t.Fatalf("unexpected gatewayToken: %q %v", value, ok)
	}
	if value, ok := metadataInt(map[string]any{"gatewayPort": "20000"}, "gatewayPort"); !ok || value != 20000 {
		t.Fatalf("unexpected string gatewayPort: %d %v", value, ok)
	}
}

type testLauncher struct{}

func (testLauncher) GatewayURL(helios.SessionRequest) string { return "ws://gateway.test:9999" }
func (testLauncher) Env(helios.SessionRequest) []string {
	return []string{"OPENCLAW_GATEWAY_MODE=managed"}
}

func TestGatewayLauncher(t *testing.T) {
	cfg := config{gatewayPort: 26888, token: "token", launcher: testLauncher{}}
	args := buildArgs(cfg, helios.SessionRequest{SessionID: "s1"})
	got := strings.Join(args, " ")
	if !strings.Contains(got, "ws://gateway.test:9999") || !strings.Contains(got, "--token token") {
		t.Fatalf("unexpected args: %v", args)
	}
	env := strings.Join(buildEnv(cfg, helios.SessionRequest{}), "\n")
	if !strings.Contains(env, "OPENCLAW_GATEWAY_MODE=managed") {
		t.Fatalf("unexpected env: %s", env)
	}
}
