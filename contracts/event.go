package contracts

import "time"

// EventType is a normalized runtime event category.
type EventType string

const (
	EventRunStarted      EventType = "run.started"
	EventRunCompleted    EventType = "run.completed"
	EventRunFailed       EventType = "run.failed"
	EventSessionStarted  EventType = "session.started"
	EventSessionStopped  EventType = "session.stopped"
	EventChunk           EventType = "chunk"
	EventToolStarted     EventType = "tool.started"
	EventToolInputDelta  EventType = "tool.input_delta"
	EventToolCompleted   EventType = "tool.completed"
	EventToolFailed      EventType = "tool.failed"
	EventQuestionAsked   EventType = "question.requested"
	EventPermissionAsked EventType = "permission.requested"
	EventPlanUpdated     EventType = "plan.updated"
	EventArtifactCreated EventType = "artifact.created"
	EventHandoffCreated  EventType = "handoff.created"
	EventUsageReported   EventType = "usage.reported"
	EventRuntimeError    EventType = "runtime.error"
)

// RunEvent is the stable event envelope applications can persist or forward.
type RunEvent struct {
	ID        string         `json:"id,omitempty"`
	Type      EventType      `json:"type"`
	RunID     string         `json:"runId,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
	AgentID   string         `json:"agentId,omitempty"`
	TeamID    string         `json:"teamId,omitempty"`
	Sequence  int64          `json:"sequence,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Chunk     *Chunk         `json:"chunk,omitempty"`
	Artifact  *Artifact      `json:"artifact,omitempty"`
	Handoff   *Handoff       `json:"handoff,omitempty"`
	Usage     *TokenUsage    `json:"usage,omitempty"`
	Error     string         `json:"error,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// NewEvent creates a timestamped runtime event.
func NewEvent(eventType EventType) RunEvent {
	return RunEvent{Type: eventType, Timestamp: time.Now().UTC()}
}
