package apiscanner

// APIEntry represents a single backend Spring Controller API endpoint.
type APIEntry struct {
	Module     string `json:"module"`
	Controller string `json:"controller"`
	HTTPMethod string `json:"httpMethod"`
	APIPath    string `json:"apiPath"`
	MethodName string `json:"methodName"`
	Summary    string `json:"summary,omitempty"`
	SourceFile string `json:"sourceFile,omitempty"` // relative path (JavaSource scanner only)
	LineNumber int    `json:"lineNumber,omitempty"` // 1-based (JavaSource scanner only)
	// ScanStatus is "partial" when path or HTTP method could not be fully resolved
	// (e.g. path is a constant reference). Empty means fully resolved.
	ScanStatus string `json:"scanStatus,omitempty"`
	// ScanReason explains why ScanStatus is "partial".
	ScanReason string `json:"scanReason,omitempty"`
}

// Scanner is the interface for collecting backend API entries.
type Scanner interface {
	Scan() ([]APIEntry, error)
	Name() string
}
