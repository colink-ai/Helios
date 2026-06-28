package acp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/colink-ai/helios/contracts"
)

type elicitationRequest struct {
	Mode            string `json:"mode"`
	SessionID       string `json:"sessionId"`
	ToolCallID      string `json:"toolCallId"`
	Message         string `json:"message"`
	RequestedSchema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	} `json:"requestedSchema"`
}

func parseElicitation(params json.RawMessage) (*elicitationRequest, error) {
	var req elicitationRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func parseElicitationQuestions(props map[string]json.RawMessage, fallbackMessage string) []contracts.QuestionItem {
	if len(props) == 0 {
		return nil
	}
	indices := make([]int, 0, len(props))
	for key := range props {
		var idx int
		if _, err := fmt.Sscanf(key, "question_%d", &idx); err != nil {
			continue
		}
		if key == fmt.Sprintf("question_%d", idx) {
			indices = append(indices, idx)
		}
	}
	sort.Ints(indices)

	out := make([]contracts.QuestionItem, 0, len(indices))
	for _, idx := range indices {
		var field struct {
			Type        string `json:"type"`
			Title       string `json:"title"`
			Description string `json:"description"`
			OneOf       []struct {
				Const string         `json:"const"`
				Title string         `json:"title"`
				Meta  map[string]any `json:"_meta"`
			} `json:"oneOf"`
			Items *struct {
				AnyOf []struct {
					Const string         `json:"const"`
					Title string         `json:"title"`
					Meta  map[string]any `json:"_meta"`
				} `json:"anyOf"`
			} `json:"items"`
		}
		if err := json.Unmarshal(props[fmt.Sprintf("question_%d", idx)], &field); err != nil {
			continue
		}

		q := contracts.QuestionItem{
			ID:          fmt.Sprintf("question_%d", idx),
			Header:      field.Title,
			Question:    field.Description,
			MultiSelect: field.Type == "array",
		}
		if q.Question == "" {
			q.Question = fallbackMessage
		}
		if field.Type == "array" && field.Items != nil {
			for _, option := range field.Items.AnyOf {
				q.Options = append(q.Options, elicitationOption(option.Const, option.Title, option.Meta))
			}
		} else {
			for _, option := range field.OneOf {
				q.Options = append(q.Options, elicitationOption(option.Const, option.Title, option.Meta))
			}
		}
		out = append(out, q)
	}
	return out
}

func elicitationOption(label, title string, meta map[string]any) contracts.QuestionOption {
	opt := contracts.QuestionOption{Label: label}
	if detail, ok := meta["_claude/askUserQuestionOption"].(map[string]any); ok {
		opt.Description = stringFromAny(detail["description"])
		opt.Preview = stringFromAny(detail["preview"])
	}
	if opt.Description == "" && title != "" && title != label {
		opt.Description = strings.TrimSpace(strings.TrimPrefix(title, label))
		opt.Description = strings.TrimPrefix(opt.Description, "—")
		opt.Description = strings.TrimSpace(strings.TrimPrefix(opt.Description, "-"))
	}
	return opt
}

func buildElicitationContent(answer string, questions []contracts.QuestionItem) map[string]any {
	trimmed := strings.TrimSpace(answer)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil && parsed != nil {
			return parsed
		}
	}
	if len(questions) == 1 && questions[0].ID != "" {
		return map[string]any{questions[0].ID: answer}
	}
	return map[string]any{"question_0": answer}
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
