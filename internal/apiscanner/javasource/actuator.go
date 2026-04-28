package javasource

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"mcp-coverage/internal/apiscanner"
)

// ScanActuator fetches /actuator/mappings from a running Spring application and
// returns the discovered API entries. baseURL is the app's base URL, e.g.
// "http://localhost:8080". Returns an error if the endpoint is unreachable.
func ScanActuator(baseURL string) ([]apiscanner.APIEntry, error) {
	url := strings.TrimRight(baseURL, "/") + "/actuator/mappings"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("actuator: GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("actuator: HTTP %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("actuator: read body: %w", err)
	}
	return parseActuatorMappings(body)
}

// ── response types ─────────────────────────────────────────────────────────

// actuatorResp models the Spring Boot /actuator/mappings JSON response.
// Compatible with Spring Boot 2.x and 3.x.
type actuatorResp struct {
	Contexts map[string]actuatorContext `json:"contexts"`
}

type actuatorContext struct {
	Mappings struct {
		DispatcherServlets map[string][]actuatorHandler `json:"dispatcherServlets"`
	} `json:"mappings"`
}

type actuatorHandler struct {
	Handler   string `json:"handler"`
	Predicate string `json:"predicate"`
	Details   *struct {
		HandlerMethod *struct {
			ClassName string `json:"className"`
			Name      string `json:"name"`
		} `json:"handlerMethod"`
		RequestMappingConditions *struct {
			Methods  []string `json:"methods"`
			Patterns []string `json:"patterns"`
		} `json:"requestMappingConditions"`
	} `json:"details"`
}

// predicateRe parses "{GET /api/path, ...}" strings from the predicate field.
var predicateRe = regexp.MustCompile(`\{(\w+(?:,\s*\w+)*)\s+(/[^\s,}]*)`)

func parseActuatorMappings(data []byte) ([]apiscanner.APIEntry, error) {
	var ar actuatorResp
	if err := json.Unmarshal(data, &ar); err != nil {
		return nil, fmt.Errorf("actuator: parse response: %w", err)
	}
	var entries []apiscanner.APIEntry
	for _, ctx := range ar.Contexts {
		for _, handlers := range ctx.Mappings.DispatcherServlets {
			for _, h := range handlers {
				entries = append(entries, actuatorHandlerEntries(h)...)
			}
		}
	}
	return entries, nil
}

func actuatorHandlerEntries(h actuatorHandler) []apiscanner.APIEntry {
	if h.Details != nil && h.Details.RequestMappingConditions != nil {
		cond := h.Details.RequestMappingConditions
		methods := cond.Methods
		if len(methods) == 0 {
			methods = []string{"GET"}
		}
		var entries []apiscanner.APIEntry
		for _, method := range methods {
			for _, pattern := range cond.Patterns {
				e := apiscanner.APIEntry{
					HTTPMethod: strings.ToUpper(method),
					APIPath:    pattern,
				}
				if h.Details.HandlerMethod != nil {
					e.MethodName = h.Details.HandlerMethod.Name
					cn := h.Details.HandlerMethod.ClassName
					if idx := strings.LastIndex(cn, "."); idx >= 0 {
						e.Controller = cn[idx+1:]
					} else {
						e.Controller = cn
					}
				}
				entries = append(entries, e)
			}
		}
		return entries
	}

	// Fallback: parse predicate string, e.g. "{GET /api/patients, produces [...]}"
	if h.Predicate == "" {
		return nil
	}
	m := predicateRe.FindStringSubmatch(h.Predicate)
	if len(m) < 3 {
		return nil
	}
	var entries []apiscanner.APIEntry
	for _, method := range strings.Split(m[1], ",") {
		method = strings.ToUpper(strings.TrimSpace(method))
		switch method {
		case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
			entries = append(entries, apiscanner.APIEntry{
				HTTPMethod: method,
				APIPath:    m[2],
			})
		}
	}
	return entries
}
