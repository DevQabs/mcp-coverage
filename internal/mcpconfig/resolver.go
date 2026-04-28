// Package mcpconfig resolves MCP server connection info from Claude config files.
// Search order:
//  1. ~/.claude/settings.json        (global Claude Code settings)
//  2. ~/Library/Application Support/Claude/claude_desktop_config.json  (Claude Desktop)
//  3. ~/.claude.json projects[cwd].mcpServers  (per-project)
//  4. ./.claude/settings.json        (local project settings)
package mcpconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ServerConfig mirrors the Claude MCP server definition.
type ServerConfig struct {
	Type    string            `json:"type"`    // stdio | sse (default: stdio)
	Command string            `json:"command"` // for stdio
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"` // for sse
}

// Resolve finds the ServerConfig for the given MCP server name by searching
// all known Claude config locations.
func Resolve(name string) (*ServerConfig, string, error) {
	locations := configLocations()

	for _, loc := range locations {
		servers, err := readMCPServers(loc.path, loc.reader)
		if err != nil {
			continue
		}
		if cfg, ok := servers[name]; ok {
			return &cfg, loc.path, nil
		}
	}

	return nil, "", fmt.Errorf(
		"MCP server %q not found in any config. Searched:\n%s",
		name, searchedPaths(locations),
	)
}

// ListAll returns all MCP server names found across all config locations.
func ListAll() map[string][]string {
	result := make(map[string][]string)
	for _, loc := range configLocations() {
		servers, err := readMCPServers(loc.path, loc.reader)
		if err != nil {
			continue
		}
		for name := range servers {
			result[name] = append(result[name], loc.path)
		}
	}
	return result
}

// ── internals ──────────────────────────────────────────────────────────────

type configLoc struct {
	path   string
	reader func(path string) (map[string]ServerConfig, error)
}

func configLocations() []configLoc {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	locs := []configLoc{
		// 1. Global Claude Code settings
		{
			path:   filepath.Join(home, ".claude", "settings.json"),
			reader: readClaudeSettings,
		},
		// 2. Claude Desktop
		{
			path:   claudeDesktopConfig(),
			reader: readClaudeDesktopConfig,
		},
		// 3. Per-project from ~/.claude.json
		{
			path:   filepath.Join(home, ".claude.json"),
			reader: func(p string) (map[string]ServerConfig, error) { return readClaudeJSON(p, cwd) },
		},
		// 4. Local .claude/settings.json
		{
			path:   filepath.Join(cwd, ".claude", "settings.json"),
			reader: readClaudeSettings,
		},
	}
	return locs
}

func claudeDesktopConfig() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	}
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		return filepath.Join(appData, "Claude", "claude_desktop_config.json")
	}
	return filepath.Join(home, ".config", "claude", "claude_desktop_config.json")
}

// readClaudeSettings reads {"mcpServers":{...}} from a settings.json file.
func readClaudeSettings(path string) (map[string]ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		MCPServers map[string]ServerConfig `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc.MCPServers, nil
}

// readClaudeDesktopConfig reads {"mcpServers":{...}} — same shape as settings.
func readClaudeDesktopConfig(path string) (map[string]ServerConfig, error) {
	return readClaudeSettings(path)
}

// readClaudeJSON reads per-project mcpServers from ~/.claude.json.
// It looks up projects[cwd].mcpServers.
func readClaudeJSON(path, cwd string) (map[string]ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Projects map[string]struct {
			MCPServers map[string]ServerConfig `json:"mcpServers"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	proj, ok := doc.Projects[cwd]
	if !ok {
		return nil, nil
	}
	return proj.MCPServers, nil
}

func readMCPServers(path string, reader func(string) (map[string]ServerConfig, error)) (map[string]ServerConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	return reader(path)
}

func searchedPaths(locs []configLoc) string {
	out := ""
	for _, l := range locs {
		out += "  - " + l.path + "\n"
	}
	return out
}
