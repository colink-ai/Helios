package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/colink-ai/helios/contracts"
)

// TeamRunRequest runs a lightweight multi-agent work graph.
type TeamRunRequest struct {
	RunID    string               `json:"runId,omitempty"`
	Team     contracts.AgentTeam  `json:"team"`
	Agents   map[string]AgentSpec `json:"agents"`
	Input    string               `json:"input"`
	WorkDir  string               `json:"workDir,omitempty"`
	Metadata map[string]any       `json:"metadata,omitempty"`
}

// TeamRunResult captures the ordered outputs and A2A messages from a team run.
type TeamRunResult struct {
	RunID    string                 `json:"runId,omitempty"`
	Output   string                 `json:"output,omitempty"`
	Messages []contracts.A2AMessage `json:"messages,omitempty"`
	Results  map[string]*RunResult  `json:"results,omitempty"`
}

// TeamRunner executes simple WorkGraph-based agent teams through an Engine.
type TeamRunner struct {
	engine *Engine
}

func NewTeamRunner(engine *Engine) *TeamRunner {
	return &TeamRunner{engine: engine}
}

func (r *TeamRunner) Run(ctx context.Context, req TeamRunRequest) (*TeamRunResult, error) {
	if r.engine == nil {
		return nil, fmt.Errorf("engine is required")
	}
	runID := req.RunID
	if runID == "" {
		runID = NewID("teamrun")
	}
	nodes := orderedAgentNodes(req.Team)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("team %s has no agent nodes", req.Team.ID)
	}
	result := &TeamRunResult{RunID: runID, Results: map[string]*RunResult{}}
	input := req.Input
	var fromAgent string
	for _, node := range nodes {
		spec, ok := req.Agents[node.AgentID]
		if !ok {
			return nil, fmt.Errorf("agent spec not found for node %s agent %s", node.ID, node.AgentID)
		}
		spec.ID = node.AgentID
		run, err := r.engine.Run(ctx, RunRequest{
			RunID:    runID + ":" + node.ID,
			Agent:    spec,
			Input:    input,
			WorkDir:  req.WorkDir,
			Metadata: req.Metadata,
		})
		if err != nil {
			return nil, err
		}
		result.Results[node.ID] = run
		output := ""
		if run != nil {
			output = run.Output
		}
		result.Messages = append(result.Messages, contracts.A2AMessage{
			ID:        NewID("a2a"),
			RunID:     runID,
			FromAgent: fromAgent,
			ToAgent:   node.AgentID,
			Content:   input,
			CreatedAt: time.Now().UTC(),
		})
		fromAgent = node.AgentID
		input = output
		result.Output = output
	}
	return result, nil
}

func orderedAgentNodes(team contracts.AgentTeam) []contracts.WorkNode {
	if team.Graph == nil {
		out := make([]contracts.WorkNode, 0, len(team.Agents))
		for _, agent := range team.Agents {
			out = append(out, contracts.WorkNode{ID: agent.ID, Type: "agent", AgentID: agent.ID, Name: agent.Name})
		}
		return out
	}
	out := []contracts.WorkNode{}
	for _, node := range team.Graph.Nodes {
		if node.Type == "agent" && node.AgentID != "" {
			out = append(out, node)
		}
	}
	return out
}
