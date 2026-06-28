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
