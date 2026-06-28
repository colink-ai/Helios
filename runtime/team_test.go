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
