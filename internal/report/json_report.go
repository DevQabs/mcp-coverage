package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"mcp-coverage/internal/coverage"
	"mcp-coverage/internal/mapping"
	"mcp-coverage/internal/mcpclient"
)

// Summary is the top-level coverage metrics block in the JSON report.
type Summary struct {
	TotalAPICount      int     `json:"totalApiCount"`
	MappedAPICount     int     `json:"mappedApiCount"`
	ReviewRequiredCount int    `json:"reviewRequiredCount"`
	UnmappedAPICount   int     `json:"unmappedApiCount"`
	CoverageRate       float64 `json:"coverageRate"`
}

// UnmappedAPI is a simplified view of an unmapped result for quick triage.
type UnmappedAPI struct {
	HTTPMethod     string  `json:"httpMethod"`
	APIPath        string  `json:"apiPath"`
	Module         string  `json:"module"`
	ControllerName string  `json:"controllerName"`
	MethodName     string  `json:"methodName"`
	SourceFile     string  `json:"sourceFile,omitempty"`
	LineNumber     int     `json:"lineNumber,omitempty"`
	MCPToolName    *string `json:"mcpToolName"` // always null for unmapped
	Status         string  `json:"status"`
	Reason         string  `json:"reason"`
}

// CoverageReport is the full JSON report structure.
type CoverageReport struct {
	GeneratedAt         time.Time                          `json:"generatedAt"`
	TargetMCP           string                             `json:"targetMcp"`
	ScannerUsed         string                             `json:"scannerUsed"`
	Summary             Summary                            `json:"summary"`
	UnmappedAPIs        []UnmappedAPI                      `json:"unmappedApis"`
	ModuleCoverage      []*coverage.ModuleMetrics          `json:"moduleCoverage"`
	ControllerCoverage  []*coverage.ControllerMetrics      `json:"controllerCoverage"`
	Results             []mapping.MappingResult            `json:"results"`
	MCPTools            []mcpclient.ToolEntry              `json:"mcpTools"`
}

// BuildReport assembles the full report.
func BuildReport(
	targetMCP, scanner string,
	results []mapping.MappingResult,
	tools []mcpclient.ToolEntry,
	metrics coverage.Metrics,
	byModule map[string]*coverage.ModuleMetrics,
	byController map[string]*coverage.ControllerMetrics,
) *CoverageReport {
	summary := Summary{
		TotalAPICount:       metrics.Total,
		MappedAPICount:      metrics.Mapped,
		ReviewRequiredCount: metrics.ReviewRequired,
		UnmappedAPICount:    metrics.Unmapped,
		CoverageRate:        metrics.CoverageRate,
	}

	unmapped := buildUnmappedList(results)

	return &CoverageReport{
		GeneratedAt:        time.Now().UTC(),
		TargetMCP:          targetMCP,
		ScannerUsed:        scanner,
		Summary:            summary,
		UnmappedAPIs:       unmapped,
		ModuleCoverage:     sortedModules(byModule),
		ControllerCoverage: sortedControllers(byController),
		Results:            results,
		MCPTools:           tools,
	}
}

// WriteJSON serializes the report to outputDir/coverage_report.json.
func WriteJSON(report *CoverageReport, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	path := filepath.Join(outputDir, "coverage_report.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ── helpers ────────────────────────────────────────────────────────────────

func buildUnmappedList(results []mapping.MappingResult) []UnmappedAPI {
	var list []UnmappedAPI
	for _, r := range results {
		if r.MappingStatus != mapping.StatusUnmapped {
			continue
		}
		var toolName *string // nil = JSON null
		list = append(list, UnmappedAPI{
			HTTPMethod:     r.HTTPMethod,
			APIPath:        r.APIPath,
			Module:         r.Module,
			ControllerName: r.Controller,
			MethodName:     r.MethodName,
			SourceFile:     r.SourceFile,
			LineNumber:     r.LineNumber,
			MCPToolName:    toolName,
			Status:         mapping.StatusUnmapped,
			Reason:         "No matching MCP Tool found",
		})
	}
	if list == nil {
		list = []UnmappedAPI{} // ensure JSON array, not null
	}
	return list
}

func sortedModules(m map[string]*coverage.ModuleMetrics) []*coverage.ModuleMetrics {
	list := make([]*coverage.ModuleMetrics, 0, len(m))
	for _, v := range m {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Module < list[j].Module })
	return list
}

func sortedControllers(m map[string]*coverage.ControllerMetrics) []*coverage.ControllerMetrics {
	list := make([]*coverage.ControllerMetrics, 0, len(m))
	for _, v := range m {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Controller < list[j].Controller })
	return list
}
