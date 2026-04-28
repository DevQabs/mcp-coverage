// Package api provides an optional HTTP admin API for querying coverage data.
// Enable with ADMIN_HTTP=true. Default port: 8080 (ADMIN_PORT).
package api

import (
	"encoding/json"
	"fmt"
	"net/http"

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
	mux.HandleFunc("/coverage", s.handleCoverage)
	mux.HandleFunc("/coverage/results", s.handleResults)
	mux.HandleFunc("/coverage/modules", s.handleModules)
	mux.HandleFunc("/coverage/controllers", s.handleControllers)
	mux.HandleFunc("/coverage/report", s.handleFullReport)

	fmt.Printf("Admin API listening on :%s\n", s.port)
	fmt.Println("  GET /coverage              — summary metrics")
	fmt.Println("  GET /coverage/results       — all mapping results (filter=UNMAPPED|REVIEW_REQUIRED|MAPPED|MODULE:x|CONTROLLER:x)")
	fmt.Println("  GET /coverage/modules       — per-module metrics")
	fmt.Println("  GET /coverage/controllers   — per-controller metrics")
	fmt.Println("  GET /coverage/report        — full JSON report")
	return http.ListenAndServe(":"+s.port, mux)
}

// ── handlers ───────────────────────────────────────────────────────────────

func (s *Server) handleCoverage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.metrics)
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	results := report.Filter(s.results, filter)
	writeJSON(w, results)
}

func (s *Server) handleModules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.byModule)
}

func (s *Server) handleControllers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.byCtrl)
}

func (s *Server) handleFullReport(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.fullReport)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
