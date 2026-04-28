// Package javasource provides a Spring Controller source-code scanner.
// It recursively walks a project directory, parses Java files to detect
// @RestController / @Controller classes and their mapping annotations, and
// returns a complete list of APIEntry values regardless of whether the APIs
// are exposed in Swagger or have any MCP Tool mapping.
//
// Discovery strategy (highest priority first):
//  1. Java source annotation scanning (primary)
//  2. Spring Actuator /actuator/mappings endpoint (optional, merged)
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
	ActuatorURL               string   // optional: base URL of running Spring app for /actuator/mappings
	Debug                     bool     // print debug stats to stderr
}

// DebugStats holds counters collected during a scan run.
type DebugStats struct {
	ScannedFiles        int
	SkippedFiles        int
	ControllerCount     int
	InterfaceControllers int
	AbstractControllers  int
	MethodsInspected    int
	DetectedAPIs        int
	ExcludedAPIs        int
	UnresolvedPaths     int
	ActuatorOnlyAPIs    int
	DuplicatePaths      []string
	SkipReasons         map[string]int // reason → count
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

// Scan walks the project directory and returns the full set of detected API entries.
//
// Discovery order:
//  1. Build a constant registry (resolves path constants like ApiPaths.PATIENT)
//  2. Parse all Java source files for Spring controller annotations
//  3. Merge /actuator/mappings results if ActuatorURL is configured
func (s *JavaSourceScanner) Scan() ([]apiscanner.APIEntry, error) {
	stats := &DebugStats{SkipReasons: make(map[string]int)}
	seen := make(map[string]int) // "METHOD /path" → count

	// Phase 1: build constant registry across the whole project.
	reg := BuildConstantRegistry(s.cfg.ProjectPath)
	if s.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[javasource] constant registry: %d entries\n", len(reg))
	}

	var entries []apiscanner.APIEntry

	// Phase 2: source scan.
	err := filepath.WalkDir(s.cfg.ProjectPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
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

		ctrl := ParseFileWithRegistry(path, string(src), reg)
		if ctrl == nil {
			return nil
		}
		stats.ControllerCount++
		if ctrl.IsInterface {
			stats.InterfaceControllers++
		}
		if ctrl.IsAbstract {
			stats.AbstractControllers++
		}
		stats.MethodsInspected += len(ctrl.Methods)

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

				if seen[key] > 0 {
					stats.DuplicatePaths = append(stats.DuplicatePaths, key)
				}
				seen[key]++

				// Determine scan status.
				scanStatus, scanReason := "", ""
				isUnresolvedPath := strings.HasPrefix(apiPath, "UNRESOLVED:") ||
					ctrl.BasePathUnresolved
				if isUnresolvedPath {
					scanStatus = "partial"
					stats.UnresolvedPaths++
					if strings.HasPrefix(apiPath, "UNRESOLVED:") {
						scanReason = "method path constant unresolved: " + method.UnresolvedRef
					} else {
						scanReason = "base path constant unresolved: " + ctrl.BasePathRef
					}
				}

				entries = append(entries, apiscanner.APIEntry{
					Module:     deriveModule(rel, apiPath),
					Controller: ctrl.ClassName,
					HTTPMethod: method.HTTPMethod,
					APIPath:    apiPath,
					MethodName: method.MethodName,
					SourceFile: rel,
					LineNumber: method.LineNumber,
					ScanStatus: scanStatus,
					ScanReason: scanReason,
				})
				stats.DetectedAPIs++
			}
		}
		return nil
	})

	// Phase 3: optional actuator merge.
	if s.cfg.ActuatorURL != "" {
		actuatorEntries, actuatorErr := ScanActuator(s.cfg.ActuatorURL)
		if actuatorErr != nil {
			if s.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[javasource] actuator scan failed: %v\n", actuatorErr)
			}
		} else {
			before := len(entries)
			entries = mergeWithActuator(entries, actuatorEntries, seen)
			stats.ActuatorOnlyAPIs = len(entries) - before
			if s.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[javasource] actuator: %d new APIs added\n", stats.ActuatorOnlyAPIs)
			}
		}
	}

	if s.cfg.Debug {
		printDebug(s.cfg.ProjectPath, stats)
	}

	return entries, err
}

