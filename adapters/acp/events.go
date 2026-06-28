package acp

import (
	"encoding/json"

	"github.com/colink-ai/helios/contracts"
)

// ParseSessionUpdate converts common ACP session/update payloads into chunks.
// It is intentionally permissive because ACP-compatible CLIs differ slightly in
// their event shape.
func ParseSessionUpdate(params json.RawMessage) ([]contracts.Chunk, error) {
	var envelope struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Update) == 0 {
		return nil, nil
	}

	var update map[string]any
	if err := json.Unmarshal(envelope.Update, &update); err != nil {
		return nil, err
	}

	if text, ok := stringValue(update, "text", "content", "delta"); ok && text != "" {
		return []contracts.Chunk{{Type: contracts.ChunkText, Content: text}}, nil
	}
	if msg, ok := stringValue(update, "message", "status"); ok && msg != "" {
		return []contracts.Chunk{{Type: contracts.ChunkStatus, Content: msg}}, nil
	}
	if name, ok := stringValue(update, "toolName", "name"); ok {
		chunk := contracts.Chunk{Type: contracts.ChunkToolUse, ToolName: name}
		if id, ok := stringValue(update, "toolCallId", "toolId", "id"); ok {
			chunk.ToolID = id
		}
		if input, ok := update["input"].(map[string]any); ok {
			chunk.ToolInput = input
		}
		return []contracts.Chunk{chunk}, nil
	}
	return nil, nil
}

func stringValue(values map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := values[key].(string); ok {
			return value, true
		}
	}
	return "", false
}
