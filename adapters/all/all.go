package all

import (
	"github.com/colink-ai/helios/adapters/claude_code"
	"github.com/colink-ai/helios/adapters/hermes"
	"github.com/colink-ai/helios/adapters/open_claw"
	"github.com/colink-ai/helios/adapters/open_code"
	helios "github.com/colink-ai/helios/runtime"
)

func Register(registry *helios.Registry) error {
	for _, register := range []func(*helios.Registry) error{
		func(r *helios.Registry) error { return hermes.Register(r) },
		func(r *helios.Registry) error { return open_code.Register(r) },
		func(r *helios.Registry) error { return claude_code.Register(r) },
		func(r *helios.Registry) error { return open_claw.Register(r) },
	} {
		if err := register(registry); err != nil {
			return err
		}
	}
	return nil
}
