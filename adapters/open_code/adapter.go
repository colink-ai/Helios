package open_code

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/colink-ai/helios/adapters/acp"
	helios "github.com/colink-ai/helios/runtime"
)

const (
	Type       = "open_code"
	providerID = "helios"
)

type Option func(*config)

type config struct {
	cliPath        string
	port           int
	permissionMode string
}

type Adapter struct {
	base     *acp.BaseAdapter
	cfg      config
	mu       sync.Mutex
	sessions map[string]sessionHTTP
}

type sessionHTTP struct {
	port int
	cwd  string
}

func WithCLIPath(path string) Option {
	return func(c *config) { c.cliPath = path }
}

func WithHTTPPort(port int) Option {
	return func(c *config) { c.port = port }
}

func WithPermissionMode(mode string) Option {
	return func(c *config) { c.permissionMode = mode }
}

func NewAdapter(opts ...Option) helios.Adapter {
	cfg := config{cliPath: "opencode"}
	for _, opt := range opts {
		opt(&cfg)
	}
	adapter := &Adapter{cfg: cfg, sessions: map[string]sessionHTTP{}}
	adapter.base = acp.NewBaseAdapter(acp.Config{
		CLIPath:   cfg.cliPath,
		BuildArgs: adapter.buildArgs,
		BuildEnv: func(req helios.SessionRequest) []string {
			return buildEnv(req, cfg)
		},
	})
	return adapter
}

func (a *Adapter) StartSession(ctx context.Context, req helios.SessionRequest) (*helios.SessionHandle, error) {
	if req.SessionID == "" {
		req.SessionID = helios.NewID("session")
	}
	handle, err := a.base.StartSession(ctx, req)
	if err != nil {
		a.forget(req.SessionID)
		return nil, err
	}
	if handle != nil && handle.ID != req.SessionID {
		a.move(req.SessionID, handle.ID)
	}
	return handle, nil
}

func (a *Adapter) Prompt(ctx context.Context, req helios.PromptRequest, onChunk helios.ChunkHandler) (*helios.RunResult, error) {
	return a.base.Prompt(ctx, req, onChunk)
}

func (a *Adapter) StopSession(ctx context.Context, sessionID string) error {
	defer a.forget(sessionID)
	return a.base.StopSession(ctx, sessionID)
}

func (a *Adapter) GetSessionStatus(ctx context.Context, sessionID string) (helios.SessionStatus, error) {
	return a.base.GetSessionStatus(ctx, sessionID)
}

func (a *Adapter) CheckHealth(ctx context.Context, spec helios.AgentSpec) error {
	handle, err := a.StartSession(ctx, helios.SessionRequest{SessionID: helios.NewID("health"), Agent: spec})
	if err != nil {
		return err
	}
	return a.StopSession(ctx, handle.ID)
}

func (a *Adapter) Run(ctx context.Context, req helios.RunRequest, onChunk helios.ChunkHandler) (*helios.RunResult, error) {
	sessionID := helios.NewID("session")
	handle, err := a.StartSession(ctx, helios.SessionRequest{
		RunID:             req.RunID,
		SessionID:         sessionID,
		Agent:             req.Agent,
		WorkDir:           req.WorkDir,
		RuntimeConfigMode: req.RuntimeConfigMode,
		RuntimeHome:       req.RuntimeHome,
		ConfigDir:         req.ConfigDir,
		MCPServers:        req.MCPServers,
		Metadata:          req.Metadata,
	})
	if err != nil {
		return nil, err
	}
	result, promptErr := a.Prompt(ctx, helios.PromptRequest{
		SessionID: handle.ID,
		Input:     req.Input,
		Images:    req.Images,
		Metadata:  req.Metadata,
	}, onChunk)
	stopErr := a.StopSession(ctx, handle.ID)
	if promptErr != nil {
		return nil, promptErr
	}
	if stopErr != nil {
		return nil, stopErr
	}
	if result != nil {
		result.RunID = req.RunID
	}
	return result, nil
}

func (a *Adapter) DetectCapabilities(ctx context.Context, spec helios.AgentSpec) (helios.Capabilities, error) {
	return a.base.DetectCapabilities(ctx, spec)
}

func (a *Adapter) SendToolResult(ctx context.Context, sessionID string, toolCallID string, result string) error {
	info, ok := a.session(sessionID)
	var httpErr error
	if ok && info.port > 0 && info.cwd != "" && strings.HasPrefix(strings.TrimSpace(result), "{") {
		if err := openCodeQuestionReply(ctx, info.cwd, toolCallID, result, info.port); err == nil {
			return nil
		} else {
			httpErr = err
		}
	}
	if err := a.base.SendToolResult(ctx, sessionID, toolCallID, result); err != nil {
		if httpErr != nil {
			return fmt.Errorf("opencode HTTP question reply failed: %v; ACP fallback failed: %w", httpErr, err)
		}
		return err
	}
	return nil
}

func (a *Adapter) SendPermissionResult(ctx context.Context, sessionID string, permissionID string, decision helios.PermissionDecision) error {
	return a.base.SendPermissionResult(ctx, sessionID, permissionID, decision)
}

func (a *Adapter) PendingRequests(ctx context.Context, sessionID string) ([]helios.PendingRequest, error) {
	return a.base.PendingRequests(ctx, sessionID)
}

func (a *Adapter) CancelPendingRequest(ctx context.Context, sessionID string, requestID string, reason string) error {
	return a.base.CancelPendingRequest(ctx, sessionID, requestID, reason)
}

func (a *Adapter) AgentSessionID(ctx context.Context, sessionID string) (string, error) {
	return a.base.AgentSessionID(ctx, sessionID)
}

func (a *Adapter) UsedNativeResume(ctx context.Context, sessionID string) (bool, error) {
	return a.base.UsedNativeResume(ctx, sessionID)
}

