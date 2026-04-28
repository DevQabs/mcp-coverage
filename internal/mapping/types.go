package mapping

import "mcp-coverage/internal/apiscanner"

const (
	StatusMapped         = "mapped"
	StatusReviewRequired = "review_required"
	StatusUnmapped       = "unmapped"
)

// ToolMetadata is the explicit mapping between an MCP tool and backend APIs.
// Stored in metadata/tools_metadata.json.
type ToolMetadata struct {
	// Single-API mapping (use when tool maps exactly one API).
	APIPath    string `json:"apiPath,omitempty"`
	HTTPMethod string `json:"httpMethod,omitempty"`
	Controller string `json:"controllerName,omitempty"`
	MethodName string `json:"methodName,omitempty"`

	// Multi-API mapping (use when tool calls multiple backend APIs).
	APIs []APIRef `json:"apis,omitempty"`
}

// APIRef is one entry in a multi-API mapping.
type APIRef struct {
	APIPath    string `json:"apiPath"`
	HTTPMethod string `json:"httpMethod"`
	Controller string `json:"controllerName,omitempty"`
	MethodName string `json:"methodName,omitempty"`
	Note       string `json:"note,omitempty"`
}

// MappingResult is the output row for one API entry.
type MappingResult struct {
	apiscanner.APIEntry
	MCPToolName   string `json:"mcpToolName"`
	MappingStatus string `json:"mappingStatus"`
	Remark        string `json:"remark"`
}
