package acp

import (
	"encoding/json"
	"testing"

	"github.com/colink-ai/helios/contracts"
)

func TestParseElicitationQuestions(t *testing.T) {
	var req elicitationRequest
	if err := json.Unmarshal([]byte(`{
		"mode":"form",
		"message":"Fallback question",
		"requestedSchema":{
			"properties":{
				"question_1":{
					"type":"array",
					"title":"Second",
					"description":"Pick many",
					"items":{"anyOf":[{"const":"B","title":"B — Beta"}]}
				},
				"question_0":{
					"type":"string",
					"title":"First",
					"oneOf":[{"const":"A","title":"A — Alpha","_meta":{"_claude/askUserQuestionOption":{"description":"Alpha desc","preview":"A preview"}}}]
				},
				"question_0_custom":{"type":"string"}
			}
		}
	}`), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	questions := parseElicitationQuestions(req.RequestedSchema.Properties, req.Message)
	if len(questions) != 2 {
		t.Fatalf("questions len = %d: %+v", len(questions), questions)
	}
	if questions[0].ID != "question_0" || questions[0].Question != "Fallback question" || questions[0].Options[0].Description != "Alpha desc" {
		t.Fatalf("unexpected first question: %+v", questions[0])
	}
	if questions[1].ID != "question_1" || !questions[1].MultiSelect || questions[1].Options[0].Description != "Beta" {
		t.Fatalf("unexpected second question: %+v", questions[1])
	}
}

func TestBuildElicitationContent(t *testing.T) {
	questions := parseElicitationQuestions(map[string]json.RawMessage{
		"question_0": json.RawMessage(`{"type":"string","title":"First"}`),
	}, "Fallback")
	content := buildElicitationContent("A", questions)
	if content["question_0"] != "A" {
		t.Fatalf("content = %+v", content)
	}
	content = buildElicitationContent(`{"question_0":"B","question_1":["C"]}`, questions)
	if content["question_0"] != "B" {
		t.Fatalf("json content = %+v", content)
	}
}

func TestParseElicitationInvalidAndEmpty(t *testing.T) {
	if _, err := parseElicitation(json.RawMessage(`{`)); err == nil {
		t.Fatalf("invalid elicitation should fail")
	}
	if questions := parseElicitationQuestions(nil, "fallback"); questions != nil {
		t.Fatalf("nil props should not produce questions: %+v", questions)
	}
	questions := parseElicitationQuestions(map[string]json.RawMessage{
		"other":      json.RawMessage(`{"type":"string"}`),
		"question_0": json.RawMessage(`{`),
	}, "fallback")
	if len(questions) != 0 {
		t.Fatalf("invalid question fields should be skipped: %+v", questions)
	}
}

func TestBuildElicitationContentFallbackKey(t *testing.T) {
	content := buildElicitationContent("plain", nil)
	if content["question_0"] != "plain" {
		t.Fatalf("fallback content = %+v", content)
	}
	content = buildElicitationContent(`{"broken"`, []contracts.QuestionItem{{ID: "question_custom"}})
	if content["question_custom"] != `{"broken"` {
		t.Fatalf("invalid json should be treated as answer: %+v", content)
	}
}
