package acp

import "encoding/json"

// Request is a JSON-RPC request used by ACP transports.
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Response is a JSON-RPC response used by ACP transports.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a JSON-RPC error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Notification is an ACP server notification.
type Notification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type InitializeParams struct {
	ProtocolVersion    int            `json:"protocolVersion"`
	ClientCapabilities map[string]any `json:"clientCapabilities,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion   int            `json:"protocolVersion,omitempty"`
	AgentCapabilities map[string]any `json:"agentCapabilities,omitempty"`
}

type SessionParams struct {
	CWD        string `json:"cwd,omitempty"`
	SessionID  string `json:"sessionId,omitempty"`
	MCPServers []any  `json:"mcpServers"`
}

type PromptParams struct {
	SessionID string         `json:"sessionId,omitempty"`
	Prompt    []ContentBlock `json:"prompt,omitempty"`
}

type EndSessionParams struct {
	SessionID string `json:"sessionId,omitempty"`
}

type ContentBlock struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	MimeType string         `json:"mimeType,omitempty"`
	Data     string         `json:"data,omitempty"`
	URL      string         `json:"url,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewRequest creates a JSON-RPC request.
func NewRequest(id any, method string, params any) Request {
	return Request{JSONRPC: "2.0", ID: id, Method: method, Params: params}
}
