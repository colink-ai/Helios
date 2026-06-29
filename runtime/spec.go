package runtime

import (
	"time"

	"github.com/colink-ai/helios/contracts"
)

// AgentSpec is the runtime-ready agent configuration supplied by a host app.
type AgentSpec struct {
	ID                 string            `json:"id,omitempty"`
	Type               string            `json:"type"`
	Name               string            `json:"name,omitempty"`
	CLIPath            string            `json:"cliPath,omitempty"`
	DefaultModel       string            `json:"defaultModel,omitempty"`
	APIURL             string            `json:"apiUrl,omitempty"`
	APIToken           string            `json:"apiToken,omitempty"`
	RuntimeConfigMode  RuntimeConfigMode `json:"runtimeConfigMode,omitempty"`
	RuntimeHome        string            `json:"runtimeHome,omitempty"`
	WorkDir            string            `json:"workDir,omitempty"`
	SystemPrompt       string            `json:"systemPrompt,omitempty"`
	SupportsMultimodal bool              `json:"supportsMultimodal,omitempty"`
	PromptTimeout      time.Duration     `json:"promptTimeout,omitempty"`
	Env                map[string]string `json:"env,omitempty"`
	Metadata           map[string]any    `json:"metadata,omitempty"`
}

// MCPServerSpec describes a tool server made available to a runtime session.
type MCPServerSpec struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// SessionRequest starts or resumes an interactive agent session.
type SessionRequest struct {
	RunID             string            `json:"runId,omitempty"`
	SessionID         string            `json:"sessionId,omitempty"`
	Agent             AgentSpec         `json:"agent"`
	WorkDir           string            `json:"workDir,omitempty"`
	RuntimeConfigMode RuntimeConfigMode `json:"runtimeConfigMode,omitempty"`
	RuntimeHome       string            `json:"runtimeHome,omitempty"`
	MCPServers        []MCPServerSpec   `json:"mcpServers,omitempty"`
	ResumeSessionID   string            `json:"resumeSessionId,omitempty"`
	Metadata          map[string]any    `json:"metadata,omitempty"`
}

// PromptRequest sends one prompt to an existing session.
type PromptRequest struct {
	SessionID string                   `json:"sessionId"`
	Input     string                   `json:"input"`
	Images    []contracts.ImageContent `json:"images,omitempty"`
	Metadata  map[string]any           `json:"metadata,omitempty"`
}

// RunRequest executes a one-shot run. Adapters may implement it directly or the
// runtime may emulate it with a short-lived session.
type RunRequest struct {
	RunID             string                   `json:"runId,omitempty"`
	Agent             AgentSpec                `json:"agent"`
	Input             string                   `json:"input"`
	Images            []contracts.ImageContent `json:"images,omitempty"`
	WorkDir           string                   `json:"workDir,omitempty"`
	RuntimeConfigMode RuntimeConfigMode        `json:"runtimeConfigMode,omitempty"`
	RuntimeHome       string                   `json:"runtimeHome,omitempty"`
	MCPServers        []MCPServerSpec          `json:"mcpServers,omitempty"`
	Metadata          map[string]any           `json:"metadata,omitempty"`
}

// RunResult is the normalized final result from a one-shot run or prompt.
type RunResult struct {
	Output         string                `json:"output,omitempty"`
	RunID          string                `json:"runId,omitempty"`
	SessionID      string                `json:"sessionId,omitempty"`
	AgentSessionID string                `json:"agentSessionId,omitempty"`
	Artifacts      []contracts.Artifact  `json:"artifacts,omitempty"`
	Usage          *contracts.TokenUsage `json:"usage,omitempty"`
	Metadata       map[string]any        `json:"metadata,omitempty"`
}
