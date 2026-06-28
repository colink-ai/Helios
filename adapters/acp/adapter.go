package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

const (
	defaultStartupTimeout = 45 * time.Second
	defaultPromptTimeout  = 30 * time.Minute
)

type Config struct {
	CLIPath              string
	BuildArgs            func(helios.SessionRequest) []string
	BuildEnv             func(helios.SessionRequest) []string
	ConfigureModelViaACP bool
	StartupTimeout       time.Duration
	PromptTimeout        time.Duration
}

type BaseAdapter struct {
	config   Config
	sessions map[string]*session
	mu       sync.RWMutex
}

type session struct {
	id             string
	agentSessionID string
	cmd            *exec.Cmd
	cancel         context.CancelFunc
	transport      *transport
	status         helios.SessionStatus
	output         strings.Builder
	stderr         strings.Builder
	onChunk        helios.ChunkHandler
	pendingRequest any
	nativeResume   bool
	mu             sync.Mutex
}

func NewBaseAdapter(config Config) *BaseAdapter {
	return &BaseAdapter{config: config, sessions: map[string]*session{}}
}

func (a *BaseAdapter) StartSession(ctx context.Context, req helios.SessionRequest) (*helios.SessionHandle, error) {
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = helios.NewID("session")
	}
	a.mu.Lock()
	if _, exists := a.sessions[sessionID]; exists {
		a.mu.Unlock()
		return nil, fmt.Errorf("acp session already exists: %s", sessionID)
	}
	a.mu.Unlock()

	cliPath := a.cliPath(req.Agent)
	if cliPath == "" {
		return nil, fmt.Errorf("acp cli path is required")
	}
	startCtx, cancel := context.WithTimeout(ctx, a.startupTimeout())
	defer cancel()
	procCtx, procCancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(procCtx, cliPath, a.buildArgs(req)...)
	if req.WorkDir != "" {
		if err := os.MkdirAll(req.WorkDir, 0o755); err != nil {
			procCancel()
			return nil, err
		}
		if abs, err := filepath.Abs(req.WorkDir); err == nil {
			cmd.Dir = abs
		} else {
			cmd.Dir = req.WorkDir
		}
	}
	cmd.Env = a.buildEnv(req)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		procCancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		procCancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		procCancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		procCancel()
		return nil, err
	}

	s := &session{cmd: cmd, cancel: procCancel, status: helios.SessionStarting}
	go captureStderr(stderr, s)
	t := newTransport(stdout, stdin, func(id any, method string, params json.RawMessage) {
		a.handleRequest(s, id, method, params)
	}, func(method string, params json.RawMessage) {
		a.handleNotification(s, method, params)
	})
	s.transport = t
	t.start()

	fail := func(stage string, cause error) error {
		a.teardown(sessionID, s)
		return fmt.Errorf("acp %s failed: %w%s", stage, cause, s.stderrText())
	}

	initResult, err := t.sendRequest(startCtx, "initialize", InitializeParams{
		ProtocolVersion:    2025,
		ClientCapabilities: map[string]any{},
	})
	if err != nil {
		return nil, fail("initialize", err)
	}
	initResp := InitializeResult{}
	_ = json.Unmarshal(initResult, &initResp)

	mcpServers := ConvertMCPServers(req.MCPServers)
	var sessionResult json.RawMessage
	if req.ResumeSessionID != "" && supportsResume(initResp.AgentCapabilities) {
		sessionResult, err = t.sendRequest(startCtx, "session/resume", SessionParams{
			CWD:        req.WorkDir,
			SessionID:  req.ResumeSessionID,
			MCPServers: mcpServers,
		})
		s.agentSessionID = req.ResumeSessionID
		s.nativeResume = true
	} else {
		sessionResult, err = t.sendRequest(startCtx, "session/new", SessionParams{
			CWD:        req.WorkDir,
			MCPServers: mcpServers,
		})
	}
	if err != nil {
		return nil, fail("session start", err)
	}
	if s.agentSessionID == "" {
		s.agentSessionID = parseSessionID(sessionResult, sessionID)
	}
	s.id = sessionID
	s.status = helios.SessionRunning

	a.mu.Lock()
	a.sessions[sessionID] = s
	a.mu.Unlock()

	return &helios.SessionHandle{
		ID:             sessionID,
		RunID:          req.RunID,
		AgentID:        req.Agent.ID,
		AgentSessionID: s.agentSessionID,
		Status:         helios.SessionRunning,
		Metadata:       map[string]any{"nativeResume": s.nativeResume},
	}, nil
}

