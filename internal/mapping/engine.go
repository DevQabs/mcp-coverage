package mapping

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mcp-coverage/internal/apiscanner"
	"mcp-coverage/internal/mcpclient"
)

const (
	// similarityThresholdMapped sets minimum score for "mapped" via name similarity.
	similarityThresholdMapped = 0.5
	// similarityThresholdReview sets minimum score for "review_required".
	similarityThresholdReview = 0.25
)

// Engine performs the API→MCP tool mapping.
type Engine struct {
	metadataDir string
	// toolMeta maps toolName → explicit metadata (priority 1)
	toolMeta map[string]ToolMetadata
	// pathMethodIndex maps "METHOD /path" → toolName (priority 2)
	pathMethodIndex map[string]string
	// toolNames for similarity matching (priority 3)
	toolNames []string
}

// NewEngine loads tool metadata and builds lookup indexes.
func NewEngine(metadataDir string, tools []mcpclient.ToolEntry) (*Engine, error) {
	e := &Engine{metadataDir: metadataDir}
	if err := e.loadMetadata(); err != nil {
		return nil, err
	}
	e.buildIndexes(tools)
	return e, nil
}

// Map maps all API entries and returns MappingResults.
func (e *Engine) Map(apis []apiscanner.APIEntry) []MappingResult {
	results := make([]MappingResult, 0, len(apis))
	for _, api := range apis {
		results = append(results, e.mapOne(api))
	}
	return results
}

// ── internals ──────────────────────────────────────────────────────────────

func (e *Engine) loadMetadata() error {
	path := filepath.Join(e.metadataDir, "tools_metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			e.toolMeta = make(map[string]ToolMetadata)
			return nil
		}
		return fmt.Errorf("load tools_metadata.json: %w", err)
	}
	if err := json.Unmarshal(data, &e.toolMeta); err != nil {
		return fmt.Errorf("parse tools_metadata.json: %w", err)
	}
	return nil
}

func (e *Engine) buildIndexes(tools []mcpclient.ToolEntry) {
	e.pathMethodIndex = make(map[string]string)
	e.toolNames = make([]string, 0, len(tools))

	for _, t := range tools {
		e.toolNames = append(e.toolNames, t.Name)

		meta, ok := e.toolMeta[t.Name]
		if !ok {
			continue
		}
		// Index single-API mapping.
		if meta.APIPath != "" && meta.HTTPMethod != "" {
			key := indexKey(meta.HTTPMethod, meta.APIPath)
			e.pathMethodIndex[key] = t.Name
		}
		// Index multi-API mappings.
		for _, ref := range meta.APIs {
			if ref.APIPath != "" && ref.HTTPMethod != "" {
				key := indexKey(ref.HTTPMethod, ref.APIPath)
				e.pathMethodIndex[key] = t.Name
			}
		}
	}
}

func (e *Engine) mapOne(api apiscanner.APIEntry) MappingResult {
	key := indexKey(api.HTTPMethod, api.APIPath)

	// Priority 1: explicit metadata (path+method index built from metadata).
	if toolName, ok := e.pathMethodIndex[key]; ok {
		return MappingResult{
			APIEntry:      api,
			MCPToolName:   toolName,
			MappingStatus: StatusMapped,
			Remark:        "explicit metadata",
		}
	}

	// Priority 2: apiPath + httpMethod fuzzy (path match without query string).
	if toolName := e.pathMethodFuzzy(api.HTTPMethod, api.APIPath); toolName != "" {
		return MappingResult{
			APIEntry:      api,
			MCPToolName:   toolName,
			MappingStatus: StatusMapped,
			Remark:        "path+method match",
		}
	}

	// Priority 3: controller/method name similarity.
	if len(e.toolNames) > 0 {
		candidate, score := bestToolMatch(api.Controller, api.MethodName, e.toolNames)
		if score >= similarityThresholdMapped {
			return MappingResult{
				APIEntry:      api,
				MCPToolName:   candidate,
				MappingStatus: StatusMapped,
				Remark:        fmt.Sprintf("name similarity %.2f", score),
			}
		}
		if score >= similarityThresholdReview {
			return MappingResult{
				APIEntry:      api,
				MCPToolName:   candidate,
				MappingStatus: StatusReviewRequired,
				Remark:        fmt.Sprintf("name similarity %.2f — needs review", score),
			}
		}
	}

	return MappingResult{
		APIEntry:      api,
		MCPToolName:   "",
		MappingStatus: StatusUnmapped,
		Remark:        "no MCP tool found",
	}
}

// pathMethodFuzzy strips query strings and normalizes before comparison.
func (e *Engine) pathMethodFuzzy(method, path string) string {
	// Strip query string from path.
	cleanPath := stripQuery(path)
	cleanMethod := strings.ToUpper(method)

	for indexedKey, toolName := range e.pathMethodIndex {
		parts := strings.SplitN(indexedKey, " ", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == cleanMethod && stripQuery(parts[1]) == cleanPath {
			return toolName
		}
	}
	return ""
}

func indexKey(method, path string) string {
	return strings.ToUpper(method) + " " + stripQuery(path)
}

func stripQuery(path string) string {
	if idx := strings.Index(path, "?"); idx >= 0 {
		return path[:idx]
	}
	return path
}
