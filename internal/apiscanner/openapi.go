package apiscanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// OpenAPIScanner fetches the Swagger/OpenAPI spec from a running Spring server
// and extracts API entries. Tries v3 first, falls back to v2.
type OpenAPIScanner struct {
	BaseURL string // e.g. http://localhost:8080
	client  *http.Client
}

func NewOpenAPIScanner(baseURL string) *OpenAPIScanner {
	return &OpenAPIScanner{
		BaseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *OpenAPIScanner) Name() string { return "OpenAPI" }

func (s *OpenAPIScanner) Scan() ([]APIEntry, error) {
	// Try OpenAPI v3 first.
	entries, err := s.scanV3()
	if err == nil && len(entries) > 0 {
		return entries, nil
	}
	// Fall back to Swagger v2.
	entries, err = s.scanV2()
	if err != nil {
		return nil, fmt.Errorf("OpenAPI scan failed (tried v3 and v2): %w", err)
	}
	return entries, nil
}

// ── OpenAPI v3 ─────────────────────────────────────────────────────────────

type openAPIV3 struct {
	Paths map[string]map[string]struct {
		Tags        []string               `json:"tags"`
		Summary     string                 `json:"summary"`
		OperationID string                 `json:"operationId"`
		Extensions  map[string]interface{} `json:"-"`
	} `json:"paths"`
}

func (s *OpenAPIScanner) scanV3() ([]APIEntry, error) {
	candidates := []string{
		s.BaseURL + "/v3/api-docs",
		s.BaseURL + "/api-docs",
		s.BaseURL + "/openapi.json",
	}
	var spec openAPIV3
	var found bool
	for _, url := range candidates {
		if err := s.fetch(url, &spec); err == nil {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("no OpenAPI v3 spec found")
	}
	return s.parseV3Paths(spec), nil
}

func (s *OpenAPIScanner) parseV3Paths(spec openAPIV3) []APIEntry {
	var entries []APIEntry
	paths := sortedKeys(spec.Paths)
	for _, path := range paths {
		methods := spec.Paths[path]
		httpMethods := sortedKeys(methods)
		for _, method := range httpMethods {
			if method == "parameters" {
				continue
			}
			op := methods[method]
			controller, methodName := parseOperationID(op.OperationID, path)
			if len(op.Tags) > 0 {
				controller = op.Tags[0]
			}
			entries = append(entries, APIEntry{
				Module:     moduleFromPath(path),
				Controller: controller,
				HTTPMethod: strings.ToUpper(method),
				APIPath:    path,
				MethodName: methodName,
				Summary:    op.Summary,
			})
		}
	}
	return entries
}

// ── Swagger v2 ─────────────────────────────────────────────────────────────

type swaggerV2 struct {
	Paths map[string]map[string]struct {
		Tags        []string `json:"tags"`
		Summary     string   `json:"summary"`
		OperationID string   `json:"operationId"`
	} `json:"paths"`
}

func (s *OpenAPIScanner) scanV2() ([]APIEntry, error) {
	candidates := []string{
		s.BaseURL + "/v2/api-docs",
		s.BaseURL + "/swagger.json",
	}
	var spec swaggerV2
	var found bool
	for _, url := range candidates {
		if err := s.fetch(url, &spec); err == nil {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("no Swagger v2 spec found")
	}
	return s.parseV2Paths(spec), nil
}

func (s *OpenAPIScanner) parseV2Paths(spec swaggerV2) []APIEntry {
	var entries []APIEntry
	paths := sortedKeys(spec.Paths)
	for _, path := range paths {
		methods := spec.Paths[path]
		httpMethods := sortedKeys(methods)
		for _, method := range httpMethods {
			if method == "parameters" {
				continue
			}
			op := methods[method]
			controller, methodName := parseOperationID(op.OperationID, path)
			if len(op.Tags) > 0 {
				controller = op.Tags[0]
			}
			entries = append(entries, APIEntry{
				Module:     moduleFromPath(path),
				Controller: controller,
				HTTPMethod: strings.ToUpper(method),
				APIPath:    path,
				MethodName: methodName,
				Summary:    op.Summary,
			})
		}
	}
	return entries
}

// ── helpers ────────────────────────────────────────────────────────────────

func (s *OpenAPIScanner) fetch(url string, out interface{}) error {
	resp, err := s.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// moduleFromPath derives a module name from the first path segment.
// /hoo010100p01/insertPatient → hoo010100p01
func moduleFromPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "unknown"
}

// parseOperationID splits "ControllerName_methodName" or "methodName" into parts.
func parseOperationID(opID, path string) (controller, method string) {
	if opID == "" {
		// Derive from path: last two segments
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) >= 2 {
			return parts[len(parts)-2], parts[len(parts)-1]
		}
		return "unknown", path
	}
	// Spring Rest Docs / SpringDoc often use "controllerName_methodName"
	if idx := strings.LastIndex(opID, "_"); idx > 0 {
		return opID[:idx], opID[idx+1:]
	}
	return moduleFromPath(path), opID
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
