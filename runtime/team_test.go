package runtime

import (
	"context"
	"testing"

	"github.com/colink-ai/helios/contracts"
)

func TestTeamRunnerRunsAgentsInOrder(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	runner := NewTeamRunner(NewEngine(reg))
	result, err := runner.Run(context.Background(), TeamRunRequest{
		Team: contracts.AgentTeam{
			ID: "team-1",
			Agents: []contracts.AgentRef{
				{ID: "agent-a"},
				{ID: "agent-b"},
			},
		},
		Agents: map[string]AgentSpec{
			"agent-a": {Type: "test"},
			"agent-b": {Type: "test"},
		},
		Input: "hello",
	})
	if err != nil {
		t.Fatalf("run team: %v", err)
	}
	if result.Output != "ok" || len(result.Messages) != 2 || len(result.Results) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Messages[1].FromAgent != "agent-a" || result.Messages[1].ToAgent != "agent-b" {
		t.Fatalf("unexpected a2a messages: %+v", result.Messages)
	}
}

func TestOrderedAgentNodesUsesGraph(t *testing.T) {
	nodes := orderedAgentNodes(contracts.AgentTeam{Graph: &contracts.WorkGraph{Nodes: []contracts.WorkNode{
		{ID: "system", Type: "system"},
		{ID: "n1", Type: "agent", AgentID: "agent-1"},
	}}})
	if len(nodes) != 1 || nodes[0].ID != "n1" {
		t.Fatalf("unexpected nodes: %+v", nodes)
	}
}

func TestOrderedAgentNodesUsesEdges(t *testing.T) {
	nodes := orderedAgentNodes(contracts.AgentTeam{Graph: &contracts.WorkGraph{
		Nodes: []contracts.WorkNode{
			{ID: "n2", Type: "agent", AgentID: "agent-2"},
			{ID: "n1", Type: "agent", AgentID: "agent-1"},
		},
		Edges: []contracts.WorkEdge{{From: "n1", To: "n2"}},
	}})
	if len(nodes) != 2 || nodes[0].ID != "n1" || nodes[1].ID != "n2" {
		t.Fatalf("unexpected nodes: %+v", nodes)
	}
}

func TestTeamRunnerContinueOnError(t *testing.T) {
	runner := NewTeamRunner(NewEngine(NewRegistry()))
	result, err := runner.Run(context.Background(), TeamRunRequest{
		Team: contracts.AgentTeam{Agents: []contracts.AgentRef{{ID: "missing"}}},
		Agents: map[string]AgentSpec{
			"missing": {Type: "missing"},
		},
		Input:           "hello",
		ContinueOnError: true,
	})
	if err != nil {
		t.Fatalf("run team: %v", err)
	}
	if result.NodeErrors["missing"] == "" {
		t.Fatalf("expected node error: %+v", result)
	}
}

func TestTeamRunnerSkipsDisabledNode(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(AdapterMeta{
		Type: "test",
		Factory: func(AgentSpec) (Adapter, error) {
			return testAdapter{}, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	runner := NewTeamRunner(NewEngine(reg))
	result, err := runner.Run(context.Background(), TeamRunRequest{
		Team: contracts.AgentTeam{Graph: &contracts.WorkGraph{Nodes: []contracts.WorkNode{
			{ID: "n1", Type: "agent", AgentID: "agent-1", Metadata: map[string]any{"condition": "skip"}},
			{ID: "n2", Type: "agent", AgentID: "agent-2"},
		}}},
		Agents: map[string]AgentSpec{
			"agent-1": {Type: "test"},
			"agent-2": {Type: "test"},
		},
		Input: "hello",
	})
	if err != nil {
		t.Fatalf("run team: %v", err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "n1" || len(result.Results) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestShouldSkipNodeConditions(t *testing.T) {
	cases := []struct {
		name string
		meta map[string]any
		want bool
	}{
		{name: "nil", want: false},
		{name: "never", meta: map[string]any{"condition": "never"}, want: true},
		{name: "disabled", meta: map[string]any{"condition": "disabled"}, want: true},
		{name: "enabled", meta: map[string]any{"condition": "enabled"}, want: false},
		{name: "bool false", meta: map[string]any{"condition": false}, want: true},
		{name: "bool true", meta: map[string]any{"condition": true}, want: false},
		{name: "unknown", meta: map[string]any{"condition": 1}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipNode(contracts.WorkNode{Metadata: tc.meta}); got != tc.want {
				t.Fatalf("shouldSkipNode = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOrderNodesByEdgesFallsBackOnCycleAndUnknownEdges(t *testing.T) {
	nodes := []contracts.WorkNode{
		{ID: "a", Type: "agent", AgentID: "agent-a"},
		{ID: "b", Type: "agent", AgentID: "agent-b"},
	}
	cyclic := orderNodesByEdges(nodes, []contracts.WorkEdge{{From: "a", To: "b"}, {From: "b", To: "a"}})
	if cyclic[0].ID != "a" || cyclic[1].ID != "b" {
		t.Fatalf("cycle should preserve input order: %+v", cyclic)
	}
	withUnknown := orderNodesByEdges(nodes, []contracts.WorkEdge{{From: "missing", To: "b"}, {From: "a", To: "b"}})
	if withUnknown[0].ID != "a" || withUnknown[1].ID != "b" {
		t.Fatalf("unknown edge should be ignored: %+v", withUnknown)
	}
	sortable := []string{"b", "a"}
	sortNodeIDs(sortable, map[string]int{"a": 0, "b": 1})
	if sortable[0] != "a" || sortable[1] != "b" {
		t.Fatalf("sortNodeIDs = %+v", sortable)
	}
}