// mergeWithActuator adds actuator-discovered entries that were not found by source scan.
func mergeWithActuator(primary []apiscanner.APIEntry, actuator []apiscanner.APIEntry, seen map[string]int) []apiscanner.APIEntry {
	merged := append([]apiscanner.APIEntry(nil), primary...)
	for _, e := range actuator {
		key := e.HTTPMethod + " " + e.APIPath
		if seen[key] == 0 {
			e.ScanStatus = "actuator-only"
			e.ScanReason = "discovered via /actuator/mappings, not found in source scan"
			merged = append(merged, e)
			seen[key]++
		}
	}
	return merged
}

// ── path combination ───────────────────────────────────────────────────────

// combinePaths returns the Cartesian product of base × method paths, normalised.
// UNRESOLVED paths are preserved as-is for transparency.
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
	// Preserve UNRESOLVED markers.
	baseUnresolved := strings.HasPrefix(base, "UNRESOLVED:")
	methodUnresolved := strings.HasPrefix(method, "UNRESOLVED:")

	if baseUnresolved && methodUnresolved {
		return "UNRESOLVED:" + base[len("UNRESOLVED:"):] + "+" + method[len("UNRESOLVED:"):]
	}
	if baseUnresolved {
		if method == "" {
			return base
		}
		return base + method
	}
	if methodUnresolved {
		if base == "" {
			return method
		}
		return base + "+" + method
	}

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
func deriveModule(relFile, apiPath string) string {
	norm := filepath.ToSlash(relFile)
	if idx := strings.Index(norm, "src/main/java/"); idx >= 0 {
		pkg := norm[idx+len("src/main/java/"):]
		parts := strings.Split(pkg, "/")
		if len(parts) >= 2 {
			return parts[len(parts)-2]
		}
	}
	apiPath = strings.TrimPrefix(apiPath, "/")
	apiPath = strings.TrimPrefix(apiPath, "UNRESOLVED:")
	if idx := strings.Index(apiPath, "/"); idx > 0 {
		return apiPath[:idx]
	}
	return strings.TrimSuffix(apiPath, "/")
}

// ── debug output ───────────────────────────────────────────────────────────

func printDebug(projectPath string, s *DebugStats) {
	fmt.Fprintf(os.Stderr, "\n[JavaSource Debug] ─────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "  Project path          : %s\n", projectPath)
	fmt.Fprintf(os.Stderr, "  Scanned .java files   : %d\n", s.ScannedFiles)
	fmt.Fprintf(os.Stderr, "  Skipped files         : %d\n", s.SkippedFiles)
	for reason, count := range s.SkipReasons {
		fmt.Fprintf(os.Stderr, "    %-22s: %d\n", reason, count)
	}
	fmt.Fprintf(os.Stderr, "  Controllers found     : %d\n", s.ControllerCount)
	fmt.Fprintf(os.Stderr, "    interfaces          : %d\n", s.InterfaceControllers)
	fmt.Fprintf(os.Stderr, "    abstract classes    : %d\n", s.AbstractControllers)
	fmt.Fprintf(os.Stderr, "  Methods inspected     : %d\n", s.MethodsInspected)
	fmt.Fprintf(os.Stderr, "  APIs detected         : %d\n", s.DetectedAPIs)
	fmt.Fprintf(os.Stderr, "  APIs excluded         : %d\n", s.ExcludedAPIs)
	fmt.Fprintf(os.Stderr, "  Unresolved paths      : %d\n", s.UnresolvedPaths)
	fmt.Fprintf(os.Stderr, "  Actuator-only APIs    : %d\n", s.ActuatorOnlyAPIs)
	if len(s.DuplicatePaths) > 0 {
		fmt.Fprintf(os.Stderr, "  Duplicate paths       : %d\n", len(s.DuplicatePaths))
		for _, dup := range s.DuplicatePaths {
			fmt.Fprintf(os.Stderr, "    %s\n", dup)
		}
	}
	fmt.Fprintf(os.Stderr, "────────────────────────────────────────────────\n")
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
