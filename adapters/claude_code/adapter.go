package claude_code

import (
	"github.com/colink-ai/helios/adapters/acp"
	helios "github.com/colink-ai/helios/runtime"
)

const Type = "claude_code"

type Option func(*config)

type config struct {
	cliPath string
}

func WithCLIPath(path string) Option {
	return func(c *config) { c.cliPath = path }
}

func NewAdapter(opts ...Option) helios.Adapter {
	cfg := config{cliPath: "claude-agent-acp"}
	for _, opt := range opts {
		opt(&cfg)
	}
	return acp.NewBaseAdapter(acp.Config{
		CLIPath: cfg.cliPath,
		BuildArgs: func(helios.SessionRequest) []string {
			return nil
		},
		BuildEnv: buildEnv,
	})
}

func Register(registry *helios.Registry, opts ...Option) error {
	return registry.Register(helios.AdapterMeta{
		Type:        Type,
		Name:        "Claude Code",
		Description: "Claude Code ACP adapter",
		DefaultPath: "claude-agent-acp",
		Factory: func(spec helios.AgentSpec) (helios.Adapter, error) {
			localOpts := append([]Option{}, opts...)
			if spec.CLIPath != "" {
				localOpts = append(localOpts, WithCLIPath(spec.CLIPath))
			}
			return NewAdapter(localOpts...), nil
		},
	})
}

func buildEnv(req helios.SessionRequest) []string {
	env := []string{}
	if helios.EffectiveRuntimeConfigMode(req) != helios.RuntimeConfigUser {
		if configDir := helios.EffectiveConfigDir(req); configDir != "" {
			env = append(env, "CLAUDE_CONFIG_DIR="+configDir)
		}
	}
	if req.Agent.APIToken != "" {
		env = append(env, "ANTHROPIC_API_KEY="+req.Agent.APIToken)
	}
	if req.Agent.APIURL != "" {
		env = append(env, "ANTHROPIC_BASE_URL="+req.Agent.APIURL)
	}
	return env
}
