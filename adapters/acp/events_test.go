package acp

import (
	"encoding/json"
	"fmt"
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

func TestParseACPQuestionToolInitialCallAndMetaName(t *testing.T) {
	params := json.RawMessage(`{
		"sessionId":"s1",
		"update":{
			"sessionUpdate":"tool_call",
			"toolCallId":"question-2",
			"_meta":{"claudeCode":{"toolName":"AskUserQuestion"}},
			"rawInput":{
				"questions":[{
					"id":"q1",
					"header":"Choice",
					"text":"Pick one",
					"multiSelect":true,
					"options":[{"label":"A","preview":"alpha"}]
				}]
			}
		}
	}`)
	chunks, err := ParseSessionUpdate(params)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkQuestion || chunks[0].ToolID != "question-2" {
		t.Fatalf("unexpected chunks: %+v", chunks)
	}
	if len(chunks[0].Questions) != 1 || chunks[0].Questions[0].Question != "Pick one" || chunks[0].Questions[0].Options[0].Preview != "alpha" {
		t.Fatalf("unexpected questions: %+v", chunks[0].Questions)
	}

	toolChunks, err := ParseSessionUpdate(json.RawMessage(`{
		"sessionId":"s1",
		"update":{
			"sessionUpdate":"tool_input_delta",
			"toolCallId":"tool-1",
			"_meta":{"claudeCode":{"toolName":"Edit"}},
			"delta":"{\"path\":\"a\"}"
		}
	}`))
	if err != nil {
		t.Fatalf("parse meta tool: %v", err)
	}
	if len(toolChunks) != 1 || toolChunks[0].ToolName != "Edit" {
		t.Fatalf("unexpected meta tool chunks: %+v", toolChunks)
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

func TestParseACPFailedToolResultAndFallbacks(t *testing.T) {
	chunks, err := ParseSessionUpdate(json.RawMessage(`{
		"sessionId":"s1",
		"update":{
			"sessionUpdate":"tool_call_update",
			"toolCallId":"tool-1",
			"status":"failed",
			"content":[{"type":"text","text":"bad input"}]
		}
	}`))
	if err != nil {
		t.Fatalf("parse failed tool result: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkToolResult || !chunks[0].IsError || chunks[0].Content != "bad input" {
		t.Fatalf("unexpected failed tool chunks: %+v", chunks)
	}

	message, err := ParseSessionUpdate(json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"agent_message_chunk","message":"fallback text"}}`))
	if err != nil {
		t.Fatalf("parse content fallback: %v", err)
	}
	if len(message) != 1 || message[0].Content != "fallback text" {
		t.Fatalf("unexpected message fallback: %+v", message)
	}

	handoff := parseHandoff(map[string]any{
		"sessionUpdate": "handoff",
		"targetType":    "agent",
		"message":       "delegate",
		"input":         map[string]any{"task": "review"},
	}, json.RawMessage(`{}`))
	if handoff[0].Handoff.Target.Type != "agent" || handoff[0].Handoff.Payload["task"] != "review" {
		t.Fatalf("unexpected handoff fallback: %+v", handoff)
	}
}

func TestParseACPArtifactHandoffPermissionAndError(t *testing.T) {
	artifact := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"artifact_created","artifactId":"a1","artifactType":"code","name":"patch.diff","path":"/tmp/patch.diff"}}`)
	chunks, err := ParseSessionUpdate(artifact)
	if err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkArtifact || chunks[0].Artifact.Name != "patch.diff" || chunks[0].Artifact.Type != contracts.ArtifactCode {
		t.Fatalf("unexpected artifact chunks: %+v", chunks)
	}

	handoff := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"handoff_requested","handoffId":"h1","reason":"needs review","target":{"type":"human","id":"u1","name":"Reviewer"},"payload":{"risk":"high"}}}`)
	chunks, err = ParseSessionUpdate(handoff)
	if err != nil {
		t.Fatalf("parse handoff: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkHandoff || chunks[0].Handoff.Target.Type != "human" || chunks[0].Handoff.Payload["risk"] != "high" {
		t.Fatalf("unexpected handoff chunks: %+v", chunks)
	}

	permission := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"permission_request","permissionId":"p1","action":"shell","command":"go test","reason":"run tests","options":[{"label":"Allow"}]}}`)
	chunks, err = ParseSessionUpdate(permission)
	if err != nil {
		t.Fatalf("parse permission: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkPermission || chunks[0].Permission.Action != "shell" || chunks[0].Permission.Options[0].Label != "Allow" {
		t.Fatalf("unexpected permission chunks: %+v", chunks)
	}

	errUpdate := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"error","message":"boom","code":"E_RUNTIME"}}`)
	chunks, err = ParseSessionUpdate(errUpdate)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkError || chunks[0].Content != "boom" || !chunks[0].IsError {
		t.Fatalf("unexpected error chunks: %+v", chunks)
	}
}

func TestParseACPToolInputDelta(t *testing.T) {
	params := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"tool_input_delta","toolCallId":"t1","title":"edit","partialJson":"{\"path\""}}`)
	chunks, err := ParseSessionUpdate(params)
	if err != nil {
		t.Fatalf("parse delta: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkInputJSONDelta || chunks[0].ToolID != "t1" || chunks[0].PartialJSON != "{\"path\"" {
		t.Fatalf("unexpected delta chunks: %+v", chunks)
	}
}

func TestParseNestedAndLooseUpdates(t *testing.T) {
	nested := json.RawMessage(`{"params":{"update":{"sessionUpdate":"agent_message_chunk","content":{"text":"nested"}}}}`)
	chunks, err := ParseSessionUpdate(nested)
	if err != nil {
		t.Fatalf("parse nested: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Content != "nested" {
		t.Fatalf("unexpected nested chunks: %+v", chunks)
	}
	status := json.RawMessage(`{"sessionId":"s1","update":{"message":"working","status":"running"}}`)
	chunks, err = ParseSessionUpdate(status)
	if err != nil {
		t.Fatalf("parse status: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkStatus {
		t.Fatalf("unexpected status chunks: %+v", chunks)
	}
	tool := json.RawMessage(`{"sessionId":"s1","update":{"title":"edit","toolId":"t1","input":{"path":"x"}}}`)
	chunks, err = ParseSessionUpdate(tool)
	if err != nil {
		t.Fatalf("parse loose tool: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Type != contracts.ChunkToolUse || chunks[0].ToolName != "edit" {
		t.Fatalf("unexpected loose tool chunks: %+v", chunks)
	}
}

func TestParseArtifactTypesAndFallbacks(t *testing.T) {
	for typ, want := range map[string]contracts.ArtifactType{
		"document": contracts.ArtifactDocument,
		"review":   contracts.ArtifactReview,
		"test":     contracts.ArtifactTest,
		"config":   contracts.ArtifactConfig,
		"data":     contracts.ArtifactData,
		"weird":    contracts.ArtifactOther,
	} {
		if got := artifactType(typ); got != want {
			t.Fatalf("artifactType(%q)=%q want %q", typ, got, want)
		}
	}
	chunks := parseArtifact(map[string]any{"sessionUpdate": "artifact", "artifactType": "weird", "id": "a1"}, json.RawMessage(`{}`))
	if chunks[0].Artifact.Name != "a1" || chunks[0].Artifact.Type != contracts.ArtifactOther {
		t.Fatalf("unexpected artifact fallback: %+v", chunks[0].Artifact)
	}
	errChunks := parseError(map[string]any{"sessionUpdate": "error"}, json.RawMessage(`{}`))
	if errChunks[0].Content == "" || !errChunks[0].IsError {
		t.Fatalf("unexpected error fallback: %+v", errChunks)
	}
}

func TestNumericAndMetadataHelpers(t *testing.T) {
	values := map[string]any{
		"i":        json.Number("42"),
		"f":        json.Number("1.5"),
		"b":        true,
		"plainInt": 7,
		"int64":    int64(9),
		"float":    2.5,
		"stringer": stringerValue("from-stringer"),
	}
	if int64Value(values, "i") != 42 || floatValue(values, "f") != 1.5 || !boolValue(values, "b") {
		t.Fatalf("unexpected numeric values")
	}
	if int64Value(values, "plainInt") != 7 || int64Value(values, "int64") != 9 {
		t.Fatalf("unexpected int values")
	}
	if floatValue(values, "plainInt") != 7 || floatValue(values, "int64") != 9 || floatValue(values, "float") != 2.5 {
		t.Fatalf("unexpected float values")
	}
	if stringValue(values, "stringer") != "from-stringer" {
		t.Fatalf("unexpected stringer value")
	}
	if len(metadata(map[string]any{}, "missing")) != 0 {
		t.Fatalf("empty metadata should be nil")
	}
	if _, err := unwrapSessionUpdate(json.RawMessage(`[]`)); err == nil {
		t.Fatalf("non-object unwrap should fail")
	}
	if _, err := object(json.RawMessage(`[]`)); err == nil {
		t.Fatalf("non-object should fail")
	}
}

type stringerValue string

func (s stringerValue) String() string {
	return fmt.Sprintf("%s", string(s))
}
