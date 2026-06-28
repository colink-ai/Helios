package runtime

import (
	"context"
	"testing"
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
