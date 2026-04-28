package apiscanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// StaticScanner loads API entries from a JSON file (metadata/apis.json).
// Used as the fallback when no project path or Swagger URL is configured.
type StaticScanner struct {
	FilePath string
}

func NewStaticScanner(metadataDir string) *StaticScanner {
	return &StaticScanner{
		FilePath: filepath.Join(metadataDir, "apis.json"),
	}
}

func (s *StaticScanner) Name() string { return "Static" }

func (s *StaticScanner) Scan() ([]APIEntry, error) {
	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("static scanner: cannot read %s: %w", s.FilePath, err)
	}
	var entries []APIEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("static scanner: invalid JSON in %s: %w", s.FilePath, err)
	}
	return entries, nil
}
