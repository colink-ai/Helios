package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/colink-ai/helios/contracts"
)

// CompatibilityScenario selects one runtime behavior to validate against an
// installed agent.
type CompatibilityScenario string

const (
	CompatDetect       CompatibilityScenario = "detect"
	CompatOneShot      CompatibilityScenario = "one_shot"
	CompatResident     CompatibilityScenario = "resident"
	CompatResume       CompatibilityScenario = "resume"
	CompatElicitation  CompatibilityScenario = "elicitation"
	CompatCapabilities CompatibilityScenario = "capabilities"
)

// CompatibilityCheck configures one compatibility probe.
type CompatibilityCheck struct {
	Scenario        CompatibilityScenario `json:"scenario"`
	Input           string                `json:"input,omitempty"`
	ResumeSessionID string                `json:"resumeSessionId,omitempty"`
	WantChunkTypes  []contracts.ChunkType `json:"wantChunkTypes,omitempty"`
	Timeout         time.Duration         `json:"timeout,omitempty"`
}

// CompatibilityResult is the outcome of one compatibility probe.
type CompatibilityResult struct {
	Scenario       CompatibilityScenario `json:"scenario"`
	Passed         bool                  `json:"passed"`
	Error          string                `json:"error,omitempty"`
	Output         string                `json:"output,omitempty"`
	AgentSessionID string                `json:"agentSessionId,omitempty"`
	Capabilities   *Capabilities         `json:"capabilities,omitempty"`
	Chunks         []contracts.Chunk     `json:"chunks,omitempty"`
	Duration       time.Duration         `json:"duration,omitempty"`
}

// CompatibilityReport summarizes an agent compatibility run.
type CompatibilityReport struct {
	AgentType string                `json:"agentType,omitempty"`
	AgentName string                `json:"agentName,omitempty"`
	Results   []CompatibilityResult `json:"results"`
}

// CompatibilityHarness runs SDK-level compatibility probes against an Engine.
type CompatibilityHarness struct {
	engine *Engine
}

func NewCompatibilityHarness(engine *Engine) *CompatibilityHarness {
	return &CompatibilityHarness{engine: engine}
}

func (h *CompatibilityHarness) Run(ctx context.Context, spec AgentSpec, checks []CompatibilityCheck) CompatibilityReport {
	report := CompatibilityReport{AgentType: spec.Type, AgentName: spec.Name}
	for _, check := range checks {
		report.Results = append(report.Results, h.runCheck(ctx, spec, check))
	}
	return report
}

func (h *CompatibilityHarness) runCheck(ctx context.Context, spec AgentSpec, check CompatibilityCheck) CompatibilityResult {
	started := time.Now()
	if check.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, check.Timeout)
		defer cancel()
	}
	result := CompatibilityResult{Scenario: check.Scenario}
	defer func() { result.Duration = time.Since(started) }()

	switch check.Scenario {
	case CompatDetect, CompatCapabilities:
		caps, err := h.engine.DetectCapabilities(ctx, spec)
		if err != nil {
			return failedCompat(result, err)
		}
		result.Passed = true
		result.Capabilities = &caps
		return result
	case CompatOneShot:
		run, chunks, err := h.runOneShot(ctx, spec, check)
		if err != nil {
			return failedCompat(result, err)
		}
		result.Output = run.Output
		result.Chunks = chunks
	case CompatResident, CompatResume, CompatElicitation:
		run, chunks, agentSessionID, err := h.runSession(ctx, spec, check)
		if err != nil {
			return failedCompat(result, err)
		}
		result.Output = run.Output
		result.Chunks = chunks
		result.AgentSessionID = agentSessionID
	default:
		return failedCompat(result, fmt.Errorf("unknown compatibility scenario: %s", check.Scenario))
	}
	if err := requireChunkTypes(result.Chunks, check.WantChunkTypes); err != nil {
		return failedCompat(result, err)
	}
	result.Passed = true
	return result
}

func (h *CompatibilityHarness) runOneShot(ctx context.Context, spec AgentSpec, check CompatibilityCheck) (*RunResult, []contracts.Chunk, error) {
	run, err := h.engine.Run(ctx, RunRequest{Agent: spec, Input: check.Input})
	return run, nil, err
}

func (h *CompatibilityHarness) runSession(ctx context.Context, spec AgentSpec, check CompatibilityCheck) (*RunResult, []contracts.Chunk, string, error) {
	handle, err := h.engine.StartSession(ctx, SessionRequest{
		Agent:           spec,
		ResumeSessionID: check.ResumeSessionID,
	})
	if err != nil {
		return nil, nil, "", err
	}
	defer h.engine.StopSession(context.Background(), handle.ID)
	chunks := []contracts.Chunk{}
	run, err := h.engine.Prompt(ctx, PromptRequest{SessionID: handle.ID, Input: check.Input})
	if err != nil {
		return nil, chunks, handle.AgentSessionID, err
	}
	return run, chunks, handle.AgentSessionID, nil
}

func requireChunkTypes(chunks []contracts.Chunk, want []contracts.ChunkType) error {
	if len(want) == 0 {
		return nil
	}
	seen := map[contracts.ChunkType]bool{}
	for _, chunk := range chunks {
		seen[chunk.Type] = true
	}
	for _, typ := range want {
		if !seen[typ] {
			return fmt.Errorf("missing chunk type %s", typ)
		}
	}
	return nil
}

func failedCompat(result CompatibilityResult, err error) CompatibilityResult {
	result.Passed = false
	if err != nil {
		result.Error = err.Error()
	}
	return result
}
