package contracts

import "time"

// HandoffTarget describes where control should move next.
type HandoffTarget struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Handoff is a runtime request to move work to another agent, human, or system.
type Handoff struct {
	ID        string         `json:"id,omitempty"`
	RunID     string         `json:"runId,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
	Reason    string         `json:"reason,omitempty"`
	Target    HandoffTarget  `json:"target"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt time.Time      `json:"createdAt,omitempty"`
}
