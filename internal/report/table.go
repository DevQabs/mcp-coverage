package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"mcp-coverage/internal/coverage"
	"mcp-coverage/internal/mapping"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// PrintTable writes a human-readable coverage table to w.
func PrintTable(w io.Writer, results []mapping.MappingResult, metrics coverage.Metrics,
	byModule map[string]*coverage.ModuleMetrics, targetMCP string) {

	fmt.Fprintf(w, "\n%s═══ MCP Coverage Report — %s %s\n\n", colorBold, targetMCP, colorReset)

	// Summary metrics.
	fmt.Fprintf(w, "  Total APIs        : %d\n", metrics.Total)
	fmt.Fprintf(w, "  %sMapped            : %d%s\n", colorGreen, metrics.Mapped, colorReset)
	fmt.Fprintf(w, "  %sReview Required   : %d%s\n", colorYellow, metrics.ReviewRequired, colorReset)
	fmt.Fprintf(w, "  %sUnmapped          : %d%s\n", colorRed, metrics.Unmapped, colorReset)
	fmt.Fprintf(w, "  Coverage Rate     : %s%.1f%%%s\n\n",
		rateColor(metrics.CoverageRate), metrics.CoverageRate, colorReset)

	// Module summary.
	fmt.Fprintf(w, "%s── Coverage by Module %s\n", colorCyan, colorReset)
	mods := sortedModuleKeys(byModule)
	for _, mod := range mods {
		m := byModule[mod]
		fmt.Fprintf(w, "  %-30s %s%5.1f%%%s  (%d/%d mapped, %d review, %d unmapped)\n",
			mod,
			rateColor(m.CoverageRate), m.CoverageRate, colorReset,
			m.Mapped, m.Total, m.ReviewRequired, m.Unmapped,
		)
	}

	// Detail table.
	fmt.Fprintf(w, "\n%s── API Detail Table %s\n\n", colorCyan, colorReset)
	printDetailTable(w, results)
}

func printDetailTable(w io.Writer, results []mapping.MappingResult) {
	const (
		wModule  = 18
		wCtrl    = 28
		wMethod  = 7
		wPath    = 50
		wMName   = 30
		wTool    = 36
		wStatus  = 17
		wRemark  = 40
	)

	header := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s",
		wModule, "MODULE",
		wCtrl, "CONTROLLER",
		wMethod, "METHOD",
		wPath, "API_PATH",
		wMName, "METHOD_NAME",
		wTool, "MCP_TOOL",
		wStatus, "STATUS",
		wRemark, "REMARK",
	)
	sep := strings.Repeat("─", len(header))
	fmt.Fprintln(w, header)
	fmt.Fprintln(w, sep)

	for _, r := range results {
		color := statusColor(r.MappingStatus)
		fmt.Fprintf(w, "%-*s %-*s %-*s %-*s %-*s %s%-*s %-*s%s %-*s\n",
			wModule, truncate(r.Module, wModule),
			wCtrl, truncate(r.Controller, wCtrl),
			wMethod, r.HTTPMethod,
			wPath, truncate(r.APIPath, wPath),
			wMName, truncate(r.MethodName, wMName),
			color, wTool, truncate(r.MCPToolName, wTool),
			wStatus, r.MappingStatus, colorReset,
			wRemark, truncate(r.Remark, wRemark),
		)
	}
	fmt.Fprintln(w)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func statusColor(status string) string {
	switch status {
	case mapping.StatusMapped:
		return colorGreen
	case mapping.StatusReviewRequired:
		return colorYellow
	default:
		return colorRed
	}
}

func rateColor(rate float64) string {
	switch {
	case rate >= 80:
		return colorGreen
	case rate >= 50:
		return colorYellow
	default:
		return colorRed
	}
}

func sortedModuleKeys(m map[string]*coverage.ModuleMetrics) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
