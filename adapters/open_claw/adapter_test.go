package open_claw

import (
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