func (a *BaseAdapter) Prompt(ctx context.Context, req helios.PromptRequest, onChunk helios.ChunkHandler) (*helios.RunResult, error) {
	s, err := a.get(req.SessionID)
	if err != nil {
		return nil, err
	}
	timeout := a.promptTimeout()
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	promptCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		promptCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	s.mu.Lock()
	s.onChunk = onChunk
	s.output.Reset()
	agentSessionID := s.agentSessionID
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.onChunk = nil
		s.mu.Unlock()
	}()

	_, err = s.transport.sendRequest(promptCtx, "session/prompt", PromptParams{
		SessionID: agentSessionID,
		Prompt:    promptBlocks(req),
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	output := s.output.String()
	s.mu.Unlock()
	return &helios.RunResult{Output: output, SessionID: req.SessionID, AgentSessionID: agentSessionID}, nil
}

func (a *BaseAdapter) Run(ctx context.Context, req helios.RunRequest, onChunk helios.ChunkHandler) (*helios.RunResult, error) {
	sessionID := helios.NewID("session")
	handle, err := a.StartSession(ctx, helios.SessionRequest{
		RunID:       req.RunID,
		SessionID:   sessionID,
		Agent:       req.Agent,
		WorkDir:     req.WorkDir,
		RuntimeHome: req.RuntimeHome,
		MCPServers:  req.MCPServers,
		Metadata:    req.Metadata,
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

func (a *BaseAdapter) StopSession(ctx context.Context, sessionID string) error {
	s, err := a.get(sessionID)
	if err != nil {
		return err
	}
	if s.transport != nil && s.agentSessionID != "" {
		endCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, _ = s.transport.sendRequest(endCtx, "session/end", EndSessionParams{SessionID: s.agentSessionID})
		cancel()
	}
	a.teardown(sessionID, s)
	return nil
}

func (a *BaseAdapter) GetSessionStatus(_ context.Context, sessionID string) (helios.SessionStatus, error) {
	s, err := a.get(sessionID)
	if err != nil {
		return helios.SessionStopped, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status, nil
}

func (a *BaseAdapter) CheckHealth(ctx context.Context, spec helios.AgentSpec) error {
	handle, err := a.StartSession(ctx, helios.SessionRequest{SessionID: helios.NewID("health"), Agent: spec})
	if err != nil {
		return err
	}
	return a.StopSession(ctx, handle.ID)
}

func (a *BaseAdapter) SendToolResult(_ context.Context, sessionID string, _ string, result string) error {
	s, err := a.get(sessionID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pending := s.pendingRequest
	s.pendingRequest = nil
	s.mu.Unlock()
	if pending == nil {
		return fmt.Errorf("session %s has no pending tool result request", sessionID)
	}
	return s.transport.sendResponse(pending, map[string]any{
		"action":  "accept",
		"content": result,
	}, nil)
}

func (a *BaseAdapter) AgentSessionID(_ context.Context, sessionID string) (string, error) {
	s, err := a.get(sessionID)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agentSessionID, nil
}

func (a *BaseAdapter) UsedNativeResume(_ context.Context, sessionID string) (bool, error) {
	s, err := a.get(sessionID)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nativeResume, nil
}

func (a *BaseAdapter) get(sessionID string) (*session, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s := a.sessions[sessionID]
	if s == nil {
		return nil, fmt.Errorf("acp session not found: %s", sessionID)
	}
	return s, nil
}

func (a *BaseAdapter) teardown(sessionID string, s *session) {
	a.mu.Lock()
	delete(a.sessions, sessionID)
	a.mu.Unlock()
	s.mu.Lock()
	s.status = helios.SessionStopped
	s.mu.Unlock()
	if s.transport != nil {
		_ = s.transport.close()
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	}
}

func (a *BaseAdapter) handleNotification(s *session, method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	chunks, err := ParseSessionUpdate(params)
	if err != nil {
		return
	}
	for _, chunk := range chunks {
		s.mu.Lock()
		if chunk.Type == contracts.ChunkText {
			s.output.WriteString(chunk.Content)
		}
		cb := s.onChunk
		s.mu.Unlock()
		if cb != nil {
			cb(chunk)
		}
	}
}

func (a *BaseAdapter) handleRequest(s *session, id any, method string, _ json.RawMessage) {
	if method == "elicitation/create" {
		s.mu.Lock()
		s.pendingRequest = id
		cb := s.onChunk
		s.mu.Unlock()
		if cb != nil {
			cb(contracts.Chunk{Type: contracts.ChunkQuestion, ToolName: "AskUserQuestion"})
		}
		return
	}
	_ = s.transport.sendResponse(id, nil, &Error{Code: -32601, Message: "method not found"})
}

func (a *BaseAdapter) cliPath(spec helios.AgentSpec) string {
	if a.config.CLIPath != "" {
		return a.config.CLIPath
	}
	return spec.CLIPath
}

func (a *BaseAdapter) buildArgs(req helios.SessionRequest) []string {
	if a.config.BuildArgs != nil {
		return a.config.BuildArgs(req)
	}
	return nil
}

func (a *BaseAdapter) buildEnv(req helios.SessionRequest) []string {
	env := os.Environ()
	for key, value := range req.Agent.Env {
		env = append(env, key+"="+value)
	}
	if a.config.BuildEnv != nil {
		env = append(env, a.config.BuildEnv(req)...)
	}
	return env
}

func (a *BaseAdapter) startupTimeout() time.Duration {
	if a.config.StartupTimeout > 0 {
		return a.config.StartupTimeout
	}
	return defaultStartupTimeout
}

func (a *BaseAdapter) promptTimeout() time.Duration {
	if a.config.PromptTimeout != 0 {
		return a.config.PromptTimeout
	}
	return defaultPromptTimeout
}

func captureStderr(stderr ioReader, s *session) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		s.mu.Lock()
		if s.stderr.Len() < 64*1024 {
			s.stderr.WriteString(scanner.Text())
			s.stderr.WriteByte('\n')
		}
		s.mu.Unlock()
	}
}

type ioReader interface {
	Read([]byte) (int, error)
}

func (s *session) stderrText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stderr.Len() == 0 {
		return ""
	}
	return "\nstderr: " + s.stderr.String()
}

func promptBlocks(req helios.PromptRequest) []ContentBlock {
	blocks := []ContentBlock{{Type: "text", Text: req.Input}}
	for _, image := range req.Images {
		blocks = append(blocks, ContentBlock{Type: "image", MimeType: image.MimeType, Data: image.Data, URL: image.URL})
	}
	return blocks
}

func parseSessionID(raw json.RawMessage, fallback string) string {
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return fallback
	}
	for _, key := range []string{"sessionId", "id"} {
		if value, ok := values[key].(string); ok && value != "" {
			return value
		}
	}
	return fallback
}

func supportsResume(capabilities map[string]any) bool {
	if capabilities == nil {
		return false
	}
	if value, ok := capabilities["sessionResume"].(bool); ok {
		return value
	}
	if sessions, ok := capabilities["sessions"].(map[string]any); ok {
		if value, ok := sessions["resume"].(bool); ok {
			return value
		}
	}
	return false
}

func ConvertMCPServers(specs []helios.MCPServerSpec) []any {
	out := make([]any, 0, len(specs))
	for _, spec := range specs {
		if spec.Name == "" {
			continue
		}
		item := map[string]any{"name": spec.Name}
		switch spec.Type {
		case "http", "sse":
			if spec.URL == "" {
				continue
			}
			item["type"] = spec.Type
			item["url"] = spec.URL
			if len(spec.Headers) > 0 {
				item["headers"] = spec.Headers
			}
		case "stdio":
			if spec.Command == "" {
				continue
			}
			item["type"] = "stdio"
			item["command"] = spec.Command
			if len(spec.Args) > 0 {
				item["args"] = spec.Args
			}
			if len(spec.Env) > 0 {
				item["env"] = spec.Env
			}
		default:
			continue
		}
		out = append(out, item)
	}
	return out
}
