package report

import (
	"strings"

	"mcp-coverage/internal/mapping"
)

// Filter returns only the results matching the filter expression.
// Supported values: ALL, UNMAPPED, REVIEW_REQUIRED, MAPPED.
// Module/controller filters use the format "module:<name>" or "controller:<name>".
func Filter(results []mapping.MappingResult, filter string) []mapping.MappingResult {
	filter = strings.ToUpper(strings.TrimSpace(filter))
	if filter == "" || filter == "ALL" {
		return results
	}

	if strings.HasPrefix(filter, "MODULE:") {
		mod := strings.TrimPrefix(filter, "MODULE:")
		return filterByModule(results, mod)
	}
	if strings.HasPrefix(filter, "CONTROLLER:") {
		ctrl := strings.TrimPrefix(filter, "CONTROLLER:")
		return filterByController(results, ctrl)
	}

	// Status filter.
	var status string
	switch filter {
	case "UNMAPPED":
		status = mapping.StatusUnmapped
	case "REVIEW_REQUIRED":
		status = mapping.StatusReviewRequired
	case "MAPPED":
		status = mapping.StatusMapped
	default:
		return results
	}

	var out []mapping.MappingResult
	for _, r := range results {
		if r.MappingStatus == status {
			out = append(out, r)
		}
	}
	return out
}

func filterByModule(results []mapping.MappingResult, module string) []mapping.MappingResult {
	var out []mapping.MappingResult
	for _, r := range results {
		if strings.EqualFold(r.Module, module) {
			out = append(out, r)
		}
	}
	return out
}

func filterByController(results []mapping.MappingResult, controller string) []mapping.MappingResult {
	var out []mapping.MappingResult
	for _, r := range results {
		if strings.EqualFold(r.Controller, controller) {
			out = append(out, r)
		}
	}
	return out
}
