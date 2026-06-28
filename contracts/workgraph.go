package contracts

// WorkGraph describes a runtime-level multi-agent plan without requiring a
// host application's workflow schema.
type WorkGraph struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name,omitempty"`
	Nodes    []WorkNode     `json:"nodes,omitempty"`
	Edges    []WorkEdge     `json:"edges,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// WorkNode is a unit of agent, tool, human, or system work.
type WorkNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Name     string         `json:"name,omitempty"`
	AgentID  string         `json:"agentId,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// WorkEdge connects two work nodes.
type WorkEdge struct {
	ID        string         `json:"id,omitempty"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Condition string         `json:"condition,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
