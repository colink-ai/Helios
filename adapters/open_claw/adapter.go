package open_claw

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/colink-ai/helios/adapters/acp"
	helios "github.com/colink-ai/helios/runtime"
)

const Type = "open_claw"

type Option func(*config)

type config struct {
	cliPath         string
	gatewayURL      string
	gatewayPort     int
	token           string
	protocolVersion int
	launcher        GatewayLauncher
}

// GatewayLauncher lets host applications provide or manage an OpenClaw gateway.
type GatewayLauncher interface {
	GatewayURL(req helios.SessionRequest) string
	Env(req helios.SessionRequest) []string
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

func WithProtocolVersion(version int) Option {
	return func(c *config) { c.protocolVersion = version }
}

func WithGatewayLauncher(launcher GatewayLauncher) Option {
	return func(c *config) { c.launcher = launcher }
}

func NewAdapter(opts ...Option) helios.Adapter {
	cfg := config{cliPath: "openclaw", gatewayPort: 26888}
	for _, opt := range opts {
		opt(&cfg)
	}
	return acp.NewBaseAdapter(acp.Config{
		CLIPath:         cfg.cliPath,
		BuildArgs:       func(req helios.SessionRequest) []string { return buildArgs(cfg, req) },
		BuildEnv:        func(req helios.SessionRequest) []string { return buildEnv(cfg, req) },
		ProtocolVersion: cfg.protocolVersion,
	})
}

func buildArgs(cfg config, req helios.SessionRequest) []string {
	sessionKey := openClawSessionKey(req)
	url := cfg.gatewayURL
	if cfg.launcher != nil {
		url = cfg.launcher.GatewayURL(req)
	}
	if url == "" {
		url = fmt.Sprintf("ws://127.0.0.1:%d", cfg.gatewayPort)
	}
	args := []string{"acp", "--url", url, "--session", sessionKey}
	if cfg.token != "" {
		args = append(args, "--token", cfg.token)
	}
	return args
}

func openClawSessionKey(req helios.SessionRequest) string {
	sessionKey := req.ResumeSessionID
	if sessionKey == "" {
		sessionKey = req.SessionID
	}
	if sessionKey == "" {
		sessionKey = helios.NewID("session")
	}
	if strings.HasPrefix(sessionKey, "agent:") {
		return sessionKey
	}
	return "agent:main:" + sessionKey
}

func buildEnv(cfg config, req helios.SessionRequest) []string {
	env := []string{}
	if helios.EffectiveRuntimeConfigMode(req) != helios.RuntimeConfigUser {
		if configDir := helios.EffectiveConfigDir(req); configDir != "" {
			env = append(env,
				"OPENCLAW_STATE_DIR="+configDir,
				"OPENCLAW_CONFIG_PATH="+filepath.Join(configDir, "openclaw.json"),
			)
		}
	}
	if cfg.gatewayPort > 0 {
		env = append(env, "OPENCLAW_GATEWAY_PORT="+strconv.Itoa(cfg.gatewayPort))
	}
	if cfg.launcher != nil {
		env = append(env, cfg.launcher.Env(req)...)
	}
	return env
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
			if url, ok := metadataString(spec.Metadata, "gatewayURL"); ok {
				localOpts = append(localOpts, WithGatewayURL(url))
			}
			if port, ok := metadataInt(spec.Metadata, "gatewayPort"); ok {
				localOpts = append(localOpts, WithGatewayPort(port))
			}
			if token, ok := metadataString(spec.Metadata, "gatewayToken"); ok {
				localOpts = append(localOpts, WithToken(token))
			}
			if version := acp.ProtocolVersionFromMetadata(spec.Metadata); version > 0 {
				localOpts = append(localOpts, WithProtocolVersion(version))
			}
			return NewAdapter(localOpts...), nil
		},
	})
}

func metadataString(metadata map[string]any, key string) (string, bool) {
	if metadata == nil {
		return "", false
	}
	value, ok := metadata[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok && text != ""
}

func metadataInt(metadata map[string]any, key string) (int, bool) {
	if metadata == nil {
		return 0, false
	}
	switch value := metadata[key].(type) {
	case int:
		return value, value > 0
	case int32:
		return int(value), value > 0
	case int64:
		return int(value), value > 0
	case float64:
		return int(value), value > 0
	case string:
		port, err := strconv.Atoi(value)
		return port, err == nil && port > 0
	default:
		return 0, false
	}
}
