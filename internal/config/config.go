package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	TargetMCPName string // TARGET_MCP_NAME
	SwaggerURL    string // SWAGGER_URL (optional; triggers OpenAPI scanner if set)
	ReportFormat  string // TABLE | JSON | BOTH (default: BOTH)
	Filter        string // ALL | UNMAPPED | REVIEW_REQUIRED (default: ALL)
	AdminHTTP     bool   // ADMIN_HTTP=true enables HTTP admin API
	AdminPort     string // ADMIN_PORT (default: 8080)
	MetadataDir   string // METADATA_DIR (default: ./metadata)
	OutputDir     string // OUTPUT_DIR (default: ./reports)
}

func Load() (*Config, error) {
	name := os.Getenv("TARGET_MCP_NAME")
	if name == "" {
		return nil, fmt.Errorf("TARGET_MCP_NAME environment variable is required")
	}

	adminHTTP, _ := strconv.ParseBool(os.Getenv("ADMIN_HTTP"))

	metaDir := os.Getenv("METADATA_DIR")
	if metaDir == "" {
		metaDir = defaultMetadataDir()
	}

	outDir := os.Getenv("OUTPUT_DIR")
	if outDir == "" {
		outDir = "./reports"
	}

	reportFmt := os.Getenv("REPORT_FORMAT")
	if reportFmt == "" {
		reportFmt = "BOTH"
	}

	filter := os.Getenv("FILTER")
	if filter == "" {
		filter = "ALL"
	}

	adminPort := os.Getenv("ADMIN_PORT")
	if adminPort == "" {
		adminPort = "8080"
	}

	return &Config{
		TargetMCPName: name,
		SwaggerURL:    os.Getenv("SWAGGER_URL"),
		ReportFormat:  reportFmt,
		Filter:        filter,
		AdminHTTP:     adminHTTP,
		AdminPort:     adminPort,
		MetadataDir:   metaDir,
		OutputDir:     outDir,
	}, nil
}

// defaultMetadataDir resolves metadata/ relative to the binary or cwd.
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
