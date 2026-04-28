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

// CoverageReport is the full JSON report structure.
type CoverageReport struct {
	GeneratedAt      time.Time                             `json:"generatedAt"`
	TargetMCP        string                                `json:"targetMcp"`
	ScannerUsed      string                                `json:"scannerUsed"`
	Metrics          coverage.Metrics                      `json:"metrics"`
	ModuleCoverage   []*coverage.ModuleMetrics             `json:"moduleCoverage"`
	ControllerCoverage []*coverage.ControllerMetrics       `json:"controllerCoverage"`
	Results          []mapping.MappingResult               `json:"results"`
	MCPTools         []mcpclient.ToolEntry                 `json:"mcpTools"`
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
	return &CoverageReport{
		GeneratedAt:        time.Now().UTC(),
		TargetMCP:          targetMCP,
		ScannerUsed:        scanner,
		Metrics:            metrics,
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
