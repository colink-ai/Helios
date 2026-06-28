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
