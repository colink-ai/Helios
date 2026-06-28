package acp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/colink-ai/helios/contracts"
)

// ParseSessionUpdate converts ACP session/update payloads into Helios chunks.
// It accepts both a full notification params envelope and a raw update object.
func ParseSessionUpdate(params json.RawMessage) ([]contracts.Chunk, error) {
	rawUpdate, err := unwrapSessionUpdate(params)
	if err != nil {
		return nil, err
	}
	if len(rawUpdate) == 0 {
		return nil, nil
	}

	update, err := object(rawUpdate)
	if err != nil {
		return nil, err
	}

	switch updateType(update) {
	case "agent_message_chunk":
		if text := contentText(update); text != "" {
			return []contracts.Chunk{{Type: contracts.ChunkText, Content: text, Raw: rawUpdate}}, nil
		}
	case "agent_thought_chunk":
		if text := contentText(update); text != "" {
			return []contracts.Chunk{{Type: contracts.ChunkThinking, Content: text, Raw: rawUpdate}}, nil
		}
	case "tool_call":
		return parseToolCall(update, rawUpdate)
	case "tool_call_update":
		return parseToolCallUpdate(update, rawUpdate)
	case "usage_update":
		return parseUsage(update, rawUpdate), nil
	case "plan":
		return parsePlan(update, rawUpdate), nil
	}

	return parseLooseUpdate(update, rawUpdate), nil
}

func unwrapSessionUpdate(params json.RawMessage) (json.RawMessage, error) {
	root, err := object(params)
	if err != nil {
		return nil, err
	}
	if update, ok := rawValue(root, "update"); ok {
		return update, nil
	}
	if paramsRaw, ok := rawValue(root, "params"); ok {
		nested, err := object(paramsRaw)
		if err != nil {
			return nil, err
		}
		if update, ok := rawValue(nested, "update"); ok {
			return update, nil
		}
	}
	return params, nil
}

func parseToolCall(update map[string]any, raw json.RawMessage) ([]contracts.Chunk, error) {
	toolInput := mapValue(update, "rawInput", "raw_input", "input")
	if isQuestionTool(update, toolInput) {
		questions := parseQuestions(toolInput)
		if len(questions) > 0 {
			return []contracts.Chunk{{
				Type:      contracts.ChunkQuestion,
				ToolName:  "AskUserQuestion",
				ToolID:    stringValue(update, "toolCallId", "tool_call_id", "id"),
				ToolInput: toolInput,
				Questions: questions,
				Raw:       raw,
			}}, nil
		}
	}
	return []contracts.Chunk{{
		Type:      contracts.ChunkToolUse,
		ToolName:  toolName(update),
		ToolID:    stringValue(update, "toolCallId", "tool_call_id", "id"),
		ToolInput: toolInput,
		Raw:       raw,
		Metadata:  metadata(update, "status", "kind"),
	}}, nil
}

func parseToolCallUpdate(update map[string]any, raw json.RawMessage) ([]contracts.Chunk, error) {
	status := strings.ToLower(stringValue(update, "status"))
	toolInput := mapValue(update, "rawInput", "raw_input", "input")
	if status == "" || status == "pending" || status == "in_progress" {
		if isQuestionTool(update, toolInput) {
			questions := parseQuestions(toolInput)
			if len(questions) > 0 {
				return []contracts.Chunk{{
					Type:      contracts.ChunkQuestion,
					ToolName:  "AskUserQuestion",
					ToolID:    stringValue(update, "toolCallId", "tool_call_id", "id"),
					ToolInput: toolInput,
					Questions: questions,
					Raw:       raw,
					Metadata:  metadata(update, "status", "kind"),
				}}, nil
			}
		}
		return []contracts.Chunk{{
			Type:      contracts.ChunkToolUse,
			ToolName:  toolName(update),
			ToolID:    stringValue(update, "toolCallId", "tool_call_id", "id"),
			ToolInput: toolInput,
			Raw:       raw,
			Metadata:  metadata(update, "status", "kind"),
		}}, nil
	}

	blocks := extractContentBlocks(update["content"])
	content := joinTextBlocks(blocks)
	return []contracts.Chunk{{
		Type:             contracts.ChunkToolResult,
		Content:          content,
		ToolID:           stringValue(update, "toolCallId", "tool_call_id", "id"),
		IsError:          status == "failed" || status == "error",
		ToolResultBlocks: blocks,
		Raw:              raw,
		Metadata:         metadata(update, "status", "kind"),
	}}, nil
}

