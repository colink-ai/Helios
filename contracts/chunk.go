package contracts

// ChunkType describes a normalized streaming output chunk.
type ChunkType string

const (
	ChunkText           ChunkType = "text"
	ChunkError          ChunkType = "error"
	ChunkStatus         ChunkType = "status"
	ChunkThinking       ChunkType = "thinking"
	ChunkToolUse        ChunkType = "tool_use"
	ChunkToolResult     ChunkType = "tool_result"
	ChunkInputJSONDelta ChunkType = "input_json_delta"
	ChunkUsage          ChunkType = "usage"
	ChunkQuestion       ChunkType = "question"
	ChunkDone           ChunkType = "done"
)

// ImageContent carries image input for multimodal agents.
type ImageContent struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
	URL      string `json:"url,omitempty"`
}

// TokenUsage reports model or adapter token usage.
type TokenUsage struct {
	InputTokens         int64   `json:"inputTokens,omitempty"`
	OutputTokens        int64   `json:"outputTokens,omitempty"`
	CacheReadTokens     int64   `json:"cacheReadTokens,omitempty"`
	CacheCreationTokens int64   `json:"cacheCreationTokens,omitempty"`
	ContextUsed         int64   `json:"contextUsed,omitempty"`
	ContextSize         int64   `json:"contextSize,omitempty"`
	CostUSD             float64 `json:"costUsd,omitempty"`
	DurationMillis      int64   `json:"durationMillis,omitempty"`
}

// QuestionOption is one selectable answer for an agent question.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Preview     string `json:"preview,omitempty"`
}

// QuestionItem represents a structured question emitted by an agent.
type QuestionItem struct {
	ID          string           `json:"id,omitempty"`
	Header      string           `json:"header,omitempty"`
	Question    string           `json:"question"`
	MultiSelect bool             `json:"multiSelect,omitempty"`
	Options     []QuestionOption `json:"options,omitempty"`
}

// Chunk is the normalized streaming unit emitted by adapters.
type Chunk struct {
	Type        ChunkType      `json:"type"`
	Content     string         `json:"content,omitempty"`
	ToolName    string         `json:"toolName,omitempty"`
	ToolID      string         `json:"toolId,omitempty"`
	ToolInput   map[string]any `json:"toolInput,omitempty"`
	ToolIndex   int            `json:"toolIndex,omitempty"`
	PartialJSON string         `json:"partialJson,omitempty"`
	IsError     bool           `json:"isError,omitempty"`
	Usage       *TokenUsage    `json:"usage,omitempty"`
	Done        bool           `json:"done,omitempty"`
	Questions   []QuestionItem `json:"questions,omitempty"`
}
