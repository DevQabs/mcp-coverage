// Package api provides an optional HTTP admin API for querying coverage data.
// Enable with ADMIN_HTTP=true. Default port: 8080 (ADMIN_PORT).
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"mcp-coverage/internal/coverage"
	"mcp-coverage/internal/mapping"
	"mcp-coverage/internal/report"
)

// Server holds the live coverage data and serves query endpoints.
type Server struct {
	port       string
	results    []mapping.MappingResult
	metrics    coverage.Metrics
	byModule   map[string]*coverage.ModuleMetrics
	byCtrl     map[string]*coverage.ControllerMetrics
	fullReport *report.CoverageReport
}

// New creates an admin HTTP server with coverage data.
func New(
	port string,
	results []mapping.MappingResult,
	metrics coverage.Metrics,
	byModule map[string]*coverage.ModuleMetrics,
	byCtrl map[string]*coverage.ControllerMetrics,
	fullReport *report.CoverageReport,
) *Server {
	return &Server{
		port:       port,
		results:    results,
		metrics:    metrics,
		byModule:   byModule,
		byCtrl:     byCtrl,
		fullReport: fullReport,
	}
}

// Run starts the HTTP server (blocking).
func (s *Server) Run() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/coverage", s.handleSummary)
	mux.HandleFunc("/coverage/results", s.handleResults)
	mux.HandleFunc("/coverage/unmapped", s.handleUnmapped)
	mux.HandleFunc("/coverage/modules", s.handleModules)
	mux.HandleFunc("/coverage/controllers", s.handleControllers)
	mux.HandleFunc("/coverage/report", s.handleFullReport)

	fmt.Printf("Admin API listening on :%s\n", s.port)
	fmt.Println("  GET /coverage                              — summary metrics")
	fmt.Println("  GET /coverage/results[?status=mapped|review_required|unmapped]")
	fmt.Println("                                              — mapping results with optional status filter")
	fmt.Println("  GET /coverage/unmapped                     — unmapped APIs only (shortcut)")
	fmt.Println("  GET /coverage/modules                      — per-module metrics")
	fmt.Println("  GET /coverage/controllers                  — per-controller metrics")
	fmt.Println("  GET /coverage/report                       — full JSON report")
	return http.ListenAndServe(":"+s.port, mux)
}

// ── handlers ───────────────────────────────────────────────────────────────

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.fullReport.Summary)
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	// Support ?status= (new) and ?filter= (legacy) query params.
	status := r.URL.Query().Get("status")
	if status == "" {
		status = r.URL.Query().Get("filter")
	}

	var results []mapping.MappingResult
	switch strings.ToLower(status) {
	case "unmapped":
		results = filterByStatus(s.results, mapping.StatusUnmapped)
	case "review_required", "review-required":
		results = filterByStatus(s.results, mapping.StatusReviewRequired)
	case "mapped":
		results = filterByStatus(s.results, mapping.StatusMapped)
	default:
		results = s.results
	}
	writeJSON(w, results)
}

func (s *Server) handleUnmapped(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.fullReport.UnmappedAPIs)
}

func (s *Server) handleModules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.fullReport.ModuleCoverage)
}

func (s *Server) handleControllers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.fullReport.ControllerCoverage)
}

func (s *Server) handleFullReport(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.fullReport)
}

// ── helpers ────────────────────────────────────────────────────────────────

func filterByStatus(results []mapping.MappingResult, status string) []mapping.MappingResult {
	var out []mapping.MappingResult
	for _, r := range results {
		if r.MappingStatus == status {
			out = append(out, r)
		}
	}
	if out == nil {
		out = []mapping.MappingResult{}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
