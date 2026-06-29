package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/colink-ai/helios/contracts"
)

// TeamRunRequest runs a lightweight multi-agent work graph.
type TeamRunRequest struct {
	RunID           string               `json:"runId,omitempty"`
	Team            contracts.AgentTeam  `json:"team"`
	Agents          map[string]AgentSpec `json:"agents"`
	Input           string               `json:"input"`
	WorkDir         string               `json:"workDir,omitempty"`
	ContinueOnError bool                 `json:"continueOnError,omitempty"`
	Metadata        map[string]any       `json:"metadata,omitempty"`
}

// TeamRunResult captures the ordered outputs and A2A messages from a team run.
type TeamRunResult struct {
	RunID      string                 `json:"runId,omitempty"`
	Output     string                 `json:"output,omitempty"`
	Messages   []contracts.A2AMessage `json:"messages,omitempty"`
	Results    map[string]*RunResult  `json:"results,omitempty"`
	NodeErrors map[string]string      `json:"nodeErrors,omitempty"`
	Skipped    []string               `json:"skipped,omitempty"`
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
	result := &TeamRunResult{RunID: runID, Results: map[string]*RunResult{}, NodeErrors: map[string]string{}}
	input := req.Input
	var fromAgent string
	for _, node := range nodes {
		if shouldSkipNode(node) {
			result.Skipped = append(result.Skipped, node.ID)
			continue
		}
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
			if !req.ContinueOnError {
				return nil, err
			}
			result.NodeErrors[node.ID] = err.Error()
			continue
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
			// Content is the input handed to this node; the node output is stored
			// in result.Results and becomes the next node's input.
			Content:   input,
			CreatedAt: time.Now().UTC(),
		})
		fromAgent = node.AgentID
		input = output
		result.Output = output
	}
	if len(result.NodeErrors) == 0 {
		result.NodeErrors = nil
	}
	return result, nil
}

func shouldSkipNode(node contracts.WorkNode) bool {
	if node.Metadata == nil {
		return false
	}
	switch value := node.Metadata["condition"].(type) {
	case string:
		return value == "never" || value == "skip" || value == "disabled"
	case bool:
		return !value
	default:
		return false
	}
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
	return orderNodesByEdges(out, team.Graph.Edges)
}

func orderNodesByEdges(nodes []contracts.WorkNode, edges []contracts.WorkEdge) []contracts.WorkNode {
	if len(nodes) < 2 || len(edges) == 0 {
		return nodes
	}
	byID := map[string]contracts.WorkNode{}
	index := map[string]int{}
	for i, node := range nodes {
		byID[node.ID] = node
		index[node.ID] = i
	}
	indegree := map[string]int{}
	next := map[string][]string{}
	for _, node := range nodes {
		indegree[node.ID] = 0
	}
	for _, edge := range edges {
		if _, ok := byID[edge.From]; !ok {
			continue
		}
		if _, ok := byID[edge.To]; !ok {
			continue
		}
		next[edge.From] = append(next[edge.From], edge.To)
		indegree[edge.To]++
	}
	ready := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if indegree[node.ID] == 0 {
			ready = append(ready, node.ID)
		}
	}
	out := make([]contracts.WorkNode, 0, len(nodes))
	for len(ready) > 0 {
		sortNodeIDs(ready, index)
		id := ready[0]
		ready = ready[1:]
		out = append(out, byID[id])
		for _, to := range next[id] {
			indegree[to]--
			if indegree[to] == 0 {
				ready = append(ready, to)
			}
		}
	}
	if len(out) != len(nodes) {
		return nodes
	}
	return out
}

func sortNodeIDs(values []string, index map[string]int) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && index[values[j]] < index[values[j-1]]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
