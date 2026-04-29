package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	TargetMCPName     string
	TargetProjectPath string
	ExcludeAPIPatterns        []string
	ExcludeControllerPatterns []string
	ReportFormat string
	Filter       string
	OutputDir    string
	MetadataDir  string
	AdminHTTP bool
	AdminPort string
	Debug bool
}

func Load() (*Config, error) {
	name := os.Getenv("TARGET_MCP_NAME")
	if name == "" {
		return nil, fmt.Errorf("TARGET_MCP_NAME environment variable is required")
	}

	projectPath := os.Getenv("TARGET_PROJECT_PATH")
	if projectPath == "" {
		return nil, fmt.Errorf("TARGET_PROJECT_PATH environment variable is required")
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
		TargetProjectPath:         projectPath,
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
