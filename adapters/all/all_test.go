package all

import (
	"testing"

	"github.com/colink-ai/helios/adapters/claude_code"
	"github.com/colink-ai/helios/adapters/hermes"
	"github.com/colink-ai/helios/adapters/open_claw"
	"github.com/colink-ai/helios/adapters/open_code"
	helios "github.com/colink-ai/helios/runtime"
)

func TestRegister(t *testing.T) {
	reg := helios.NewRegistry()
	if err := Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	got := map[string]bool{}
	for _, meta := range reg.Types() {
		got[meta.Type] = true
	}
	for _, typ := range []string{hermes.Type, open_code.Type, claude_code.Type, open_claw.Type} {
		if !got[typ] {
			t.Fatalf("missing adapter type %s in %+v", typ, got)
		}
	}
}
