package runtime

// Capabilities describes the stable feature surface a host application can
// expect from an agent runtime.
type Capabilities struct {
	AgentType          string         `json:"agentType,omitempty"`
	AgentName          string         `json:"agentName,omitempty"`
	Protocol           string         `json:"protocol,omitempty"`
	ResidentSessions   bool           `json:"residentSessions,omitempty"`
	OneShotRuns        bool           `json:"oneShotRuns,omitempty"`
	NativeResume       bool           `json:"nativeResume,omitempty"`
	SessionLoad        bool           `json:"sessionLoad,omitempty"`
	MCPServers         bool           `json:"mcpServers,omitempty"`
	Questions          bool           `json:"questions,omitempty"`
	ToolResults        bool           `json:"toolResults,omitempty"`
	Usage              bool           `json:"usage,omitempty"`
	Plans              bool           `json:"plans,omitempty"`
	Artifacts          bool           `json:"artifacts,omitempty"`
	Handoffs           bool           `json:"handoffs,omitempty"`
	PermissionRequests bool           `json:"permissionRequests,omitempty"`
	Multimodal         bool           `json:"multimodal,omitempty"`
	Raw                map[string]any `json:"raw,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// StaticCapabilities returns conservative SDK-level capabilities for adapters
// that do not provide runtime probing.
func StaticCapabilities(spec AgentSpec, adapter Adapter) Capabilities {
	_, oneShot := adapter.(RunAdapter)
	return Capabilities{
		AgentType:        spec.Type,
		AgentName:        spec.Name,
		ResidentSessions: true,
		OneShotRuns:      oneShot,
		Multimodal:       spec.SupportsMultimodal,
	}
}
