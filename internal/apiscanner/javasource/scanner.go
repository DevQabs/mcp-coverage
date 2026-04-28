// Package javasource provides a Spring Controller source-code scanner.
// It recursively walks a project directory, parses Java files to detect
// @RestController / @Controller classes and their mapping annotations, and
// returns a complete list of APIEntry values regardless of whether the APIs
// are exposed in Swagger or have any MCP Tool mapping.
package javasource

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mcp-coverage/internal/apiscanner"
)

// Config is the configuration for the Java source scanner.
type Config struct {
	ProjectPath               string   // root of the Spring project to scan
	ExcludeAPIPatterns        []string // glob patterns for API paths to exclude
	ExcludeControllerPatterns []string // glob patterns for controller class names to exclude
	Debug                     bool     // print debug stats to stderr
}

// DebugStats holds counters collected during a scan run.
type DebugStats struct {
	ScannedFiles    int
	SkippedFiles    int
	ControllerCount int
	DetectedAPIs    int
	ExcludedAPIs    int
	DuplicatePaths  []string
	SkipReasons     map[string]int // reason → count
}

// JavaSourceScanner implements apiscanner.Scanner by parsing Java source files.
type JavaSourceScanner struct {
	cfg Config
}

// ScannedEntry is an exported alias for apiscanner.APIEntry for use in tests.
type ScannedEntry = apiscanner.APIEntry

// New creates a JavaSourceScanner with the given config.
func New(cfg Config) *JavaSourceScanner {
	return &JavaSourceScanner{cfg: cfg}
}

func (s *JavaSourceScanner) Name() string { return "JavaSource" }

// Scan walks the project directory, parses all Java files, and returns the full
// set of detected API entries after applying configured exclusions.
func (s *JavaSourceScanner) Scan() ([]apiscanner.APIEntry, error) {
	stats := &DebugStats{SkipReasons: make(map[string]int)}
	seen := make(map[string]int) // "METHOD /path" → count, for duplicate detection

	var entries []apiscanner.APIEntry

	err := filepath.WalkDir(s.cfg.ProjectPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			// Skip hidden and common non-source directories.
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "build" || name == "target" ||
				name == "node_modules" || name == "out" || name == ".gradle" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".java") {
			stats.SkippedFiles++
			stats.SkipReasons["not .java"]++
			return nil
		}

		stats.ScannedFiles++

		src, err := os.ReadFile(path)
		if err != nil {
			stats.SkipReasons["read error"]++
			return nil
		}

		ctrl := ParseFile(path, string(src))
		if ctrl == nil {
			return nil // not a controller
		}
		stats.ControllerCount++

		// Controller-level exclusion.
		if matchAny(s.cfg.ExcludeControllerPatterns, ctrl.ClassName) {
			count := countAPIs(ctrl)
			stats.ExcludedAPIs += count
			if s.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[javasource] EXCLUDED controller %s (%d APIs)\n",
					ctrl.ClassName, count)
			}
			return nil
		}

		rel, _ := filepath.Rel(s.cfg.ProjectPath, path)

		for _, method := range ctrl.Methods {
			fullPaths := combinePaths(ctrl.BasePaths, method.Paths)
			for _, apiPath := range fullPaths {
				key := method.HTTPMethod + " " + apiPath

				if matchAny(s.cfg.ExcludeAPIPatterns, apiPath) {
					stats.ExcludedAPIs++
					continue
				}

				// Track duplicates (same HTTP method + path from different controllers).
				if seen[key] > 0 {
					stats.DuplicatePaths = append(stats.DuplicatePaths, key)
				}
				seen[key]++

				entry := apiscanner.APIEntry{
					Module:     deriveModule(rel, apiPath),
					Controller: ctrl.ClassName,
					HTTPMethod: method.HTTPMethod,
					APIPath:    apiPath,
					MethodName: method.MethodName,
					SourceFile: rel,
					LineNumber: method.LineNumber,
				}
				entries = append(entries, entry)
				stats.DetectedAPIs++
			}
		}
		return nil
	})

	if s.cfg.Debug {
		printDebug(s.cfg.ProjectPath, stats)
	}

	return entries, err
}

// ── path combination ───────────────────────────────────────────────────────

// combinePaths returns the Cartesian product of base × method paths, normalized.
// If either slice is empty, the other is used as-is.
func combinePaths(basePaths, methodPaths []string) []string {
	if len(basePaths) == 0 {
		basePaths = []string{""}
	}
	if len(methodPaths) == 0 {
		methodPaths = []string{""}
	}

	var out []string
	for _, base := range basePaths {
		for _, method := range methodPaths {
			out = append(out, joinPaths(base, method))
		}
	}
	return out
}

func joinPaths(base, method string) string {
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(base, "/") && base != "" {
		base = "/" + base
	}
	if method != "" && !strings.HasPrefix(method, "/") {
		method = "/" + method
	}
	result := base + method
	if result == "" {
		return "/"
	}
	if !strings.HasPrefix(result, "/") {
		result = "/" + result
	}
	return result
}

// ── module derivation ──────────────────────────────────────────────────────

// deriveModule returns a module name from the source file path or API path.
// Prefers the meaningful package segment after src/main/java.
func deriveModule(relFile, apiPath string) string {
	// Try src/main/java/.../<package>/<File>.java
	norm := filepath.ToSlash(relFile)
	if idx := strings.Index(norm, "src/main/java/"); idx >= 0 {
		pkg := norm[idx+len("src/main/java/"):]
		parts := strings.Split(pkg, "/")
		// Take the last package component before the file name.
		if len(parts) >= 2 {
			return parts[len(parts)-2]
		}
	}
	// Fall back to first API path segment.
	apiPath = strings.TrimPrefix(apiPath, "/")
	if idx := strings.Index(apiPath, "/"); idx > 0 {
		return apiPath[:idx]
	}
	return strings.TrimSuffix(apiPath, "/")
}

// ── debug output ───────────────────────────────────────────────────────────

func printDebug(projectPath string, s *DebugStats) {
	fmt.Fprintf(os.Stderr, "\n[JavaSource Debug]\n")
	fmt.Fprintf(os.Stderr, "  Project path      : %s\n", projectPath)
	fmt.Fprintf(os.Stderr, "  Scanned .java     : %d\n", s.ScannedFiles)
	fmt.Fprintf(os.Stderr, "  Skipped files     : %d\n", s.SkippedFiles)
	for reason, count := range s.SkipReasons {
		fmt.Fprintf(os.Stderr, "    %-20s: %d\n", reason, count)
	}
	fmt.Fprintf(os.Stderr, "  Controllers found : %d\n", s.ControllerCount)
	fmt.Fprintf(os.Stderr, "  APIs detected     : %d\n", s.DetectedAPIs)
	fmt.Fprintf(os.Stderr, "  APIs excluded     : %d\n", s.ExcludedAPIs)
	if len(s.DuplicatePaths) > 0 {
		fmt.Fprintf(os.Stderr, "  Duplicate paths   : %d\n", len(s.DuplicatePaths))
		for _, dup := range s.DuplicatePaths {
			fmt.Fprintf(os.Stderr, "    %s\n", dup)
		}
	}
}

// countAPIs counts how many API entries a controller would produce.
func countAPIs(ctrl *ControllerDef) int {
	count := 0
	basePaths := ctrl.BasePaths
	if len(basePaths) == 0 {
		basePaths = []string{""}
	}
	for _, m := range ctrl.Methods {
		methodPaths := m.Paths
		if len(methodPaths) == 0 {
			methodPaths = []string{""}
		}
		count += len(basePaths) * len(methodPaths)
	}
	return count
}
