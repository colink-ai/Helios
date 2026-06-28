package runtime

import (
	"context"
	"testing"

	"github.com/colink-ai/helios/contracts"
)

func TestCompatibilityHarnessRun(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return detectingAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	harness := NewCompatibilityHarness(NewEngine(reg))
	report := harness.Run(context.Background(), AgentSpec{Type: "test", Name: "Test"}, []CompatibilityCheck{
		{Scenario: CompatDetect},
		{Scenario: CompatOneShot, Input: "hello"},
		{Scenario: CompatResident, Input: "hello"},
	})
	if report.AgentType != "test" || len(report.Results) != 3 {
		t.Fatalf("unexpected report: %+v", report)
	}
	for _, result := range report.Results {
		if !result.Passed {
			t.Fatalf("scenario %s failed: %+v", result.Scenario, result)
		}
	}
}

func TestCompatibilityHarnessUnknownScenario(t *testing.T) {
	harness := NewCompatibilityHarness(NewEngine(NewRegistry()))
	report := harness.Run(context.Background(), AgentSpec{Type: "missing"}, []CompatibilityCheck{{Scenario: "weird"}})
	if len(report.Results) != 1 || report.Results[0].Passed || report.Results[0].Error == "" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

type chunkingAdapter struct {
	testAdapter
}

func (chunkingAdapter) Prompt(_ context.Context, _ PromptRequest, onChunk ChunkHandler) (*RunResult, error) {
	onChunk(contracts.Chunk{Type: contracts.ChunkText, Content: "hello"})
	return &RunResult{Output: "hello"}, nil
}

func TestCompatibilityHarnessCapturesChunks(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "chunking",
		Factory: func(AgentSpec) (Adapter, error) {
			return chunkingAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	harness := NewCompatibilityHarness(NewEngine(reg))
	report := harness.Run(context.Background(), AgentSpec{Type: "chunking"}, []CompatibilityCheck{{
		Scenario:       CompatResident,
		Input:          "hello",
		WantChunkTypes: []contracts.ChunkType{contracts.ChunkText},
	}})
	if len(report.Results) != 1 || !report.Results[0].Passed || len(report.Results[0].Chunks) != 1 || report.Results[0].Duration == 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestCompatibilityHarnessFailsMissingChunk(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "chunking",
		Factory: func(AgentSpec) (Adapter, error) {
			return chunkingAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	harness := NewCompatibilityHarness(NewEngine(reg))
	report := harness.Run(context.Background(), AgentSpec{Type: "chunking"}, []CompatibilityCheck{{
		Scenario:       CompatResident,
		Input:          "hello",
		WantChunkTypes: []contracts.ChunkType{contracts.ChunkToolUse},
	}})
	if len(report.Results) != 1 || report.Results[0].Passed || report.Results[0].Error == "" {
		t.Fatalf("unexpected report: %+v", report)
	}
}
