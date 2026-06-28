package open_claw

import (
	"fmt"
	"strconv"

	"github.com/colink-ai/helios/adapters/acp"
	helios "github.com/colink-ai/helios/runtime"
)

const Type = "open_claw"

type Option func(*config)

type config struct {
	cliPath     string
	gatewayURL  string
	gatewayPort int
	token       string
}

func WithCLIPath(path string) Option {
	return func(c *config) { c.cliPath = path }
}

func WithGatewayURL(url string) Option {
	return func(c *config) { c.gatewayURL = url }
}

func WithGatewayPort(port int) Option {
	return func(c *config) { c.gatewayPort = port }
}

func WithToken(token string) Option {
	return func(c *config) { c.token = token }
}

func NewAdapter(opts ...Option) helios.Adapter {
	cfg := config{cliPath: "openclaw", gatewayPort: 26888}
	for _, opt := range opts {
		opt(&cfg)
	}
	return acp.NewBaseAdapter(acp.Config{
		CLIPath: cfg.cliPath,
		BuildArgs: func(req helios.SessionRequest) []string {
			sessionKey := req.SessionID
			if sessionKey == "" {
				sessionKey = helios.NewID("session")
			}
			url := cfg.gatewayURL
			if url == "" {
				url = fmt.Sprintf("ws://127.0.0.1:%d", cfg.gatewayPort)
			}
			args := []string{"acp", "--url", url, "--session", "agent:main:" + sessionKey}
			if cfg.token != "" {
				args = append(args, "--token", cfg.token)
			}
			return args
		},
		BuildEnv: func(helios.SessionRequest) []string {
			env := []string{}
			if cfg.gatewayPort > 0 {
				env = append(env, "OPENCLAW_GATEWAY_PORT="+strconv.Itoa(cfg.gatewayPort))
			}
			return env
		},
	})
}

func Register(registry *helios.Registry, opts ...Option) error {
	return registry.Register(helios.AdapterMeta{
		Type:        Type,
		Name:        "OpenClaw",
		Description: "OpenClaw ACP bridge adapter",
		DefaultPath: "openclaw",
		Factory: func(spec helios.AgentSpec) (helios.Adapter, error) {
			localOpts := append([]Option{}, opts...)
			if spec.CLIPath != "" {
				localOpts = append(localOpts, WithCLIPath(spec.CLIPath))
			}
			return NewAdapter(localOpts...), nil
		},
	})
}
