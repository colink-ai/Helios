package hermes

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/colink-ai/helios/adapters/acp"
	helios "github.com/colink-ai/helios/runtime"
	"gopkg.in/yaml.v3"
)

const Type = "hermes"

type Option func(*config)

// ConfigMutator lets host applications add adapter-specific Hermes config
// keys without reimplementing Hermes config rendering in the host.
type ConfigMutator func(map[string]any)

type config struct {
	cliPath         string
	configMutators  []ConfigMutator
	protocolVersion int
	promptTimeout   time.Duration
}

func WithCLIPath(path string) Option {
	return func(c *config) { c.cliPath = path }
}

func WithConfigMutator(mutator ConfigMutator) Option {
	return func(c *config) {
		if mutator != nil {
			c.configMutators = append(c.configMutators, mutator)
		}
	}
}

func WithPromptTimeout(timeout time.Duration) Option {
	return func(c *config) { c.promptTimeout = timeout }
}

func WithProtocolVersion(version int) Option {
	return func(c *config) { c.protocolVersion = version }
}

func NewAdapter(opts ...Option) helios.Adapter {
	cfg := config{cliPath: "hermes"}
	for _, opt := range opts {
		opt(&cfg)
	}
	return acp.NewBaseAdapter(acp.Config{
		CLIPath: cfg.cliPath,
		BuildArgs: func(helios.SessionRequest) []string {
			return []string{"acp"}
		},
		BuildEnv: func(req helios.SessionRequest) []string {
			home := runtimeHome(req)
			if home != "" {
				_ = writeConfig(home, req.Agent, req.MCPServers, cfg.configMutators...)
			}
			return buildEnv(home, req.Agent)
		},
		ProtocolVersion: cfg.protocolVersion,
		PromptTimeout:   cfg.promptTimeout,
	})
}

func Register(registry *helios.Registry, opts ...Option) error {
	return registry.Register(helios.AdapterMeta{
		Type:        Type,
		Name:        "Hermes",
		Description: "Hermes ACP adapter",
		DefaultPath: "hermes",
		Factory: func(spec helios.AgentSpec) (helios.Adapter, error) {
			localOpts := append([]Option{}, opts...)
			if spec.CLIPath != "" {
				localOpts = append(localOpts, WithCLIPath(spec.CLIPath))
			}
			if spec.PromptTimeout != 0 {
				localOpts = append(localOpts, WithPromptTimeout(spec.PromptTimeout))
			}
			if version := acp.ProtocolVersionFromMetadata(spec.Metadata); version > 0 {
				localOpts = append(localOpts, WithProtocolVersion(version))
			}
			return NewAdapter(localOpts...), nil
		},
	})
}

func runtimeHome(req helios.SessionRequest) string {
	if helios.EffectiveRuntimeConfigMode(req) == helios.RuntimeConfigUser {
		return ""
	}
	if configDir := helios.EffectiveConfigDir(req); configDir != "" {
		return configDir
	}
	return helios.EffectiveRuntimeHome(req)
}

func buildEnv(home string, spec helios.AgentSpec) []string {
	env := []string{
		"NO_PROXY=127.0.0.1,localhost,::1",
		"no_proxy=127.0.0.1,localhost,::1",
	}
	if spec.APIURL != "" || spec.APIToken != "" {
		env = append(env, "HERMES_INFERENCE_PROVIDER=custom")
		if spec.APIURL != "" {
			env = append(env, "CUSTOM_BASE_URL="+spec.APIURL)
		}
		if spec.APIToken != "" {
			env = append(env, "OPENAI_API_KEY="+spec.APIToken)
		}
	}
	if home != "" {
		env = append(env, "HERMES_HOME="+abs(home))
	}
	return env
}

func writeConfig(home string, spec helios.AgentSpec, servers []helios.MCPServerSpec, mutators ...ConfigMutator) error {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	path := filepath.Join(home, "config.yaml")
	content, err := renderConfig(loadConfig(path), spec, servers, mutators...)
	if err != nil {
		return err
	}
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func loadConfig(path string) map[string]any {
	cfg := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return map[string]any{}
	}
	return cfg
}

func renderConfig(existing map[string]any, spec helios.AgentSpec, servers []helios.MCPServerSpec, mutators ...ConfigMutator) (string, error) {
	cfg := existing
	if cfg == nil {
		cfg = map[string]any{}
	}
	if spec.DefaultModel != "" || spec.APIURL != "" || spec.APIToken != "" {
		model, _ := cfg["model"].(map[string]any)
		if model == nil {
			model = map[string]any{}
		}
		if spec.DefaultModel != "" {
			model["default"] = spec.DefaultModel
		}
		if spec.APIURL != "" || spec.APIToken != "" {
			model["provider"] = "custom"
		}
		if spec.APIURL != "" {
			model["base_url"] = spec.APIURL
		}
		cfg["model"] = model
	}
	if mcp := renderMCPServers(servers); len(mcp) > 0 {
		cfg["mcp_servers"] = mcp
	} else {
		delete(cfg, "mcp_servers")
	}
	for _, mutator := range mutators {
		if mutator != nil {
			mutator(cfg)
		}
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func renderMCPServers(servers []helios.MCPServerSpec) map[string]any {
	filtered := make([]helios.MCPServerSpec, 0, len(servers))
	for _, server := range servers {
		if server.Name != "" {
			filtered = append(filtered, server)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	out := make(map[string]any, len(filtered))
	for _, server := range filtered {
		item := map[string]any{"enabled": true}
		switch server.Type {
		case "http", "sse":
			if server.URL == "" {
				continue
			}
			item["url"] = server.URL
			if len(server.Headers) > 0 {
				item["headers"] = server.Headers
			}
		case "stdio":
			if server.Command == "" {
				continue
			}
			item["command"] = server.Command
			if len(server.Args) > 0 {
				item["args"] = server.Args
			}
			if len(server.Env) > 0 {
				item["env"] = server.Env
			}
		default:
			continue
		}
		out[server.Name] = item
	}
	return out
}

func abs(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if absPath, err := filepath.Abs(path); err == nil {
		return absPath
	}
	return path
}

func quoteKey(s string) string {
	if s == "" {
		return `""`
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return quote(s)
	}
	return s
}

func quote(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return `"` + replacer.Replace(s) + `"`
}
