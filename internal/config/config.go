package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	// Target MCP server (required)
	TargetMCPName string

	// API source — priority: JavaSource > OpenAPI > Static
	TargetProjectPath string // TARGET_PROJECT_PATH — scan Spring source directly
	SwaggerURL        string // SWAGGER_URL — fetch live OpenAPI spec
	ActuatorURL       string // ACTUATOR_URL — optional running app for /actuator/mappings merge

	// Exclusions (JavaSource scanner only)
	ExcludeAPIPatterns        []string // EXCLUDE_API_PATTERNS comma-separated glob patterns
	ExcludeControllerPatterns []string // EXCLUDE_CONTROLLER_PATTERNS comma-separated glob patterns

	// Output
	ReportFormat string // TABLE | JSON | BOTH
	Filter       string // ALL | MAPPED | UNMAPPED | REVIEW_REQUIRED | MODULE:<n> | CONTROLLER:<n>
	OutputDir    string // directory for coverage_report.json
	MetadataDir  string // directory for apis.json and tools_metadata.json

	// Admin HTTP API
	AdminHTTP bool
	AdminPort string

	// Diagnostics
	Debug bool // DEBUG=true prints detailed scanner stats
}

func Load() (*Config, error) {
	name := os.Getenv("TARGET_MCP_NAME")
	if name == "" {
		return nil, fmt.Errorf("TARGET_MCP_NAME environment variable is required")
	}

	adminHTTP, _ := strconv.ParseBool(os.Getenv("ADMIN_HTTP"))
	debug, _ := strconv.ParseBool(os.Getenv("DEBUG"))

	reportFmt := envOr("REPORT_FORMAT", "BOTH")
	filter := envOr("FILTER", "ALL")
	adminPort := envOr("ADMIN_PORT", "8080")
	outputDir := envOr("OUTPUT_DIR", "./reports")
	metaDir := envOr("METADATA_DIR", defaultMetadataDir())

	return &Config{
		TargetMCPName:             name,
		TargetProjectPath:         os.Getenv("TARGET_PROJECT_PATH"),
		SwaggerURL:                os.Getenv("SWAGGER_URL"),
		ActuatorURL:               os.Getenv("ACTUATOR_URL"),
		ExcludeAPIPatterns:        splitPatterns(os.Getenv("EXCLUDE_API_PATTERNS")),
		ExcludeControllerPatterns: splitPatterns(os.Getenv("EXCLUDE_CONTROLLER_PATTERNS")),
		ReportFormat:              reportFmt,
		Filter:                    filter,
		OutputDir:                 outputDir,
		MetadataDir:               metaDir,
		AdminHTTP:                 adminHTTP,
		AdminPort:                 adminPort,
		Debug:                     debug,
	}, nil
}

// ── helpers ────────────────────────────────────────────────────────────────

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitPatterns(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func defaultMetadataDir() string {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "metadata")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "./metadata"
}
