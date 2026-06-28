package contracts

import "time"

// A2AMessage is the runtime-level message exchanged between agents.
type A2AMessage struct {
	ID        string         `json:"id,omitempty"`
	RunID     string         `json:"runId,omitempty"`
	FromAgent string         `json:"fromAgent,omitempty"`
	ToAgent   string         `json:"toAgent,omitempty"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt,omitempty"`
}

// AgentTeam describes a group of agents that can collaborate on a run.
type AgentTeam struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name,omitempty"`
	Agents   []AgentRef     `json:"agents,omitempty"`
	Graph    *WorkGraph     `json:"graph,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AgentRef is a stable reference to an agent known by a host application.
type AgentRef struct {
	ID          string         `json:"id"`
	Name        string         `json:"name,omitempty"`
	Type        string         `json:"type,omitempty"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