func parseUsage(update map[string]any, raw json.RawMessage) []contracts.Chunk {
	usage := &contracts.TokenUsage{
		ContextUsed:         int64Value(update, "used", "contextUsed", "context_used"),
		ContextSize:         int64Value(update, "size", "contextSize", "context_size"),
		InputTokens:         int64Value(update, "inputTokens", "input_tokens"),
		OutputTokens:        int64Value(update, "outputTokens", "output_tokens"),
		CacheReadTokens:     int64Value(update, "cacheReadTokens", "cache_read_tokens", "cache_read_input_tokens"),
		CacheCreationTokens: int64Value(update, "cacheCreationTokens", "cache_creation_tokens", "cache_creation_input_tokens"),
		CostUSD:             floatValue(update, "costUsd", "cost_usd"),
		DurationMillis:      int64Value(update, "durationMillis", "duration_ms"),
	}
	return []contracts.Chunk{{Type: contracts.ChunkUsage, Usage: usage, Raw: raw}}
}

func parsePlan(update map[string]any, raw json.RawMessage) []contracts.Chunk {
	entries := []contracts.PlanEntry{}
	if values, ok := update["entries"].([]any); ok {
		for _, value := range values {
			if item, ok := value.(map[string]any); ok {
				entries = append(entries, contracts.PlanEntry{
					ID:       stringValue(item, "id"),
					Content:  stringValue(item, "content", "text", "message"),
					Status:   stringValue(item, "status"),
					Priority: int(int64Value(item, "priority")),
					Metadata: withoutKeys(item, "id", "content", "text", "message", "status", "priority"),
				})
			}
		}
	}
	return []contracts.Chunk{{
		Type:     contracts.ChunkStatus,
		Content:  planText(entries),
		Plan:     entries,
		Raw:      raw,
		Metadata: map[string]any{"sessionUpdate": updateType(update)},
	}}
}

func parseLooseUpdate(update map[string]any, raw json.RawMessage) []contracts.Chunk {
	if text := stringValue(update, "text", "content", "delta"); text != "" {
		return []contracts.Chunk{{Type: contracts.ChunkText, Content: text, Raw: raw}}
	}
	if msg := stringValue(update, "message", "status"); msg != "" {
		return []contracts.Chunk{{Type: contracts.ChunkStatus, Content: msg, Raw: raw}}
	}
	if name := stringValue(update, "toolName", "name", "title"); name != "" {
		return []contracts.Chunk{{
			Type:      contracts.ChunkToolUse,
			ToolName:  name,
			ToolID:    stringValue(update, "toolCallId", "toolId", "id"),
			ToolInput: mapValue(update, "input", "rawInput"),
			Raw:       raw,
		}}
	}
	return nil
}

func updateType(update map[string]any) string {
	return stringValue(update, "sessionUpdate", "session_update", "type")
}

func contentText(update map[string]any) string {
	if content := mapValue(update, "content"); content != nil {
		if text := stringValue(content, "text", "content"); text != "" {
			return text
		}
	}
	return stringValue(update, "text", "content", "message")
}

func toolName(update map[string]any) string {
	if meta := mapValue(update, "_meta"); meta != nil {
		if claude := mapValue(meta, "claudeCode"); claude != nil {
			if name := stringValue(claude, "toolName"); name != "" {
				return name
			}
		}
	}
	return stringValue(update, "title", "name", "toolName")
}

func isQuestionTool(update map[string]any, input map[string]any) bool {
	name := strings.ToLower(toolName(update))
	kind := strings.ToLower(stringValue(update, "kind"))
	if strings.Contains(name, "askuserquestion") ||
		strings.Contains(name, "ask user") ||
		strings.Contains(name, "user question") ||
		name == "question" ||
		kind == "ask_user" ||
		kind == "user_input" ||
		kind == "question" {
		return true
	}
	if input != nil {
		_, ok := input["questions"]
		return ok
	}
	return false
}

