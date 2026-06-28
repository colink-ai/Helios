package runtime

import (
	"context"
	"fmt"
)

// SessionRequestFromSnapshot builds a resume request from application-owned
// session metadata.
func SessionRequestFromSnapshot(snapshot SessionSnapshot, agent AgentSpec) (SessionRequest, error) {
	if snapshot.SessionID == "" {
		return SessionRequest{}, fmt.Errorf("snapshot session id is required")
	}
	if snapshot.AgentSessionID == "" {
		return SessionRequest{}, fmt.Errorf("snapshot agent session id is required")
	}
	if agent.Type == "" {
		agent.Type = snapshot.AgentType
	}
	if agent.ID == "" {
		agent.ID = snapshot.AgentID
	}
	return SessionRequest{
		RunID:           snapshot.RunID,
		SessionID:       snapshot.SessionID,
		Agent:           agent,
		ResumeSessionID: snapshot.AgentSessionID,
		Metadata:        snapshot.Metadata,
	}, nil
}

// ResumeSessionFromSnapshot starts a runtime session using resume metadata
// loaded by the host application.
func (e *Engine) ResumeSessionFromSnapshot(ctx context.Context, snapshot SessionSnapshot, agent AgentSpec) (*SessionHandle, error) {
	req, err := SessionRequestFromSnapshot(snapshot, agent)
	if err != nil {
		return nil, err
	}
	return e.StartSession(ctx, req)
}
