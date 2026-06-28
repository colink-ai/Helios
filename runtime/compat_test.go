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

func TestCompatibilityHarnessResumeAndElicitationScenarios(t *testing.T) {
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
	report := harness.Run(context.Background(), AgentSpec{Type: "chunking"}, []CompatibilityCheck{
		{Scenario: CompatResume, ResumeSessionID: "agent-session-1", Input: "resume"},
		{Scenario: CompatElicitation, Input: "ask"},
		{Scenario: CompatCapabilities},
	})
	if len(report.Results) != 3 {
		t.Fatalf("unexpected report: %+v", report)
	}
	for _, result := range report.Results {
		if !result.Passed {
			t.Fatalf("scenario %s failed: %+v", result.Scenario, result)
		}
	}
	if report.Results[2].Capabilities == nil {
		t.Fatalf("capabilities scenario should attach capabilities: %+v", report.Results[2])
	}
}

func TestCompatibilityHarnessUnknownScenario(t *testing.T) {
	harness := NewCompatibilityHarness(NewEngine(NewRegistry()))
	report := harness.Run(context.Background(), AgentSpec{Type: "missing"}, []CompatibilityCheck{{Scenario: "weird"}})
	if len(report.Results) != 1 || report.Results[0].Passed || report.Results[0].Error == "" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestCompatibilityHarnessReportsFailures(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "prompt-fail",
		Factory: func(AgentSpec) (Adapter, error) {
			return failingPromptAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register prompt fail: %v", err)
	}
	if err := reg.Register(AdapterMeta{
		Type: "detect-fail",
		Factory: func(AgentSpec) (Adapter, error) {
			return failingDetector{}, nil
		},
	}); err != nil {
		t.Fatalf("register detect fail: %v", err)
	}
	harness := NewCompatibilityHarness(NewEngine(reg))
	report := harness.Run(context.Background(), AgentSpec{Type: "prompt-fail"}, []CompatibilityCheck{
		{Scenario: CompatOneShot, Input: "hello"},
		{Scenario: CompatResident, Input: "hello"},
	})
	if len(report.Results) != 2 || report.Results[0].Passed || report.Results[1].Passed {
		t.Fatalf("prompt failures should be reported: %+v", report)
	}
	detectReport := harness.Run(context.Background(), AgentSpec{Type: "detect-fail"}, []CompatibilityCheck{{Scenario: CompatDetect}})
	if len(detectReport.Results) != 1 || detectReport.Results[0].Passed || detectReport.Results[0].Error == "" {
		t.Fatalf("detect failure should be reported: %+v", detectReport)
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

func TestCompatibilityHarnessDoesNotReuseEngineStoreByDefault(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	store := NewMemorySessionStore()
	harness := NewCompatibilityHarness(NewEngine(reg, WithSessionStore(store)))
	report := harness.Run(context.Background(), AgentSpec{Type: "test"}, []CompatibilityCheck{{Scenario: CompatResident, Input: "hello"}})
	if len(report.Results) != 1 || !report.Results[0].Passed {
		t.Fatalf("unexpected report: %+v", report)
	}
	snapshot, err := store.LoadSession(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot != nil {
		t.Fatalf("harness should not write to engine store by default: %+v", snapshot)
	}
}

func TestCompatibilityHarnessExplicitStore(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	store := &countingStore{SessionStore: NewMemorySessionStore()}
	harness := NewCompatibilityHarness(NewEngine(reg)).WithSessionStore(store)
	report := harness.Run(context.Background(), AgentSpec{Type: "test"}, []CompatibilityCheck{{Scenario: CompatResident, Input: "hello"}})
	if len(report.Results) != 1 || !report.Results[0].Passed {
		t.Fatalf("unexpected report: %+v", report)
	}
	if store.saved != 1 || store.deleted != 1 {
		t.Fatalf("explicit harness store should be used, saved=%d deleted=%d", store.saved, store.deleted)
	}
}

type countingStore struct {
	SessionStore
	saved   int
	deleted int
}

func (s *countingStore) SaveSession(ctx context.Context, snapshot SessionSnapshot) error {
	s.saved++
	return s.SessionStore.SaveSession(ctx, snapshot)
}

func (s *countingStore) DeleteSession(ctx context.Context, sessionID string) error {
	s.deleted++
	return s.SessionStore.DeleteSession(ctx, sessionID)
}
