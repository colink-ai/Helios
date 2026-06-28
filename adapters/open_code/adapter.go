package open_code

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"github.com/colink-ai/helios/adapters/acp"
	helios "github.com/colink-ai/helios/runtime"
)

const (
	Type       = "open_code"
	providerID = "helios"
)

type Option func(*config)

type config struct {
	cliPath string
	port    int
}

func WithCLIPath(path string) Option {
	return func(c *config) { c.cliPath = path }
}

func WithHTTPPort(port int) Option {
	return func(c *config) { c.port = port }
}

func NewAdapter(opts ...Option) helios.Adapter {
	cfg := config{cliPath: "opencode"}
	for _, opt := range opts {
		opt(&cfg)
	}
	return acp.NewBaseAdapter(acp.Config{
		CLIPath: cfg.cliPath,
		BuildArgs: func(helios.SessionRequest) []string {
			args := []string{"acp"}
			if cfg.port > 0 {
				args = append(args, "--port", strconv.Itoa(cfg.port))
			}
			return args
		},
		BuildEnv: func(req helios.SessionRequest) []string {
			return buildEnv(req)
		},
	})
}

func Register(registry *helios.Registry, opts ...Option) error {
	return registry.Register(helios.AdapterMeta{
		Type:        Type,
		Name:        "OpenCode",
		Description: "OpenCode ACP adapter",
		DefaultPath: "opencode",
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
	env := []string{"OPENCODE_PURE=1", "OPENCODE_ENABLE_QUESTION_TOOL=1"}
	configDir := configDir(req)
	if configDir != "" {
		_ = os.MkdirAll(configDir, 0o755)
		env = append(env, "OPENCODE_CONFIG_DIR="+configDir)
	}
	if content := buildConfigContent(req.Agent); content != "" {
		env = append(env, "OPENCODE_CONFIG_CONTENT="+content)
	}
	return env
}

func configDir(req helios.SessionRequest) string {
	if req.RuntimeHome != "" {
		return filepath.Join(req.RuntimeHome, "opencode")
	}
	if req.Agent.RuntimeHome != "" {
		return filepath.Join(req.Agent.RuntimeHome, "opencode")
	}
	if req.WorkDir != "" {
		return filepath.Join(req.WorkDir, ".opencode")
	}
	if req.Agent.WorkDir != "" {
		return filepath.Join(req.Agent.WorkDir, ".opencode")
	}
	return ""
}

func buildConfigContent(spec helios.AgentSpec) string {
	if spec.APIURL == "" && spec.APIToken == "" && spec.DefaultModel == "" {
		return ""
	}
	cfg := openCodeConfig{
		Provider: map[string]openCodeProvider{
			providerID: {
				Name: "Helios Provider",
				Npm:  "@ai-sdk/openai-compatible",
				Options: openCodeProviderOptions{
					APIKey:  spec.APIToken,
					BaseURL: spec.APIURL,
				},
			},
		},
		Permission: "allow",
	}
	if spec.DefaultModel != "" {
		provider := cfg.Provider[providerID]
		provider.Models = map[string]openCodeModel{
			spec.DefaultModel: {
				ID:         spec.DefaultModel,
				Name:       spec.DefaultModel,
				Attachment: true,
				Modalities: &openCodeModalities{
					Input:  []string{"text", "image"},
					Output: []string{"text"},
				},
			},
		}
		cfg.Provider[providerID] = provider
		cfg.Model = providerID + "/" + spec.DefaultModel
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(data)
}

type openCodeConfig struct {
	Provider   map[string]openCodeProvider `json:"provider,omitempty"`
	Model      string                      `json:"model,omitempty"`
	Permission any                         `json:"permission,omitempty"`
}

type openCodeProvider struct {
	Name    string                   `json:"name,omitempty"`
	Npm     string                   `json:"npm,omitempty"`
	Options openCodeProviderOptions  `json:"options,omitempty"`
	Models  map[string]openCodeModel `json:"models,omitempty"`
}

type openCodeProviderOptions struct {
	APIKey  string `json:"apiKey,omitempty"`
	BaseURL string `json:"baseURL,omitempty"`
}

type openCodeModel struct {
	ID         string              `json:"id,omitempty"`
	Name       string              `json:"name,omitempty"`
	Attachment bool                `json:"attachment,omitempty"`
	Modalities *openCodeModalities `json:"modalities,omitempty"`
}

type openCodeModalities struct {
	Input  []string `json:"input,omitempty"`
	Output []string `json:"output,omitempty"`
}
