package hermes

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/colink-ai/helios/adapters/acp"
	helios "github.com/colink-ai/helios/runtime"
)

const Type = "hermes"

type Option func(*config)

type config struct {
	cliPath string
}

func WithCLIPath(path string) Option {
	return func(c *config) { c.cliPath = path }
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
				_ = writeConfig(home, req.Agent, req.MCPServers)
			}
			return buildEnv(home, req.Agent)
		},
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
			return NewAdapter(localOpts...), nil
		},
	})
}

func runtimeHome(req helios.SessionRequest) string {
	if req.RuntimeHome != "" {
		return req.RuntimeHome
	}
	return req.Agent.RuntimeHome
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

func writeConfig(home string, spec helios.AgentSpec, servers []helios.MCPServerSpec) error {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	content := renderConfig(spec, servers)
	path := filepath.Join(home, "config.yaml")
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func renderConfig(spec helios.AgentSpec, servers []helios.MCPServerSpec) string {
	var b strings.Builder
	if spec.DefaultModel != "" || spec.APIURL != "" || spec.APIToken != "" {
		b.WriteString("model:\n")
		if spec.DefaultModel != "" {
			b.WriteString("  default: ")
			b.WriteString(quote(spec.DefaultModel))
			b.WriteByte('\n')
		}
		if spec.APIURL != "" || spec.APIToken != "" {
			b.WriteString("  provider: custom\n")
		}
		if spec.APIURL != "" {
			b.WriteString("  base_url: ")
			b.WriteString(quote(spec.APIURL))
			b.WriteByte('\n')
		}
	}
	mcp := renderMCPServers(servers)
	if mcp != "" {
		b.WriteString("mcp_servers:\n")
		b.WriteString(mcp)
	}
	return b.String()
}

func renderMCPServers(servers []helios.MCPServerSpec) string {
	filtered := make([]helios.MCPServerSpec, 0, len(servers))
	for _, server := range servers {
		if server.Name != "" {
			filtered = append(filtered, server)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	var b strings.Builder
	for _, server := range filtered {
		switch server.Type {
		case "http", "sse":
			if server.URL == "" {
				continue
			}
			b.WriteString("  ")
			b.WriteString(quoteKey(server.Name))
			b.WriteString(":\n    enabled: true\n    url: ")
			b.WriteString(quote(server.URL))
			b.WriteByte('\n')
			writeStringMap(&b, "headers", server.Headers, 4)
		case "stdio":
			if server.Command == "" {
				continue
			}
			b.WriteString("  ")
			b.WriteString(quoteKey(server.Name))
			b.WriteString(":\n    enabled: true\n    command: ")
			b.WriteString(quote(server.Command))
			b.WriteByte('\n')
			if len(server.Args) > 0 {
				b.WriteString("    args:\n")
				for _, arg := range server.Args {
					b.WriteString("      - ")
					b.WriteString(quote(arg))
					b.WriteByte('\n')
				}
			}
			writeStringMap(&b, "env", server.Env, 4)
		}
	}
	return b.String()
}

func writeStringMap(b *strings.Builder, name string, values map[string]string, indent int) {
	if len(values) == 0 {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	spaces := strings.Repeat(" ", indent)
	b.WriteString(spaces)
	b.WriteString(name)
	b.WriteString(":\n")
	for _, key := range keys {
		b.WriteString(spaces)
		b.WriteString("  ")
		b.WriteString(quoteKey(key))
		b.WriteString(": ")
		b.WriteString(quote(values[key]))
		b.WriteByte('\n')
	}
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
