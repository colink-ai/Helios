package acp

import (
	"encoding/json"
	"strings"

	"github.com/colink-ai/helios/contracts"
)

func isPermissionRequestMethod(method string) bool {
	switch strings.ToLower(method) {
	case "permission/request", "permission/create", "approval/request", "approval/create":
		return true
	default:
		return false
	}
}

func parsePermissionRequest(params json.RawMessage) *contracts.PermissionRequest {
	values := map[string]any{}
	_ = json.Unmarshal(params, &values)
	permission := &contracts.PermissionRequest{
		ID:       stringValue(values, "permissionId", "permission_id", "toolCallId", "tool_call_id", "id"),
		Action:   stringValue(values, "action", "operation", "toolName", "name", "title"),
		Resource: stringValue(values, "resource", "path", "command", "url"),
		Reason:   stringValue(values, "reason", "message", "description"),
		Metadata: withoutKeys(values, "permissionId", "permission_id", "toolCallId", "tool_call_id", "id", "action", "operation", "toolName", "name", "title", "resource", "path", "command", "url", "reason", "message", "description", "options"),
	}
	if options, ok := values["options"].([]any); ok {
		permission.Options = parseOptions(options)
	}
	return permission
}
