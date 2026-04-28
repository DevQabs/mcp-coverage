package apiscanner

// APIEntry represents a single backend Spring Controller API endpoint.
type APIEntry struct {
	Module     string `json:"module"`
	Controller string `json:"controller"`
	HTTPMethod string `json:"httpMethod"`
	APIPath    string `json:"apiPath"`
	MethodName string `json:"methodName"`
	Summary    string `json:"summary"`
}

// Scanner is the interface for collecting backend API entries.
type Scanner interface {
	Scan() ([]APIEntry, error)
	Name() string
}