func (a *Adapter) Diagnostics(ctx context.Context, sessionID string) (helios.SessionDiagnostics, error) {
	return a.base.Diagnostics(ctx, sessionID)
}

func (a *Adapter) SessionEvents(ctx context.Context, sessionID string) (<-chan helios.SessionRuntimeEvent, error) {
	return a.base.SessionEvents(ctx, sessionID)
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
			if port, ok := metadataInt(spec.Metadata, "httpPort"); ok {
				localOpts = append(localOpts, WithHTTPPort(port))
			}
			if mode, ok := metadataString(spec.Metadata, "permission"); ok {
				localOpts = append(localOpts, WithPermissionMode(mode))
			}
			return NewAdapter(localOpts...), nil
		},
	})
}

func (a *Adapter) buildArgs(req helios.SessionRequest) []string {
	args := []string{"acp"}
	port := a.cfg.port
	if port <= 0 {
		var err error
		port, err = findFreePort()
		if err != nil {
			return args
		}
	}
	args = append(args, "--port", strconv.Itoa(port))
	if req.SessionID != "" {
		a.mu.Lock()
		a.sessions[req.SessionID] = sessionHTTP{port: port, cwd: effectiveCWD(req)}
		a.mu.Unlock()
	}
	return args
}

func (a *Adapter) session(sessionID string) (sessionHTTP, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	info, ok := a.sessions[sessionID]
	return info, ok
}

func (a *Adapter) forget(sessionID string) {
	a.mu.Lock()
	delete(a.sessions, sessionID)
	a.mu.Unlock()
}

func (a *Adapter) move(from string, to string) {
	if from == "" || to == "" || from == to {
		return
	}
	a.mu.Lock()
	if info, ok := a.sessions[from]; ok {
		delete(a.sessions, from)
		a.sessions[to] = info
	}
	a.mu.Unlock()
}

func buildEnv(req helios.SessionRequest, cfg config) []string {
	env := []string{"OPENCODE_PURE=1", "OPENCODE_ENABLE_QUESTION_TOOL=1"}
	configDir := configDir(req)
	if configDir != "" {
		_ = os.MkdirAll(configDir, 0o755)
		env = append(env, "OPENCODE_CONFIG_DIR="+configDir)
	}
	if content := buildConfigContent(req.Agent, cfg.permissionMode); content != "" {
		env = append(env, "OPENCODE_CONFIG_CONTENT="+content)
	}
	return env
}

func configDir(req helios.SessionRequest) string {
	if helios.EffectiveRuntimeConfigMode(req) == helios.RuntimeConfigUser {
		return ""
	}
	if configDir := helios.EffectiveConfigDir(req); configDir != "" {
		return configDir
	}
	if home := helios.EffectiveRuntimeHome(req); home != "" {
		return filepath.Join(home, "opencode")
	}
	if workDir := helios.EffectiveWorkDir(req); workDir != "" {
		return filepath.Join(workDir, ".opencode")
	}
	return ""
}

func effectiveCWD(req helios.SessionRequest) string {
	cwd := helios.EffectiveWorkDir(req)
	if cwd == "" {
		cwd = "."
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		return abs
	}
	return cwd
}

func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener addr %T", listener.Addr())
	}
	return addr.Port, nil
}

func openCodeQuestionReply(ctx context.Context, cwd string, toolCallID string, jsonAnswer string, port int) error {
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	listReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/question?directory="+url.QueryEscape(cwd), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		return fmt.Errorf("GET /question: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET /question returned %d: %s", resp.StatusCode, string(body))
	}
	var pending []struct {
		ID   string `json:"id"`
		Tool *struct {
			CallID string `json:"callID"`
		} `json:"tool"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pending); err != nil {
		return fmt.Errorf("parse /question response: %w", err)
	}
	requestID := ""
	for _, item := range pending {
		if item.Tool != nil && item.Tool.CallID == toolCallID {
			requestID = item.ID
			break
		}
	}
	if requestID == "" {
		return fmt.Errorf("no pending question found with tool.callID=%s", toolCallID)
	}
	answers, err := openCodeReplyAnswers(jsonAnswer)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"answers": answers})
	if err != nil {
		return err
	}
	replyReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/question/"+url.PathEscape(requestID)+"/reply?directory="+url.QueryEscape(cwd), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	replyReq.Header.Set("Content-Type", "application/json")
	replyResp, err := http.DefaultClient.Do(replyReq)
	if err != nil {
		return fmt.Errorf("POST /question/%s/reply: %w", requestID, err)
	}
	defer replyResp.Body.Close()
	if replyResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(replyResp.Body, 512))
		return fmt.Errorf("POST /question/%s/reply returned %d: %s", requestID, replyResp.StatusCode, string(body))
	}
	return nil
}

func openCodeReplyAnswers(jsonAnswer string) ([][]string, error) {
	values := map[string]any{}
	if err := json.Unmarshal([]byte(jsonAnswer), &values); err != nil {
		return nil, fmt.Errorf("parse question answer: %w", err)
	}
	out := [][]string{}
	for i := 0; ; i++ {
		value, ok := values[fmt.Sprintf("question_%d", i)]
		if !ok {
			break
		}
		switch typed := value.(type) {
		case string:
			out = append(out, []string{typed})
		case []any:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok {
					items = append(items, text)
				}
			}
			out = append(out, items)
		case []string:
			out = append(out, append([]string(nil), typed...))
		default:
			return nil, fmt.Errorf("question_%d has unsupported answer type %T", i, value)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no question answers found")
	}
	return out, nil
}

func buildConfigContent(spec helios.AgentSpec, permissionMode string) string {
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
	}
	if permissionMode != "" {
		cfg.Permission = permissionMode
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
