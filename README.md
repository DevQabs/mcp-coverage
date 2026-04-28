# mcp-coverage

**MCP API Coverage Tracker** — Measure, audit, and govern the coverage of Spring Controller APIs by MCP Tools in enterprise AI-integrated systems.

---

## Table of Contents

- [Background](#background)
- [Problem Statement](#problem-statement)
- [What This Tool Does](#what-this-tool-does)
- [Design Principles](#design-principles)
- [Non-Goals](#non-goals)
- [Architecture Overview](#architecture-overview)
- [Project Structure](#project-structure)
- [How It Works](#how-it-works)
  - [Step 1: Config Resolution](#step-1-config-resolution)
  - [Step 2: MCP Tool Collection](#step-2-mcp-tool-collection)
  - [Step 3: API Collection](#step-3-api-collection)
  - [Step 4: Mapping Engine](#step-4-mapping-engine)
  - [Step 5: Coverage Calculation](#step-5-coverage-calculation)
  - [Step 6: Report Generation](#step-6-report-generation)
- [Mapping Strategy](#mapping-strategy)
  - [Priority 1 — Explicit Metadata](#priority-1--explicit-metadata)
  - [Priority 2 — Path and Method Match](#priority-2--path-and-method-match)
  - [Priority 3 — Name Similarity](#priority-3--name-similarity)
  - [Why Explicit Metadata Is the Source of Truth](#why-explicit-metadata-is-the-source-of-truth)
- [MCP Tool Metadata Design](#mcp-tool-metadata-design)
  - [Single-API Mapping](#single-api-mapping)
  - [Multi-API Mapping](#multi-api-mapping)
  - [Metadata Governance Rules](#metadata-governance-rules)
- [Target MCP Resolution](#target-mcp-resolution)
  - [Resolution Order](#resolution-order)
  - [Example Resolution Flow](#example-resolution-flow)
- [API Registry](#api-registry)
  - [Static Registry](#static-registry)
  - [OpenAPI Scanner](#openapi-scanner)
- [Installation](#installation)
- [Configuration](#configuration)
- [Running the Tool](#running-the-tool)
  - [Basic Execution](#basic-execution)
  - [Common Scenarios](#common-scenarios)
- [Output](#output)
  - [Terminal Table Report](#terminal-table-report)
  - [JSON Report](#json-report)
- [HTTP Admin API](#http-admin-api)
- [CI/CD Integration](#cicd-integration)
- [Audience and Use Cases](#audience-and-use-cases)
- [Governance Strategy](#governance-strategy)
- [Best Practices](#best-practices)
- [Security Considerations](#security-considerations)
- [Performance Considerations](#performance-considerations)
- [Limitations](#limitations)
- [Troubleshooting](#troubleshooting)
- [Roadmap](#roadmap)

---

## Background

Modern enterprise systems increasingly integrate AI-powered agents through the **Model Context Protocol (MCP)**. In these architectures, AI agents interact with backend systems not through direct API calls, but through **MCP Tools** — structured, schema-validated interfaces that abstract the underlying REST APIs.

In a hospital EMR (Electronic Medical Record) system, for example, an AI agent might:

- Register a new patient by invoking the `create_patient` MCP Tool
- Schedule a clinical reception via `create_reception`
- Look up drug prescriptions using `search_prsc`
- Submit an insurance claim through `create_invoice`
- Complete a clinical visit with `complete_clinic`

Each of these MCP Tools internally maps to one or more Spring Controller REST endpoints. The MCP Tool is the public-facing agent interface; the Spring API is the authoritative system of record.

As systems grow — new features are added, APIs are refactored, new controllers appear — the question inevitably arises:

> **How many of our Spring APIs are actually reachable by an AI agent through the MCP layer?**

Without explicit tooling, this question cannot be answered reliably. The answer requires knowing both what APIs exist and what the MCP Tools actually cover.

`mcp-coverage` was built to answer this question systematically.

---

## Problem Statement

Enterprise teams adopting MCP-integrated AI systems face several compounding governance problems:

**1. Coverage Blindness**
There is no automated way to know which Spring Controller endpoints are accessible via MCP Tools and which are not. Teams rely on tribal knowledge or manual documentation, both of which decay rapidly.

**2. Silent Regressions**
When new APIs are added to the Spring backend, they are invisible to the AI agent unless an MCP Tool is explicitly created for them. Without tracking, these gaps accumulate silently.

**3. No Mapping Audit Trail**
Which MCP Tool covers which API? Is the mapping intentional or inferred? There is no structured record of the relationship between the AI interface layer and the backend.

**4. Engineering Manager Visibility**
Engineering managers and technical leads have no dashboard-level visibility into AI capability coverage. They cannot answer "What percentage of our backend is AI-accessible?" without manually tracing code.

**5. Developer Onboarding Friction**
New MCP developers or backend engineers joining a project have no reference showing which APIs are covered, which need MCP tools, and which are intentionally excluded.

**6. Compliance and Governance Risk**
In regulated domains like healthcare, financial systems, or government platforms, the ability to demonstrate that AI agent access is bounded, documented, and auditable is increasingly required.

`mcp-coverage` addresses all six problems with a single, automated, configurable tool.

---

## What This Tool Does

`mcp-coverage` performs the following pipeline at runtime:

1. **Resolves** the target MCP server connection from existing Claude configuration files using only `TARGET_MCP_NAME` — no separate transport or URL configuration required.
2. **Connects** to the MCP server via stdio JSON-RPC and retrieves the full list of registered Tools.
3. **Collects** all backend Spring Controller API definitions — either from a live Swagger/OpenAPI endpoint or from a curated static registry.
4. **Maps** each API to an MCP Tool using a three-priority matching engine: explicit metadata first, path/method match second, name similarity last.
5. **Calculates** coverage metrics at the total, module, and controller level.
6. **Generates** a color-coded terminal table and a structured JSON report.
7. **Optionally exposes** an HTTP admin API for integration with dashboards, CI checks, or internal tooling.

The tool is **read-only with respect to all existing business logic**. It does not modify MCP Tool handlers, Spring Controllers, or any application code. The only files it reads and writes are its own metadata registry and report output.

---

## Design Principles

**Explicit over inferred.**
Explicit metadata in `tools_metadata.json` is always the authoritative mapping source. Automated matching via path or name similarity is a safety net, not the primary strategy. Teams are expected to maintain explicit mappings as part of their MCP development workflow.

**Non-invasive.**
The tool does not modify any existing MCP Tool handler code, Spring Controller code, or business logic. All mapping information lives in separate metadata files managed by this tool.

**Fail open on unmapped.**
Any API that has no mapping — explicit, path-matched, or similarity-matched — automatically appears as `unmapped` in the report. There is no silent omission. New APIs without MCP Tools surface immediately.

**Single configuration input.**
The operator specifies only `TARGET_MCP_NAME`. All other connection details (command, arguments, environment variables) are resolved automatically from existing Claude configuration files. This prevents configuration duplication and drift.

**Composable output.**
The tool produces both a human-readable terminal report and a machine-readable JSON report. The JSON report is designed for downstream consumption by CI pipelines, dashboards, and alerting systems.

**Deterministic and idempotent.**
Given the same `apis.json`, `tools_metadata.json`, and MCP Tool list, the tool always produces the same mapping results. There is no runtime state that affects mapping decisions.

---

## Non-Goals

- **This tool does not test MCP Tool correctness.** It verifies coverage, not behavior. Whether a mapped MCP Tool correctly invokes the backend API it claims to cover is a separate concern addressed by integration tests.
- **This tool does not enforce MCP Tool creation.** It reports gaps but does not block deployments. Enforcement is a CI/CD policy decision made by the team.
- **This tool does not manage MCP Tool business logic.** Tool handlers, input validation, and response formatting remain entirely under the ownership of MCP developers.
- **This tool does not replace API documentation.** Coverage tracking is not a substitute for OpenAPI specs, developer wikis, or architectural decision records.
- **This tool does not discover APIs dynamically at runtime.** APIs must be declared in `apis.json` or accessible via Swagger. Runtime route scanning is not performed.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        mcp-coverage CLI                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────────┐    ┌─────────────────┐    ┌───────────────┐  │
│   │ Config Loader│    │  MCP Config     │    │  API Scanner  │  │
│   │ (env vars)   │───▶│  Resolver       │    │  (OpenAPI /   │  │
│   └──────────────┘    │  (~/.claude/    │    │   Static JSON)│  │
│                       │  settings.json  │    └───────┬───────┘  │
│                       │  claude_desktop │            │          │
│                       │  ~/.claude.json)│    ┌───────▼───────┐  │
│                       └────────┬────────┘    │  API Registry │  │
│                                │             │  []APIEntry   │  │
│                       ┌────────▼────────┐    └───────┬───────┘  │
│                       │   MCP Client    │            │          │
│                       │ (stdio JSON-RPC)│    ┌───────▼───────┐  │
│                       └────────┬────────┘    │ Mapping Engine│  │
│                                │             │  P1: metadata │  │
│                       ┌────────▼────────┐    │  P2: path     │  │
│                       │  MCP Tool List  │────▶  P3: similarity│  │
│                       │  []ToolEntry    │    └───────┬───────┘  │
│                       └─────────────────┘            │          │
│                                              ┌───────▼───────┐  │
│                                              │   Calculator  │  │
│                                              │ (metrics by   │  │
│                                              │  module/ctrl) │  │
│                                              └───────┬───────┘  │
│                                                      │          │
│                              ┌───────────────────────┤          │
│                              │                       │          │
│                     ┌────────▼───────┐    ┌──────────▼───────┐  │
│                     │  Table Report  │    │   JSON Report    │  │
│                     │  (stdout)      │    │ reports/*.json   │  │
│                     └────────────────┘    └──────────────────┘  │
│                                                      │          │
│                                          ┌───────────▼───────┐  │
│                                          │  HTTP Admin API   │  │
│                                          │  (optional)       │  │
│                                          └───────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Data flow summary:**

1. Environment is loaded → target MCP name is resolved to a server config.
2. MCP client spawns the server process and collects tools via `tools/list`.
3. API scanner collects backend endpoints from Swagger or `metadata/apis.json`.
4. Mapping engine applies three-priority matching to pair each API with a tool.
5. Calculator aggregates mapped/review_required/unmapped counts per module and controller.
6. Reports are written to stdout (table) and disk (JSON).
7. Optionally, an HTTP server exposes query endpoints for the collected data.

---

## Project Structure

```
mcp-coverage/
├── cmd/
│   └── main.go                      # CLI entry point; orchestrates all pipeline steps
│
├── internal/
│   ├── config/
│   │   └── config.go                # Loads and validates all environment variables
│   │
│   ├── mcpconfig/
│   │   └── resolver.go              # Searches Claude config files for MCP server definitions
│   │
│   ├── apiscanner/
│   │   ├── types.go                 # APIEntry struct and Scanner interface
│   │   ├── openapi.go               # OpenAPI v3 / Swagger v2 HTTP fetcher and parser
│   │   └── static.go                # Loads APIEntry list from metadata/apis.json
│   │
│   ├── mcpclient/
│   │   └── client.go                # Stdio subprocess MCP client; implements JSON-RPC init + tools/list
│   │
│   ├── mapping/
│   │   ├── types.go                 # MappingResult, ToolMetadata, APIRef, status constants
│   │   ├── engine.go                # Three-priority mapping engine with index construction
│   │   └── similarity.go            # Token-based Jaccard similarity for name matching
│   │
│   ├── coverage/
│   │   └── calculator.go            # Computes Metrics, ModuleMetrics, ControllerMetrics
│   │
│   └── report/
│       ├── table.go                 # ANSI-colored terminal table with module summary
│       ├── json_report.go           # Builds and serializes CoverageReport to JSON
│       └── filter.go                # Filters MappingResults by status, module, or controller
│
├── api/
│   └── admin.go                     # Optional HTTP server exposing coverage data via REST
│
├── metadata/
│   ├── apis.json                    # Authoritative static backend API registry
│   └── tools_metadata.json          # Explicit MCP Tool → API mapping declarations
│
├── reports/                         # Generated JSON reports (gitignored or archived)
│   └── coverage_report.json
│
├── go.mod
└── README.md
```

### Package Responsibilities

| Package | Responsibility |
|---------|----------------|
| `cmd` | Wires all packages together, handles CLI flags, controls output format |
| `internal/config` | Single point of environment variable loading and validation |
| `internal/mcpconfig` | Decouples MCP server discovery from connection logic; supports multiple config sources |
| `internal/apiscanner` | Abstracts API collection behind a `Scanner` interface; swappable implementations |
| `internal/mcpclient` | Implements the MCP stdio protocol without any external dependencies |
| `internal/mapping` | Core business logic; the mapping engine is the most critical component |
| `internal/coverage` | Pure calculation logic; no I/O, easily unit-testable |
| `internal/report` | Presentation layer; separated so CI consumers can use JSON without terminal formatting |
| `api` | Optional operational surface; not part of the core analysis pipeline |
| `metadata` | Human-managed data files; the primary interface for platform teams |

---

## How It Works

### Step 1: Config Resolution

The tool reads `TARGET_MCP_NAME` from the environment and searches all known Claude configuration locations to find the matching server definition. No separate URL, command, or transport configuration is needed.

```
TARGET_MCP_NAME=emr-mcp
```

The resolver searches in this order:

1. `~/.claude/settings.json`
2. `~/Library/Application Support/Claude/claude_desktop_config.json`
3. `~/.claude.json` (per-project entries under the current working directory)
4. `./.claude/settings.json`

Once found, the server's `command`, `args`, and `env` fields are used to spawn the subprocess.

### Step 2: MCP Tool Collection

The MCP client spawns the resolved server as a subprocess and communicates over stdin/stdout using the MCP JSON-RPC protocol:

```
initialize  →  notifications/initialized  →  tools/list
```

Each tool's `name`, `description`, and `inputSchema` are collected. The tool list is the live, authoritative source of what the MCP server currently exposes.

### Step 3: API Collection

If `SWAGGER_URL` is set, the OpenAPI scanner fetches the spec from the running Spring server (tries v3 first, then v2) and extracts all path/method/tag combinations into `APIEntry` structs.

If `SWAGGER_URL` is not set, the static scanner reads `metadata/apis.json`. This is the expected mode for most teams: `apis.json` is curated alongside the Spring codebase and committed to the repository.

### Step 4: Mapping Engine

For each API entry, the engine applies three matching strategies in strict priority order. The first strategy that produces a match wins; lower-priority strategies are not evaluated.

### Step 5: Coverage Calculation

The calculator aggregates `MappingResult` rows into:

- Global totals (total / mapped / review_required / unmapped / coverage_rate)
- Per-module breakdowns
- Per-controller breakdowns

Coverage rate formula:
```
coverage_rate = mapped / total × 100
```

Note: `review_required` items are **not** counted as mapped. They require human confirmation before they can be treated as covered.

### Step 6: Report Generation

Depending on `REPORT_FORMAT`:
- `TABLE` — writes a color-coded table to stdout
- `JSON` — writes `coverage_report.json` to `OUTPUT_DIR`
- `BOTH` — does both (default)

---

## Mapping Strategy

The mapping engine uses three strategies applied in strict priority order. This design reflects the reality that automated matching is useful but imperfect, and human declarations must always take precedence.

### Priority 1 — Explicit Metadata

**Source:** `metadata/tools_metadata.json`

The operator explicitly declares which API path(s) and HTTP method(s) each MCP Tool covers. This is the only mapping strategy that is guaranteed to be correct. All other strategies are heuristics.

```json
"create_patient": {
  "apiPath": "/hoo010100p01/insertPatient",
  "httpMethod": "POST",
  "controllerName": "PatientController",
  "methodName": "insertPatient"
}
```

When a match is found via explicit metadata, the result is marked `mapped` with remark `"explicit metadata"`.

Tools that call multiple backend APIs declare an `apis` array:

```json
"complete_payment": {
  "apis": [
    { "apiPath": "/reception/selectOverallCalculationInfo", "httpMethod": "GET" },
    { "apiPath": "/reception/insertPayData", "httpMethod": "POST" }
  ]
}
```

### Priority 2 — Path and Method Match

**Source:** Live MCP Tool list cross-referenced against the path index built from `tools_metadata.json`.

If an API's path and HTTP method exactly match a path registered in the metadata index, it is treated as mapped. This handles cases where the same API appears in `apis.json` under slightly different formatting (e.g., with or without query string).

When a match is found via path/method, the result is marked `mapped` with remark `"path+method match"`.

### Priority 3 — Name Similarity

**Source:** MCP Tool names compared against API controller and method names using token-based Jaccard similarity.

Both strings are tokenized by splitting on camelCase boundaries, underscores, hyphens, slashes, and dots. Common stop words (`controller`, `service`, `handler`, `api`) are removed. The similarity score is the Jaccard coefficient of the resulting token sets.

| Score | Status | Remark |
|-------|--------|--------|
| ≥ 0.50 | `mapped` | `"name similarity 0.67"` |
| 0.25–0.49 | `review_required` | `"name similarity 0.33 — needs review"` |
| < 0.25 | `unmapped` | `"no MCP tool found"` |

**Example:** `ReceptionController.insertReception` vs `create_reception`

Tokens from controller+method: `[reception, insert, reception]` → `{reception, insert}`
Tokens from tool name: `[create, reception]` → `{create, reception}`
Shared: `{reception}` — Jaccard = 1/3 = 0.33 → `review_required`

This illustrates why similarity alone is insufficient: `insert` and `create` are semantically equivalent but tokenize differently, causing the score to fall below the `mapped` threshold. The explicit metadata declaration resolves this definitively.

### Why Explicit Metadata Is the Source of Truth

Automated matching strategies — path matching and name similarity — are valuable safety nets that reduce initial setup friction, but they carry fundamental limitations in enterprise contexts:

**1. Semantic gaps in naming conventions.**
Spring Controller method names follow no universal convention. `insertPatient`, `createPatient`, `registerPatient`, and `addPatient` all refer to the same operation. No string similarity algorithm can reliably equate these without domain knowledge.

**2. Fan-out: one tool, many APIs.**
Many MCP Tools aggregate multiple backend calls. `complete_payment` calls both `selectOverallCalculationInfo` (GET) and `insertPayData` (POST). Path matching can only resolve one-to-one relationships; explicit metadata handles one-to-many natively.

**3. Internal APIs.**
Some backend endpoints are called internally by MCP Tools during data enrichment (e.g., `selectReceptionList` is called by `search_doctor` to look up doctor history sequence numbers). These internal calls are not discoverable through interface-level analysis.

**4. Compliance requirements.**
In regulated domains — healthcare (EMR, insurance), financial services, government — AI system boundary documentation must be authoritative and auditable. An automated similarity score does not satisfy this requirement. A human-declared, version-controlled mapping file does.

**5. Drift prevention.**
Explicit metadata is committed to the repository. When APIs are renamed, paths change, or tools are refactored, the mapping file must be updated — creating a deliberate, visible, reviewable change. Purely automated matching hides these changes.

**The operating principle:** treat name similarity matches as prompts for human review, not as confirmed coverage. Only explicit metadata declares coverage with confidence.

---

## MCP Tool Metadata Design

All mapping declarations live in `metadata/tools_metadata.json`. This file is the primary interface between platform architects, backend developers, and MCP developers.

### Single-API Mapping

Use when the MCP Tool maps directly to exactly one backend endpoint:

```json
{
  "create_patient": {
    "apiPath": "/hoo010100p01/insertPatient",
    "httpMethod": "POST",
    "controllerName": "PatientController",
    "methodName": "insertPatient"
  },
  "update_reception_status": {
    "apiPath": "/diagnose/reception/updateReceptionStatus",
    "httpMethod": "POST",
    "controllerName": "DiagnoseController",
    "methodName": "updateReceptionStatus"
  }
}
```

### Multi-API Mapping

Use when the MCP Tool internally calls multiple backend endpoints. The `note` field documents the purpose of each call:

```json
{
  "search_doctor": {
    "apis": [
      {
        "apiPath": "/oneai/doctors",
        "httpMethod": "GET",
        "controllerName": "OneAIController",
        "methodName": "getDoctors",
        "note": "Primary search by doctor name"
      },
      {
        "apiPath": "/MMD/ReservationV2/getMdcrDeptAndDoctor",
        "httpMethod": "POST",
        "controllerName": "ReservationController",
        "methodName": "getMdcrDeptAndDoctor",
        "note": "Lookup by employee ID (internal)"
      },
      {
        "apiPath": "/reception/selectReceptionList",
        "httpMethod": "GET",
        "controllerName": "ReceptionController",
        "methodName": "selectReceptionList",
        "note": "Resolve history sequence number (internal)"
      }
    ]
  },
  "complete_payment": {
    "apis": [
      {
        "apiPath": "/reception/selectOverallCalculationInfo",
        "httpMethod": "GET",
        "note": "Fetch pre-payment calculation"
      },
      {
        "apiPath": "/reception/insertPayData",
        "httpMethod": "POST",
        "note": "Submit payment record"
      }
    ]
  }
}
```

### Metadata Governance Rules

These rules apply to all entries in `tools_metadata.json`:

1. **Every MCP Tool that calls a backend API must have an entry.** No exceptions for "obvious" or "simple" tools.

2. **`apiPath` must exactly match the path in the Spring Controller's `@RequestMapping` annotation** (or the equivalent) without query string parameters.

3. **`httpMethod` must be uppercase** (`GET`, `POST`, `PUT`, `DELETE`, `PATCH`).

4. **Internal APIs must be declared.** If a tool calls a backend API purely for data enrichment (not as the primary operation), it must still appear in the `apis` array with a `note` explaining its role.

5. **When a tool covers the same API as another tool**, both tools declare the mapping independently. The API will appear as `mapped` and the most recently matched tool name will appear in the report. This is expected and does not represent an error.

6. **Removing a tool from `tools_metadata.json` when the MCP Tool is deleted** is mandatory. Stale metadata entries cause false positives in coverage reports.

7. **Adding a new API to `apis.json` without a corresponding `tools_metadata.json` entry** is the correct workflow for flagging unmapped APIs. The API will appear as `unmapped` until an MCP Tool is created and registered.

---

## Target MCP Resolution

### Resolution Order

`mcp-coverage` searches for the named MCP server in the following configuration files, in order:

| Priority | File | Scope |
|----------|------|-------|
| 1 | `~/.claude/settings.json` | Global — Claude Code CLI settings |
| 2 | `~/Library/Application Support/Claude/claude_desktop_config.json` | Global — Claude Desktop application |
| 3 | `~/.claude.json` (projects section) | Per-project — keyed by working directory path |
| 4 | `./.claude/settings.json` | Local — current project directory |

On Linux, the Claude Desktop path resolves to `~/.config/claude/claude_desktop_config.json`. On Windows, it resolves to `%APPDATA%/Claude/claude_desktop_config.json`.

The first file that contains an entry matching `TARGET_MCP_NAME` wins. The resolved server definition provides `command`, `args`, and `env` for spawning the subprocess.

### Example Resolution Flow

**Environment:**
```bash
TARGET_MCP_NAME=emr-mcp
```

**`~/.claude/settings.json` contains:**
```json
{
  "mcpServers": {
    "emr-mcp": {
      "command": "/Users/ktg2926/Documents/mcp/wehagoH/emr-mcp",
      "env": {
        "EMR_ENV": "local",
        "EMR_CONFIG": "/Users/ktg2926/.emr-mcp/config.json"
      }
    }
  }
}
```

**Resolution result:**
```
Server "emr-mcp" found in /Users/ktg2926/.claude/settings.json
Command: /Users/ktg2926/Documents/mcp/wehagoH/emr-mcp
Env:     EMR_ENV=local, EMR_CONFIG=/Users/ktg2926/.emr-mcp/config.json
```

The tool then spawns:
```
/Users/ktg2926/Documents/mcp/wehagoH/emr-mcp
```
with `EMR_ENV` and `EMR_CONFIG` merged into the parent process environment. No additional transport configuration is required from the operator.

To inspect which servers are discoverable from the current environment:

```bash
./bin/mcp-coverage -list-servers
```

Output:
```
Discovered MCP servers:
  emr-mcp
    └─ /Users/ktg2926/.claude/settings.json
    └─ /Users/ktg2926/Library/Application Support/Claude/claude_desktop_config.json
  kibana
    └─ /Users/ktg2926/Library/Application Support/Claude/claude_desktop_config.json
```

---

## API Registry

### Static Registry

`metadata/apis.json` is the curated list of all known backend Spring Controller APIs. It is the primary API source when no Swagger endpoint is available (local development, offline CI, environments where the Spring server is not running during analysis).

**Structure:**
```json
[
  {
    "module": "hoo010100p01",
    "controller": "PatientController",
    "httpMethod": "POST",
    "apiPath": "/hoo010100p01/insertPatient",
    "methodName": "insertPatient",
    "summary": "Register a new patient record"
  },
  {
    "module": "reception",
    "controller": "ReceptionController",
    "httpMethod": "POST",
    "apiPath": "/reception/insertReception",
    "methodName": "insertReception",
    "summary": "Create a new clinical reception entry"
  },
  {
    "module": "invoice",
    "controller": "InvoiceController",
    "httpMethod": "POST",
    "apiPath": "/invoice",
    "methodName": "createInvoice",
    "summary": "Generate insurance claim invoice (routes to insurance server)"
  }
]
```

**Field definitions:**

| Field | Required | Description |
|-------|----------|-------------|
| `module` | Yes | Top-level path segment; used for grouping in reports |
| `controller` | Yes | Spring Controller class name |
| `httpMethod` | Yes | HTTP verb in uppercase |
| `apiPath` | Yes | Request path without query string |
| `methodName` | Yes | Controller handler method name |
| `summary` | No | Human-readable description |

**Workflow for adding new APIs:**

When a backend developer adds a new Spring Controller endpoint, they add a corresponding entry to `apis.json` in the same pull request. This entry will appear as `unmapped` in coverage reports until a corresponding MCP Tool is created and registered in `tools_metadata.json`.

This creates a visible, trackable gap that the MCP development team must close.

### OpenAPI Scanner

When `SWAGGER_URL` is set, `mcp-coverage` fetches the OpenAPI specification directly from the running Spring server. This is preferred in environments where Swagger is available at runtime.

**Endpoint discovery order (OpenAPI v3):**
1. `{SWAGGER_URL}/v3/api-docs`
2. `{SWAGGER_URL}/api-docs`
3. `{SWAGGER_URL}/openapi.json`

**Fallback (Swagger v2):**
1. `{SWAGGER_URL}/v2/api-docs`
2. `{SWAGGER_URL}/swagger.json`

The scanner derives `module` from the first URL path segment, `controller` from the operation's `tags` array (or `operationId` prefix), and `methodName` from the `operationId` suffix.

**Trade-offs:**

| Mode | Pros | Cons |
|------|------|------|
| Static (`apis.json`) | Works offline; version-controlled; predictable | Requires manual updates when APIs change |
| OpenAPI | Always reflects live API surface | Requires Spring server to be running; OpenAPI quality varies |

For most teams, the recommended workflow is: maintain `apis.json` as the source of truth, and use `SWAGGER_URL` periodically to audit that `apis.json` is complete.

---

## Installation

**Prerequisites:**
- Go 1.25 or later
- An MCP server registered in one of the Claude configuration files
- Access to `metadata/apis.json` and `metadata/tools_metadata.json`

**Clone and build:**
```bash
git clone <repository-url> mcp-coverage
cd mcp-coverage
go build -o bin/mcp-coverage ./cmd/
```

The binary has no runtime dependencies beyond the Go standard library.

**Verify:**
```bash
./bin/mcp-coverage -list-servers
```

---

## Configuration

All configuration is provided through environment variables. There are no configuration files for the tool itself — only for the data it analyzes (`metadata/`).

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `TARGET_MCP_NAME` | — | **Yes** | Name of the MCP server to analyze, as registered in Claude config |
| `SWAGGER_URL` | — | No | Base URL of the Spring server for live OpenAPI scanning |
| `REPORT_FORMAT` | `BOTH` | No | `TABLE`, `JSON`, or `BOTH` |
| `FILTER` | `ALL` | No | `ALL`, `MAPPED`, `UNMAPPED`, `REVIEW_REQUIRED`, `MODULE:<name>`, `CONTROLLER:<name>` |
| `ADMIN_HTTP` | `false` | No | `true` to enable the HTTP admin API |
| `ADMIN_PORT` | `8080` | No | Port for the HTTP admin API |
| `METADATA_DIR` | `./metadata` | No | Directory containing `apis.json` and `tools_metadata.json` |
| `OUTPUT_DIR` | `./reports` | No | Directory where `coverage_report.json` is written |

---

## Running the Tool

### Basic Execution

```bash
# Minimal — table + JSON report, no filter
TARGET_MCP_NAME=emr-mcp ./bin/mcp-coverage

# Verbose (shows MCP server stderr during tool listing)
TARGET_MCP_NAME=emr-mcp ./bin/mcp-coverage -v
```

### Common Scenarios

**Show only unmapped APIs**
```bash
TARGET_MCP_NAME=emr-mcp FILTER=UNMAPPED ./bin/mcp-coverage
```
Use when onboarding new backend APIs and triaging which ones need MCP Tools.

**Show only review-required APIs**
```bash
TARGET_MCP_NAME=emr-mcp FILTER=REVIEW_REQUIRED ./bin/mcp-coverage
```
Use during sprint planning to identify APIs where similarity matching produced uncertain results.

**Filter by module**
```bash
TARGET_MCP_NAME=emr-mcp FILTER=MODULE:reception ./bin/mcp-coverage
```
Use when a specific backend team is responsible for a module and wants to check their own coverage.

**Filter by controller**
```bash
TARGET_MCP_NAME=emr-mcp FILTER=CONTROLLER:PatientController ./bin/mcp-coverage
```

**Use live Swagger instead of static registry**
```bash
TARGET_MCP_NAME=emr-mcp SWAGGER_URL=http://localhost:8080/wehagoh/devclinic ./bin/mcp-coverage
```

**Generate JSON report only (CI mode)**
```bash
TARGET_MCP_NAME=emr-mcp \
  REPORT_FORMAT=JSON \
  OUTPUT_DIR=./reports \
  ./bin/mcp-coverage
```

**Start HTTP admin API for dashboard integration**
```bash
TARGET_MCP_NAME=emr-mcp \
  REPORT_FORMAT=BOTH \
  ADMIN_HTTP=true \
  ADMIN_PORT=9090 \
  ./bin/mcp-coverage
```

**Use a non-default metadata directory**
```bash
TARGET_MCP_NAME=emr-mcp \
  METADATA_DIR=/etc/mcp-coverage/emr \
  OUTPUT_DIR=/var/log/mcp-coverage \
  ./bin/mcp-coverage
```

---

## Output

### Terminal Table Report

The table report is written to stdout with ANSI color coding. Green indicates mapped, yellow indicates review_required, and red indicates unmapped.

**Summary header:**
```
═══ MCP Coverage Report — emr-mcp

  Total APIs        : 16
  Mapped            : 14
  Review Required   : 1
  Unmapped          : 1
  Coverage Rate     : 87.5%

── Coverage by Module
  ElasticSearch      100.0%  (1/1 mapped, 0 review, 0 unmapped)
  MED_010000         100.0%  (1/1 mapped, 0 review, 0 unmapped)
  diagnose           100.0%  (1/1 mapped, 0 review, 0 unmapped)
  hoo010100p01       100.0%  (2/2 mapped, 0 review, 0 unmapped)
  invoice            100.0%  (1/1 mapped, 0 review, 0 unmapped)
  oneai              100.0%  (2/2 mapped, 0 review, 0 unmapped)
  reception           83.3%  (5/6 mapped, 0 review, 1 unmapped)
  service            100.0%  (1/1 mapped, 0 review, 0 unmapped)
  MMD                  0.0%  (0/1 mapped, 1 review, 0 unmapped)
```

**Detail table columns:**

| Column | Description |
|--------|-------------|
| `MODULE` | First URL path segment; groups APIs by backend domain |
| `CONTROLLER` | Spring Controller class name |
| `METHOD` | HTTP verb |
| `API_PATH` | Request path |
| `METHOD_NAME` | Controller handler method |
| `MCP_TOOL` | Matched MCP Tool name (colored by status) |
| `STATUS` | `mapped`, `review_required`, or `unmapped` |
| `REMARK` | Matching strategy used or reason for unmapped status |

### JSON Report

Written to `{OUTPUT_DIR}/coverage_report.json`. Suitable for downstream processing by CI pipelines, dashboards, and alerting systems.

**Full structure:**

```json
{
  "generatedAt": "2026-04-28T04:24:44.101497Z",
  "targetMcp": "emr-mcp",
  "scannerUsed": "Static",
  "metrics": {
    "total": 16,
    "mapped": 14,
    "reviewRequired": 1,
    "unmapped": 1,
    "coverageRate": 87.5
  },
  "moduleCoverage": [
    {
      "module": "reception",
      "total": 6,
      "mapped": 5,
      "reviewRequired": 0,
      "unmapped": 1,
      "coverageRate": 83.3
    }
  ],
  "controllerCoverage": [
    {
      "controller": "ReceptionController",
      "module": "reception",
      "total": 6,
      "mapped": 5,
      "reviewRequired": 0,
      "unmapped": 1,
      "coverageRate": 83.3
    }
  ],
  "results": [
    {
      "module": "reception",
      "controller": "ReceptionController",
      "httpMethod": "POST",
      "apiPath": "/reception/newAdmissionEndpoint",
      "methodName": "insertAdmission",
      "summary": "Register inpatient admission",
      "mcpToolName": "",
      "mappingStatus": "unmapped",
      "remark": "no MCP tool found"
    },
    {
      "module": "hoo010100p01",
      "controller": "PatientController",
      "httpMethod": "POST",
      "apiPath": "/hoo010100p01/insertPatient",
      "methodName": "insertPatient",
      "summary": "Register a new patient record",
      "mcpToolName": "create_patient",
      "mappingStatus": "mapped",
      "remark": "explicit metadata"
    }
  ],
  "mcpTools": [
    {
      "name": "create_patient",
      "description": "환자 신규 등록...",
      "inputSchema": { "type": "object", "properties": { ... } }
    }
  ]
}
```

**Parsing unmapped APIs from the JSON report:**
```bash
cat reports/coverage_report.json \
  | jq '.results[] | select(.mappingStatus == "unmapped") | {apiPath, controller, httpMethod}'
```

**Checking coverage rate in CI:**
```bash
RATE=$(cat reports/coverage_report.json | jq '.metrics.coverageRate')
if (( $(echo "$RATE < 80" | bc -l) )); then
  echo "Coverage below threshold: $RATE%"
  exit 1
fi
```

---

## HTTP Admin API

When `ADMIN_HTTP=true`, `mcp-coverage` starts an HTTP server after analysis completes. This server exposes the collected data for integration with internal dashboards, monitoring systems, or additional automation.

**Endpoints:**

| Method | Path | Query Params | Description |
|--------|------|-------------|-------------|
| `GET` | `/coverage` | — | Top-level summary metrics |
| `GET` | `/coverage/results` | `filter=` | All mapping results; supports filter expressions |
| `GET` | `/coverage/modules` | — | Per-module coverage breakdown |
| `GET` | `/coverage/controllers` | — | Per-controller coverage breakdown |
| `GET` | `/coverage/report` | — | Full report as returned by the JSON report writer |

**Filter expressions for `/coverage/results`:**

| Filter | Returns |
|--------|---------|
| `filter=UNMAPPED` | APIs with no MCP Tool match |
| `filter=REVIEW_REQUIRED` | APIs where only similarity matching applied |
| `filter=MAPPED` | All successfully mapped APIs |
| `filter=MODULE:reception` | All APIs in the `reception` module |
| `filter=CONTROLLER:PatientController` | All APIs in the specified controller |

**Usage examples:**

```bash
# Summary
curl -s http://localhost:8080/coverage | jq .

# Unmapped only
curl -s "http://localhost:8080/coverage/results?filter=UNMAPPED" | jq '.[].apiPath'

# Module breakdown
curl -s http://localhost:8080/coverage/modules | jq '.[] | {module, coverageRate}'

# Controller breakdown for a specific team
curl -s "http://localhost:8080/coverage/results?filter=CONTROLLER:ReceptionController"
```

The server is read-only and performs no modification to any system.

---

## CI/CD Integration

`mcp-coverage` is designed to integrate into CI pipelines as a coverage gate. The JSON report provides machine-readable output; the exit code is always 0 unless the tool itself fails (not based on coverage thresholds — threshold enforcement is left to the CI script).

**Example: GitHub Actions**

```yaml
name: MCP Coverage Check

on: [push, pull_request]

jobs:
  mcp-coverage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Build mcp-coverage
        run: go build -o bin/mcp-coverage ./cmd/
        working-directory: mcp-coverage

      - name: Run MCP Coverage Analysis
        env:
          TARGET_MCP_NAME: emr-mcp
          REPORT_FORMAT: JSON
          OUTPUT_DIR: ./reports
        run: ./bin/mcp-coverage
        working-directory: mcp-coverage

      - name: Check Coverage Threshold
        run: |
          RATE=$(cat mcp-coverage/reports/coverage_report.json | jq '.metrics.coverageRate')
          UNMAPPED=$(cat mcp-coverage/reports/coverage_report.json | jq '.metrics.unmapped')
          echo "Coverage: $RATE%  |  Unmapped: $UNMAPPED"
          if (( $(echo "$RATE < 80" | bc -l) )); then
            echo "FAIL: MCP coverage below 80%"
            exit 1
          fi

      - name: Upload Coverage Report
        uses: actions/upload-artifact@v4
        with:
          name: mcp-coverage-report
          path: mcp-coverage/reports/coverage_report.json
```

**Coverage gate recommendations by team maturity:**

| Stage | Threshold | Action |
|-------|-----------|--------|
| Initial adoption | ≥ 50% mapped | Warn only |
| Active MCP development | ≥ 75% mapped | Block PR merge |
| Production steady state | ≥ 90% mapped | Block release |
| Strict governance | 0 unmapped | Block merge + require review for `review_required` |

In practice, thresholds should account for intentionally excluded APIs (health checks, internal admin endpoints, deprecated routes). Consider maintaining an exclusion list in `apis.json` via a dedicated `excluded` field if needed.

---

## Audience and Use Cases

### Platform Architects

Platform architects use `mcp-coverage` to **audit the AI agent access boundary** of the system. The key questions:

- What percentage of our backend is reachable by the AI agent?
- Are there sensitive APIs (e.g., administrative operations, data deletion endpoints) that are correctly unmapped?
- Does the coverage profile match our architectural intent?

Architects should run `mcp-coverage` at major milestones and review the module-level breakdown to ensure that high-risk modules have deliberate, documented coverage decisions.

### Backend Developers

Backend developers use `mcp-coverage` to **track the impact of their API additions**. The workflow:

1. Add new endpoint to Spring Controller.
2. Add entry to `metadata/apis.json`.
3. Run `mcp-coverage`: new entry appears as `unmapped`.
4. Create MCP Tool (or confirm with MCP team that it is intentionally excluded).
5. Add entry to `metadata/tools_metadata.json`.
6. Run `mcp-coverage`: entry moves to `mapped`.

The tool makes the handoff between backend development and MCP development explicit and auditable.

### MCP Developers

MCP developers use `mcp-coverage` to **understand their coverage obligations** and **validate their work**. Before starting an MCP Tool sprint:

```bash
TARGET_MCP_NAME=emr-mcp FILTER=UNMAPPED ./bin/mcp-coverage
```

This produces the exact list of APIs that need MCP Tools. After implementing each tool:

```bash
# Add tool mapping to tools_metadata.json
# Re-run coverage
TARGET_MCP_NAME=emr-mcp ./bin/mcp-coverage
```

The unmapped count decreases. The developer can see immediately whether their implementation resolved the gap.

### Technical Leads

Technical leads use `mcp-coverage` as a **sprint tracking and code review tool**. Coverage rate trends over time indicate whether the MCP layer is keeping pace with backend development. The `review_required` category flags items that need human judgment — these should be prioritized for review before the next release.

Technical leads should establish policies around:

- Which modules require 100% coverage before release
- Who is responsible for resolving `review_required` items
- How long an `unmapped` API is allowed to exist before escalation

### Engineering Managers

Engineering managers use the JSON report and admin API to **track progress without reading code**. A simple dashboard query against `GET /coverage` provides the executive view:

```json
{
  "total": 24,
  "mapped": 20,
  "reviewRequired": 2,
  "unmapped": 2,
  "coverageRate": 83.3
}
```

Coverage rate trends can be tracked over time by archiving the JSON report at each CI run and graphing `metrics.coverageRate` and `metrics.unmapped`.

---

## Governance Strategy

Sustainable MCP coverage governance requires both process and tooling. `mcp-coverage` provides the tooling; the following process recommendations apply to most enterprise teams.

### Ownership Model

Assign explicit ownership of `metadata/apis.json` and `metadata/tools_metadata.json`:

- **`apis.json` owner:** The backend team or platform engineering team. Updated whenever a new Spring endpoint is added. Reviewed in backend PRs.
- **`tools_metadata.json` owner:** The MCP development team. Updated whenever a new MCP Tool is created. Reviewed in MCP PRs.

Both files should be committed to the same repository as the MCP server code (or a dedicated platform repository). Changes require pull request review.

### Coverage Review Cadence

| Cadence | Activity |
|---------|----------|
| Every PR | CI runs `mcp-coverage`; results visible in PR checks |
| Weekly | Technical lead reviews `FILTER=REVIEW_REQUIRED` items; resolves or escalates |
| Monthly | Architecture review of module-level coverage trends |
| Quarterly | Audit of `tools_metadata.json` against live Spring Controller set |

### The `review_required` Lifecycle

Items in `review_required` status must not linger indefinitely. Each item requires one of three resolutions:

1. **Promote to `mapped`:** Add an explicit entry to `tools_metadata.json` confirming the similarity match was correct.
2. **Create a new MCP Tool:** The API was correctly identified as unmapped by similarity; now it needs a tool.
3. **Mark as intentionally excluded:** Add a note in `apis.json` (`"excluded": true`) and remove from tracking scope.

Teams should target zero `review_required` items at the end of each sprint.

### Breaking Change Detection

When a Spring API path changes (e.g., a refactor moves `/hoo010100p01/insertPatient` to `/patients/register`):

1. Update `apis.json` to reflect the new path.
2. Update `tools_metadata.json` to map to the new path.
3. Run `mcp-coverage` to confirm 0 unmapped.

Without this process, the old path remains in `tools_metadata.json` while the new path appears as `unmapped` — making the breakage immediately visible in CI.

---

## Best Practices

**Commit `metadata/` to version control.**
`apis.json` and `tools_metadata.json` are the authoritative record of your AI agent's access boundary. They belong in the repository, not on a developer's machine.

**Never use only similarity matching in production.**
If an API is in `review_required` status in a production system, treat it as effectively unmapped. Add the explicit metadata entry.

**Keep `apis.json` minimal and accurate.**
Only include APIs that the AI agent should potentially be able to call. Exclude internal health checks, actuator endpoints, and administrative operations that should never be agent-accessible. These exclusions are part of your security posture.

**Review `mcpTools` in the JSON report.**
The report includes the full MCP Tool list fetched from the live server. Review this list periodically to confirm that no unexpected tools have been added.

**Use `MODULE:` filters for team-based accountability.**
When multiple teams own different backend modules, each team can run `FILTER=MODULE:<name>` against their own module and be accountable for their own coverage rate.

**Archive JSON reports.**
Store `coverage_report.json` as a CI artifact on every run. This creates a historical trend dataset that can be analyzed for regression detection.

**Document intentional gaps.**
APIs that are deliberately not covered by MCP Tools (security boundaries, deprecated endpoints, admin-only operations) should be noted in `apis.json` via a `summary` explaining the exclusion reason. This prevents recurring "why is this unmapped?" questions.

---

## Security Considerations

**MCP Tool boundary as a security control.**
The set of MCP Tools registered in the server defines the operations that an AI agent can perform on behalf of a user. `mcp-coverage` makes this boundary visible and auditable. Teams should review the tool list in the JSON report (`mcpTools` field) as part of their security review process.

**Sensitive API exclusion.**
APIs that should never be agent-accessible (bulk delete operations, user privilege changes, audit log modification, direct database operations) should be excluded from `apis.json`. Including them as `unmapped` is acceptable for documentation purposes but creates noise. Consider using a dedicated `securityExcluded: true` field to distinguish security exclusions from coverage gaps.

**Metadata file access control.**
`apis.json` and `tools_metadata.json` define the coverage contract. Access to modify these files should be controlled via branch protection rules and required reviewers in the same way as application code.

**MCP client subprocess isolation.**
The MCP client spawns the target server as a subprocess using the command and environment from the Claude config file. The spawned process inherits the operator's environment. In CI, ensure that sensitive environment variables (API keys, database credentials) are not present in the CI runner environment if the MCP server does not need them for tool listing.

**Admin API exposure.**
The HTTP admin API (`ADMIN_HTTP=true`) is read-only but exposes internal API path information and tool definitions. In shared environments, restrict access to the admin port using network policies or reverse proxy authentication. Do not expose the admin API to the public internet.

**No credential storage.**
`mcp-coverage` does not store, transmit, or log any credentials. The MCP server's own authentication configuration is respected but not read or modified.

---

## Performance Considerations

**Startup time.**
The primary performance cost is spawning the MCP server subprocess and completing the JSON-RPC handshake. For a server with a large number of tools, `tools/list` may take 1–3 seconds to complete. This is acceptable for CI use; for very large tool sets (>500 tools), consider increasing the read timeout in `mcpclient/client.go`.

**Static vs. OpenAPI scanning.**
The static scanner reads a local JSON file and completes in milliseconds. The OpenAPI scanner makes HTTP requests to the Spring server and depends on network latency. In CI environments where the Spring server is running locally, expect 100–500ms for OpenAPI scanning.

**Memory footprint.**
The tool holds the full API list, tool list, and mapping results in memory simultaneously. For a typical enterprise system with hundreds of APIs and dozens of tools, memory usage is well under 50 MB.

**Admin API concurrency.**
The HTTP admin API is served by Go's standard `net/http` package, which handles concurrent requests correctly. The coverage data is computed once at startup and served read-only thereafter; there are no concurrency concerns with the data itself.

---

## Limitations

**No dynamic route discovery.**
`mcp-coverage` cannot discover Spring Controller endpoints by inspecting the Spring application's runtime state. It relies on `apis.json` or Swagger. If the Swagger spec is incomplete (e.g., controllers missing `@Tag` annotations, endpoints without `@Operation`), the static registry must compensate.

**No tool correctness verification.**
Coverage tracking confirms that a mapping declaration exists; it does not verify that the MCP Tool actually invokes the declared API correctly. A tool could be mapped to `/hoo010100p01/insertPatient` but call `/patients/create` internally. Integration tests are required to detect this.

**Similarity matching is domain-agnostic.**
The name similarity algorithm uses token overlap without domain knowledge. It cannot reason that "insert" and "create" are semantically equivalent, or that `hoo010100p01` is a legacy module name for patient management. This is a fundamental limitation of lexical matching. Explicit metadata is the only solution.

**Single MCP server per invocation.**
The tool analyzes one MCP server per run. If a backend is served by multiple MCP servers (e.g., separate servers for different departments or environments), run the tool separately for each server and compare reports manually. Aggregate multi-server reporting is a roadmap item.

**No historical trend tracking built-in.**
The tool does not store or compare historical reports. Trend analysis requires archiving JSON reports externally (CI artifacts, S3, Elasticsearch) and querying them independently.

**Subprocess model limitations.**
The MCP client spawns the server as a subprocess. If the server requires interactive authentication, GUI interaction, or special process supervision, the subprocess model may not work. This applies primarily to MCP servers that require browser-based OAuth flows before serving tool lists.

---

## Troubleshooting

**`TARGET_MCP_NAME environment variable is required`**
You did not set `TARGET_MCP_NAME`. Set it to the name of an MCP server registered in your Claude configuration.

**`MCP server "emr-mcp" not found in any config`**
The tool searched all Claude configuration files and did not find an entry matching the specified name. Run `./bin/mcp-coverage -list-servers` to see which servers are discoverable. Verify the server name matches exactly (case-sensitive).

**`cannot start "/path/to/server": exec: no such file or directory`**
The command specified in the Claude configuration does not exist or is not executable. The MCP server binary may need to be built first. Check `command` in the configuration file.

**`timeout waiting for response id=1`**
The MCP server started but did not respond to the `initialize` request within 15 seconds. This may indicate the server is starting slowly (e.g., downloading dependencies, connecting to a database) or that it does not implement the MCP protocol correctly. Run with `-v` to see the server's stderr output.

**`static scanner: cannot read ./metadata/apis.json`**
The metadata directory does not contain `apis.json`. Either set `METADATA_DIR` to the correct path or create the file. See [Static Registry](#static-registry) for the expected format.

**`parse tools_metadata.json: invalid JSON`**
`metadata/tools_metadata.json` contains a JSON syntax error. Validate with `jq . metadata/tools_metadata.json`.

**`dyld: missing LC_UUID load command` (macOS)**
The binary was compiled with Go 1.22, which does not generate the `LC_UUID` load command required by macOS 15+. Use Go 1.25 or later. The `go.mod` in this project specifies `toolchain go1.25.5`, which causes `GOTOOLCHAIN=auto` to automatically use the correct toolchain if it has been downloaded. Run `go build` again or download Go 1.25 manually.

**Coverage shows 100% but I just added a new API**
You added the new endpoint to the Spring Controller but did not add it to `metadata/apis.json`. The static scanner only knows about APIs declared in `apis.json`. Add the entry and re-run.

**A tool I created is not appearing in the tool list**
The MCP Tool must be registered with `s.AddTool(...)` in the MCP server's main function. If the tool is registered but not appearing, re-build the MCP server binary and ensure the `command` path in the Claude configuration points to the new binary.

---

## Roadmap

The following capabilities are planned for future versions. Contributions are welcome.

**v1.1 — Trend Tracking**
Built-in coverage trend storage using an embedded SQLite database. Each run appends a snapshot; the report includes a trend chart in JSON format showing coverage rate over the last N runs.

**v1.2 — Exclusion List Support**
A first-class `excluded` field in `apis.json` to mark APIs that are intentionally not covered by MCP Tools (security boundaries, deprecated endpoints). Excluded APIs do not appear in coverage totals, eliminating noise from deliberately uncovered endpoints.

**v1.3 — Multi-Server Aggregation**
Support for analyzing multiple MCP servers in a single run. Useful for organizations where different teams own separate MCP servers covering different backend domains. The aggregate report shows cross-server coverage with per-server breakdowns.

**v1.4 — Swagger Diff Mode**
When both `SWAGGER_URL` and `apis.json` are provided, a diff mode highlights APIs present in the live Swagger spec but missing from `apis.json`. This automates the process of keeping the static registry up to date.

**v1.5 — Notification Integration**
Push coverage summaries and unmapped API alerts to Slack, Microsoft Teams, or a webhook endpoint. Configurable thresholds trigger alerts when coverage drops below a specified level.

**v2.0 — Web Dashboard**
A lightweight web UI served by the admin HTTP server, providing a visual coverage matrix, trend graphs, and a searchable API/tool table. Intended for engineering managers who prefer a browser-based interface to terminal output.

**v2.1 — Spring Annotation Scanner**
Direct scanning of Spring Controller source code (via AST parsing of `.java` files) to extract API definitions without requiring a running server or manually maintained `apis.json`. Reduces maintenance overhead for teams with access to the Spring source.

**v2.2 — Policy Engine**
A declarative policy file specifying coverage requirements per module (e.g., `reception: minimum 95%`, `admin: maximum 0%`). The tool evaluates policies and returns non-zero exit codes when policies are violated, enabling fine-grained CI gates.

---

## Contributing

Contributions to metadata correctness, additional scanners, report formats, and tooling integrations are welcome. Before submitting a pull request:

1. Ensure `go build ./...` succeeds with no errors.
2. Ensure all changes to `metadata/apis.json` and `metadata/tools_metadata.json` are validated with `jq .`.
3. Do not modify any existing MCP Tool handler code or Spring Controller code in this repository.
4. Add a description of the change and its impact on coverage to the pull request description.

---

## License

Internal platform tooling. Distribution and usage policies are determined by the owning organization.
