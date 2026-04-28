package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"mcp-coverage/api"
	"mcp-coverage/internal/apiscanner"
	"mcp-coverage/internal/config"
	"mcp-coverage/internal/coverage"
	"mcp-coverage/internal/mapping"
	"mcp-coverage/internal/mcpclient"
	"mcp-coverage/internal/mcpconfig"
	"mcp-coverage/internal/report"
)

func main() {
	verbose := flag.Bool("v", false, "verbose: show server stderr during tool listing")
	listServers := flag.Bool("list-servers", false, "list all discovered MCP servers and exit")
	flag.Parse()

	if *listServers {
		printServerList()
		return
	}

	cfg, err := config.Load()
	die(err)

	if *verbose {
		fmt.Fprintf(os.Stderr, "[mcp-coverage] target MCP: %s\n", cfg.TargetMCPName)
		fmt.Fprintf(os.Stderr, "[mcp-coverage] metadata dir: %s\n", cfg.MetadataDir)
	}

	// ── Step 1: resolve MCP server config ──────────────────────────────────
	serverCfg, foundIn, err := mcpconfig.Resolve(cfg.TargetMCPName)
	die(err)
	if *verbose {
		fmt.Fprintf(os.Stderr, "[mcp-coverage] resolved %q from %s\n", cfg.TargetMCPName, foundIn)
	}

	// ── Step 2: collect MCP tools ───────────────────────────────────────────
	fmt.Fprintf(os.Stderr, "Connecting to MCP server %q...\n", cfg.TargetMCPName)
	mcpClient := mcpclient.New(cfg.TargetMCPName, serverCfg)
	tools, err := mcpClient.ListTools()
	die(err)
	fmt.Fprintf(os.Stderr, "  Found %d MCP tools\n", len(tools))

	// ── Step 3: collect backend APIs ───────────────────────────────────────
	scanner := apiscanner.NewScanner(cfg.SwaggerURL, cfg.MetadataDir)
	fmt.Fprintf(os.Stderr, "Scanning APIs via %s scanner...\n", scanner.Name())
	apis, err := scanner.Scan()
	die(err)
	fmt.Fprintf(os.Stderr, "  Found %d backend APIs\n", len(apis))

	// ── Step 4: map APIs → MCP tools ───────────────────────────────────────
	engine, err := mapping.NewEngine(cfg.MetadataDir, tools)
	die(err)
	results := engine.Map(apis)

	// ── Step 5: calculate coverage ─────────────────────────────────────────
	metrics, byModule, byController := coverage.Calculate(results)

	// ── Step 6: apply filter ───────────────────────────────────────────────
	filtered := report.Filter(results, cfg.Filter)

	// ── Step 7: output ─────────────────────────────────────────────────────
	fullReport := report.BuildReport(
		cfg.TargetMCPName, scanner.Name(),
		results, tools, metrics, byModule, byController,
	)

	format := strings.ToUpper(cfg.ReportFormat)

	if format == "TABLE" || format == "BOTH" {
		report.PrintTable(os.Stdout, filtered, metrics, byModule, cfg.TargetMCPName)
	}

	if format == "JSON" || format == "BOTH" {
		path, err := report.WriteJSON(fullReport, cfg.OutputDir)
		die(err)
		fmt.Fprintf(os.Stderr, "JSON report written to %s\n", path)
	}

	// ── Step 8: optional admin API ─────────────────────────────────────────
	if cfg.AdminHTTP {
		srv := api.New(cfg.AdminPort, results, metrics, byModule, byController, fullReport)
		die(srv.Run())
	}
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func printServerList() {
	all := mcpconfig.ListAll()
	if len(all) == 0 {
		fmt.Println("No MCP servers found in any config.")
		return
	}
	fmt.Println("Discovered MCP servers:")
	for name, paths := range all {
		fmt.Printf("  %s\n", name)
		for _, p := range paths {
			fmt.Printf("    └─ %s\n", p)
		}
	}
}
