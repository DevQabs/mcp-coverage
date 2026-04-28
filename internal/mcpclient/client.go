// Package mcpclient implements a minimal MCP stdio client that connects to a
// server subprocess, initializes the MCP session, and fetches the tool list.
package mcpclient

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mcp-coverage/internal/mcpconfig"
)

// ToolEntry mirrors an MCP tool definition.
type ToolEntry struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Client connects to an MCP server via stdio and can list its tools.
type Client struct {
	serverName string
	cfg        *mcpconfig.ServerConfig
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	scanner    *bufio.Scanner
	mu         sync.Mutex
	nextID     int
}

// New creates a Client for the named MCP server (config already resolved).
func New(serverName string, cfg *mcpconfig.ServerConfig) *Client {
	return &Client{serverName: serverName, cfg: cfg}
}

// ListTools starts the server, initializes the MCP session, lists tools, stops.
func (c *Client) ListTools() ([]ToolEntry, error) {
	if err := c.start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", c.serverName, err)
	}
	defer c.stop()

	if err := c.initialize(); err != nil {
		return nil, fmt.Errorf("MCP initialize: %w", err)
	}

	tools, err := c.toolsList()
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}
	return tools, nil
}

// ── lifecycle ──────────────────────────────────────────────────────────────

func (c *Client) start() error {
	command := c.cfg.Command
	if command == "" {
		return fmt.Errorf("server %q has no command configured", c.serverName)
	}

	// Resolve relative path to absolute.
	if !filepath.IsAbs(command) {
		if abs, err := exec.LookPath(command); err == nil {
			command = abs
		}
	}

	c.cmd = exec.Command(command, c.cfg.Args...)

	// Merge parent env + server-specific env.
	c.cmd.Env = os.Environ()
	for k, v := range c.cfg.Env {
		c.cmd.Env = append(c.cmd.Env, k+"="+v)
	}

	// Redirect stderr to our stderr so server logs are visible with -v.
	c.cmd.Stderr = os.Stderr

	stdinPipe, err := c.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdoutPipe, err := c.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("cannot start %q: %w", command, err)
	}

	c.stdin = stdinPipe
	c.scanner = bufio.NewScanner(stdoutPipe)
	c.scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	return nil
}

func (c *Client) stop() {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		// Give the server 2 s to exit gracefully.
		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			c.cmd.Process.Kill()
		}
	}
}

// ── JSON-RPC ───────────────────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *Client) send(method string, params interface{}) (int, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		return 0, err
	}
	data = append(data, '\n')
	_, err = c.stdin.Write(data)
	return id, err
}

func (c *Client) notify(method string, params interface{}) error {
	req := rpcRequest{JSONRPC: "2.0", Method: method, Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

func (c *Client) readResponse(expectedID int) (json.RawMessage, error) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !c.scanner.Scan() {
			if err := c.scanner.Err(); err != nil {
				return nil, err
			}
			return nil, io.EOF
		}
		line := strings.TrimSpace(c.scanner.Text())
		if line == "" {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			// Skip non-JSON lines (e.g. server boot messages).
			continue
		}
		if resp.ID != expectedID {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
	return nil, fmt.Errorf("timeout waiting for response id=%d", expectedID)
}

// ── MCP protocol ───────────────────────────────────────────────────────────

func (c *Client) initialize() error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "mcp-coverage",
			"version": "1.0.0",
		},
	}
	id, err := c.send("initialize", params)
	if err != nil {
		return err
	}
	if _, err := c.readResponse(id); err != nil {
		return fmt.Errorf("initialize response: %w", err)
	}
	// Send initialized notification (no response expected).
	return c.notify("notifications/initialized", map[string]interface{}{})
}

func (c *Client) toolsList() ([]ToolEntry, error) {
	id, err := c.send("tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	raw, err := c.readResponse(id)
	if err != nil {
		return nil, err
	}

	var result struct {
		Tools []ToolEntry `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse tools/list response: %w", err)
	}
	return result.Tools, nil
}
