package runtime

import (
	"context"

	"github.com/colink-ai/helios/contracts"
)

// SessionStatus reports adapter-level session state.
type SessionStatus string

const (
	SessionIdle      SessionStatus = "idle"
	SessionStarting  SessionStatus = "starting"
	SessionRunning   SessionStatus = "running"
	SessionPaused    SessionStatus = "paused"
	SessionCompleted SessionStatus = "completed"
	SessionFailed    SessionStatus = "failed"
	SessionStopped   SessionStatus = "stopped"
)

// ChunkHandler receives normalized streaming chunks.
type ChunkHandler func(contracts.Chunk)

// Adapter is the core runtime adapter contract implemented by agent backends.
type Adapter interface {
	StartSession(ctx context.Context, req SessionRequest) (*SessionHandle, error)
	Prompt(ctx context.Context, req PromptRequest, onChunk ChunkHandler) (*RunResult, error)
	StopSession(ctx context.Context, sessionID string) error
	GetSessionStatus(ctx context.Context, sessionID string) (SessionStatus, error)
	CheckHealth(ctx context.Context, spec AgentSpec) error
}

// RunAdapter is implemented by adapters with a native one-shot execution mode.
type RunAdapter interface {
	Run(ctx context.Context, req RunRequest, onChunk ChunkHandler) (*RunResult, error)
}

// CapabilityDetector is implemented by adapters that can inspect an installed
// agent runtime and report its protocol capabilities.
type CapabilityDetector interface {
	DetectCapabilities(ctx context.Context, spec AgentSpec) (Capabilities, error)
}

// ToolResultSender is implemented by adapters that can resume blocked tool or
// elicitation calls with user-provided results.
type ToolResultSender interface {
	SendToolResult(ctx context.Context, sessionID string, toolCallID string, result string) error
}

// PermissionDecision is the host application's answer to a runtime permission
// request.
type PermissionDecision struct {
	Allow    bool           `json:"allow"`
	Reason   string         `json:"reason,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PermissionResultSender is implemented by adapters that can continue a
// blocked permission request with a host decision.
type PermissionResultSender interface {
	SendPermissionResult(ctx context.Context, sessionID string, permissionID string, decision PermissionDecision) error
}

// SessionInspector exposes implementation-specific resume metadata.
type SessionInspector interface {
	AgentSessionID(ctx context.Context, sessionID string) (string, error)
	UsedNativeResume(ctx context.Context, sessionID string) (bool, error)
}

// SessionDiagnostics is a runtime-readable view of adapter health details.
type SessionDiagnostics struct {
	SessionID      string         `json:"sessionId"`
	AgentSessionID string         `json:"agentSessionId,omitempty"`
	Status         SessionStatus  `json:"status"`
	Stderr         string         `json:"stderr,omitempty"`
	TransportError string         `json:"transportError,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// DiagnosticProvider is implemented by adapters that expose session diagnostics.
type DiagnosticProvider interface {
	Diagnostics(ctx context.Context, sessionID string) (SessionDiagnostics, error)
}

// SessionRuntimeEvent is emitted by adapters for out-of-band runtime changes
// such as process exits.
type SessionRuntimeEvent struct {
	SessionID string         `json:"sessionId"`
	Type      string         `json:"type"`
	Error     string         `json:"error,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// SessionEventSource is implemented by adapters that can push runtime events
// independent of Prompt streaming.
type SessionEventSource interface {
	SessionEvents(ctx context.Context, sessionID string) (<-chan SessionRuntimeEvent, error)
}

// SessionHandle is returned after a session has been created.
type SessionHandle struct {
	ID             string         `json:"id"`
	RunID          string         `json:"runId,omitempty"`
	AgentID        string         `json:"agentId,omitempty"`
	AgentSessionID string         `json:"agentSessionId,omitempty"`
	Status         SessionStatus  `json:"status"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}