func parseQuestions(input map[string]any) []contracts.QuestionItem {
	if input == nil {
		return nil
	}
	values, ok := input["questions"].([]any)
	if !ok {
		return nil
	}
	out := make([]contracts.QuestionItem, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		q := contracts.QuestionItem{
			ID:          stringValue(item, "id"),
			Header:      stringValue(item, "header"),
			Question:    stringValue(item, "question", "text", "message"),
			MultiSelect: boolValue(item, "multiSelect", "multiple"),
		}
		if options, ok := item["options"].([]any); ok {
			for _, option := range options {
				opt, ok := option.(map[string]any)
				if !ok {
					continue
				}
				q.Options = append(q.Options, contracts.QuestionOption{
					Label:       stringValue(opt, "label"),
					Description: stringValue(opt, "description"),
					Preview:     stringValue(opt, "preview"),
				})
			}
		}
		out = append(out, q)
	}
	return out
}

func extractContentBlocks(value any) []contracts.ContentBlock {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]contracts.ContentBlock, 0, len(values))
	for _, value := range values {
		block, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if nested := mapValue(block, "content"); nested != nil {
			if text := stringValue(nested, "text", "content"); text != "" {
				out = append(out, contracts.ContentBlock{Type: stringValue(nested, "type"), Text: text, Raw: rawJSON(value)})
				continue
			}
		}
		out = append(out, contracts.ContentBlock{
			Type: stringValue(block, "type"),
			Text: stringValue(block, "text", "content"),
			Raw:  rawJSON(value),
		})
	}
	return out
}

func joinTextBlocks(blocks []contracts.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func planText(entries []contracts.PlanEntry) string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		prefix := ""
		if entry.Priority != 0 || entry.Status != "" {
			prefix = "[" + strconv.Itoa(entry.Priority) + "] [" + entry.Status + "] "
		}
		lines = append(lines, prefix+entry.Content)
	}
	return strings.Join(lines, "\n")
}

func object(raw json.RawMessage) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse acp object: %w", err)
	}
	return out, nil
}

func rawValue(values map[string]any, key string) (json.RawMessage, bool) {
	value, ok := values[key]
	if !ok {
		return nil, false
	}
	return rawJSON(value), true
}

func rawJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func mapValue(values map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value, ok := values[key].(map[string]any); ok {
			return value
		}
	}
	return nil
}

func stringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			return value
		case fmt.Stringer:
			return value.String()
		}
	}
	return ""
}

func boolValue(values map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := values[key].(bool); ok {
			return value
		}
	}
	return false
}

func int64Value(values map[string]any, keys ...string) int64 {
	for _, key := range keys {
		switch value := values[key].(type) {
		case float64:
			return int64(value)
		case int64:
			return value
		case int:
			return int64(value)
		case json.Number:
			n, _ := value.Int64()
			return n
		}
	}
	return 0
}

func floatValue(values map[string]any, keys ...string) float64 {
	for _, key := range keys {
		switch value := values[key].(type) {
		case float64:
			return value
		case int:
			return float64(value)
		case int64:
			return float64(value)
		case json.Number:
			n, _ := value.Float64()
			return n
		}
	}
	return 0
}

func metadata(values map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	if typ := updateType(values); typ != "" {
		out["sessionUpdate"] = typ
	}
	for _, key := range keys {
		if value, ok := values[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func withoutKeys(values map[string]any, keys ...string) map[string]any {
	skip := map[string]bool{}
	for _, key := range keys {
		skip[key] = true
	}
	out := map[string]any{}
	for key, value := range values {
		if !skip[key] {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	ordered := make(map[string]any, len(out))
	keysOut := make([]string, 0, len(out))
	for key := range out {
		keysOut = append(keysOut, key)
	}
	sort.Strings(keysOut)
	for _, key := range keysOut {
		ordered[key] = out[key]
	}
	return ordered
}
