package acp

import (
	"encoding/json"
	"testing"

	"github.com/colink-ai/helios/contracts"
)

func TestParseSessionUpdateText(t *testing.T) {
	params := json.RawMessage(`{"sessionId":"s1","update":{"text":"hello"}}`)
	chunks, err := ParseSessionUpdate(params)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkText || chunks[0].Content != "hello" {
		t.Fatalf("unexpected chunks: %+v", chunks)
	}
}

func TestParseSessionUpdateToolUse(t *testing.T) {
	params := json.RawMessage(`{"sessionId":"s1","update":{"name":"search","toolCallId":"t1","input":{"q":"helios"}}}`)
	chunks, err := ParseSessionUpdate(params)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkToolUse || chunks[0].ToolName != "search" {
		t.Fatalf("unexpected chunks: %+v", chunks)
	}
	if chunks[0].ToolInput["q"] != "helios" {
		t.Fatalf("unexpected tool input: %+v", chunks[0].ToolInput)
	}
}

func TestParseACPAgentMessageAndThought(t *testing.T) {
	message := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hello"}}}`)
	chunks, err := ParseSessionUpdate(message)
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkText || chunks[0].Content != "hello" {
		t.Fatalf("unexpected message chunks: %+v", chunks)
	}

	thought := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"thinking"}}}`)
	chunks, err = ParseSessionUpdate(thought)
	if err != nil {
		t.Fatalf("parse thought: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkThinking || chunks[0].Content != "thinking" {
		t.Fatalf("unexpected thought chunks: %+v", chunks)
	}
}

func TestParseACPToolCallUpdateResultNestedContent(t *testing.T) {
	params := json.RawMessage(`{
		"sessionId":"s1",
		"update":{
			"sessionUpdate":"tool_call_update",
			"toolCallId":"tool-1",
			"status":"completed",
			"content":[
				{"type":"content","content":{"type":"text","text":"nested result"}}
			]
		}
	}`)
	chunks, err := ParseSessionUpdate(params)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkToolResult || chunks[0].Content != "nested result" {
		t.Fatalf("unexpected chunks: %+v", chunks)
	}
	if len(chunks[0].ToolResultBlocks) != 1 || chunks[0].ToolResultBlocks[0].Text != "nested result" {
		t.Fatalf("unexpected blocks: %+v", chunks[0].ToolResultBlocks)
	}
}

func TestParseACPQuestionTool(t *testing.T) {
	params := json.RawMessage(`{
		"sessionId":"s1",
		"update":{
			"sessionUpdate":"tool_call_update",
			"toolCallId":"question-1",
			"title":"question",
			"kind":"question",
			"status":"in_progress",
			"rawInput":{
				"questions":[{
					"id":"q1",
					"header":"Choice",
					"question":"Pick one",
					"multiple":true,
					"options":[{"label":"A","description":"Alpha"}]
				}]
			}
		}
	}`)
	chunks, err := ParseSessionUpdate(params)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkQuestion || chunks[0].ToolName != "AskUserQuestion" {
		t.Fatalf("unexpected chunks: %+v", chunks)
	}
	if len(chunks[0].Questions) != 1 || !chunks[0].Questions[0].MultiSelect || chunks[0].Questions[0].Options[0].Label != "A" {
		t.Fatalf("unexpected questions: %+v", chunks[0].Questions)
	}
}

func TestParseACPUsageAndPlan(t *testing.T) {
	usage := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"usage_update","used":12,"size":100}}`)
	chunks, err := ParseSessionUpdate(usage)
	if err != nil {
		t.Fatalf("parse usage: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkUsage || chunks[0].Usage.ContextUsed != 12 || chunks[0].Usage.ContextSize != 100 {
		t.Fatalf("unexpected usage chunks: %+v", chunks)
	}

	plan := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"plan","entries":[{"priority":1,"status":"todo","content":"write tests"}]}}`)
	chunks, err = ParseSessionUpdate(plan)
	if err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	if len(chunks) != 1 || len(chunks[0].Plan) != 1 || chunks[0].Plan[0].Content != "write tests" {
		t.Fatalf("unexpected plan chunks: %+v", chunks)
	}
}
