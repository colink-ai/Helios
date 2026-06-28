package acp

import (
	"encoding/json"
	"testing"
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
