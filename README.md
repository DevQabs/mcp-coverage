# mcp-coverage

MCP API Coverage Tracker — measures how well your Spring Controller APIs are covered by MCP Tools.

## Overview

When a new API is added to a Spring backend without a corresponding MCP Tool, AI agents cannot access it. This tool compares the full list of Spring APIs against MCP Tools, measures coverage, and identifies unmapped APIs.

- **API discovery**: Java source scanner (primary), Spring Actuator, Swagger/OpenAPI, or static registry
- **MCP Tool discovery**: connects directly to the target MCP server via `tools/list`
- **3-tier mapping**: explicit metadata → path matching → name similarity
- **Reports**: color terminal table + JSON file

## Installation

```bash
git clone https://github.com/DevQabs/mcp-coverage
cd mcp-coverage
go build -o bin/mcp-coverage ./cmd/
```

**Requirements:** Go 1.25+

## Usage

Only `TARGET_MCP_NAME` is required. Connection details are automatically resolved from your Claude config files.

```bash
TARGET_MCP_NAME=emr-mcp ./bin/mcp-coverage
```

### Common options

```bash
# Scan Spring source code directly — discovers all APIs including Swagger-hidden ones
TARGET_MCP_NAME=emr-mcp TARGET_PROJECT_PATH=/path/to/spring-project ./bin/mcp-coverage

# Also merge routes from a running Spring app's /actuator/mappings endpoint
TARGET_MCP_NAME=emr-mcp TARGET_PROJECT_PATH=/path/to/project ACTUATOR_URL=http://localhost:8080 ./bin/mcp-coverage

# Show only unmapped APIs
TARGET_MCP_NAME=emr-mcp FILTER=UNMAPPED ./bin/mcp-coverage

# Filter by module or controller
TARGET_MCP_NAME=emr-mcp FILTER=MODULE:reception ./bin/mcp-coverage
TARGET_MCP_NAME=emr-mcp FILTER=CONTROLLER:PatientController ./bin/mcp-coverage

# Exclude noisy paths and controllers
TARGET_MCP_NAME=emr-mcp \
  EXCLUDE_API_PATTERNS="/actuator/**,/error" \
  EXCLUDE_CONTROLLER_PATTERNS="*HealthCheckController" \
  TARGET_PROJECT_PATH=/path/to/project \
  ./bin/mcp-coverage

# JSON report only (CI use)
TARGET_MCP_NAME=emr-mcp REPORT_FORMAT=JSON OUTPUT_DIR=./reports ./bin/mcp-coverage

# Enable HTTP admin API
TARGET_MCP_NAME=emr-mcp ADMIN_HTTP=true ADMIN_PORT=8080 ./bin/mcp-coverage

# Print detailed scan diagnostics
TARGET_MCP_NAME=emr-mcp TARGET_PROJECT_PATH=/path/to/project DEBUG=true ./bin/mcp-coverage

# List all discovered MCP servers
./bin/mcp-coverage -list-servers
```

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TARGET_MCP_NAME` | **required** | Name of the MCP server to analyze |
| `TARGET_PROJECT_PATH` | — | Root path of Spring project (enables Java source scanner) |
| `ACTUATOR_URL` | — | Base URL of running Spring app for `/actuator/mappings` merge |
| `SWAGGER_URL` | — | Spring server base URL for OpenAPI scanning |
| `EXCLUDE_API_PATTERNS` | — | Comma-separated glob patterns to exclude API paths (e.g. `/actuator/**,/error`) |
| `EXCLUDE_CONTROLLER_PATTERNS` | — | Comma-separated glob patterns to exclude controllers (e.g. `*HealthCheckController`) |
| `REPORT_FORMAT` | `BOTH` | `TABLE` / `JSON` / `BOTH` |
| `FILTER` | `ALL` | `UNMAPPED` / `REVIEW_REQUIRED` / `MAPPED` / `MODULE:<name>` / `CONTROLLER:<name>` |
| `ADMIN_HTTP` | `false` | Enable HTTP admin API |
| `ADMIN_PORT` | `8080` | Admin API port |
| `METADATA_DIR` | `./metadata` | Path to `apis.json` and `tools_metadata.json` |
| `OUTPUT_DIR` | `./reports` | JSON report output directory |
| `DEBUG` | `false` | Print detailed scan diagnostics to stderr |

## API discovery (Java source scanner)

When `TARGET_PROJECT_PATH` is set, the scanner directly parses Java source files — no Swagger required.

**Detects:**
- `@RestController`, `@Controller` (including fully qualified `org.springframework.web.bind.annotation.*` forms)
- `@RequestMapping`, `@GetMapping`, `@PostMapping`, `@PutMapping`, `@DeleteMapping`, `@PatchMapping`
- Class-level + method-level path combination
- Multiple paths: `@GetMapping({"/patients", "/members"})`
- Multiple HTTP methods: `@RequestMapping(method = {RequestMethod.GET, RequestMethod.POST})`
- `value` and `path` attributes: `@GetMapping(value = "/patients")`, `@GetMapping(path = "/patients")`
- Interface controllers (feign-client style API contracts)
- Abstract base controllers with abstract method mappings
- Multi-module Gradle/Maven projects (all nested `src/main/java` roots)

**Path constant resolution:**

If controllers use path constants like `@GetMapping(ApiPaths.PATIENT_BASE)`, the scanner builds a project-wide constant registry from `final String` declarations before scanning. Resolved constants produce normal entries; unresolvable constants produce `UNRESOLVED:<ref>` entries that are still included in the report rather than silently dropped.

**Discovery priority (merged in this order):**
1. Java source annotation scanner
2. `/actuator/mappings` endpoint (if `ACTUATOR_URL` set)
3. OpenAPI/Swagger (if `SWAGGER_URL` set)
4. Static registry (`metadata/apis.json`)

## Debug output

`DEBUG=true` prints a full diagnostics report:

```
[JavaSource Debug] ─────────────────────────────
  Project path          : /path/to/project
  Scanned .java files   : 142
  Skipped files         : 891
  Controllers found     : 23
    interfaces          : 3
    abstract classes    : 1
  Methods inspected     : 187
  APIs detected         : 164
  APIs excluded         : 12
  Unresolved paths      : 4
  Actuator-only APIs    : 2
  Duplicate paths       : 1
    GET /api/patients
────────────────────────────────────────────────
```

## Example output

```
═══ MCP Coverage Report — emr-mcp

  Total APIs        : 36
  Mapped            : 16
  Review Required   : 3
  Unmapped          : 17
  Coverage Rate     : 44.4%

── Coverage by Module
  lab              0.0%  (0/3 mapped, 0 review, 3 unmapped)
  nursing          0.0%  (0/2 mapped, 0 review, 2 unmapped)
  reception       75.0%  (6/8 mapped, 1 review, 1 unmapped)
  oneai          100.0%  (2/2 mapped, 0 review, 0 unmapped)
```

### JSON report structure

```json
{
  "generatedAt": "2026-04-28T04:24:44Z",
  "targetMcp": "emr-mcp",
  "summary": {
    "totalApiCount": 36,
    "mappedApiCount": 16,
    "reviewRequiredCount": 3,
    "unmappedApiCount": 17,
    "coverageRate": 44.44
  },
  "unmappedApis": [
    {
      "httpMethod": "POST",
      "apiPath": "/lab/insertLabOrder",
      "module": "lab",
      "controllerName": "LabOrderController",
      "methodName": "insertLabOrder",
      "sourceFile": "lab-service/src/main/java/com/example/lab/LabOrderController.java",
      "lineNumber": 42,
      "mcpToolName": null,
      "status": "unmapped",
      "reason": "No matching MCP Tool found"
    }
  ],
  "moduleCoverage": [...],
  "controllerCoverage": [...],
  "results": [...]
}
```

Entries with unresolvable path constants include `"scanStatus": "partial"` and `"scanReason": "method path constant unresolved: ApiPaths.SEARCH"`.

## HTTP admin API

When `ADMIN_HTTP=true`:

| Endpoint | Description |
|----------|-------------|
| `GET /coverage` | Overall coverage metrics |
| `GET /coverage/results?status=unmapped` | Filter by status (`mapped` / `review_required` / `unmapped`) |
| `GET /coverage/unmapped` | Unmapped API list |
| `GET /coverage/modules` | Coverage by module |
| `GET /coverage/controllers` | Coverage by controller |
| `GET /coverage/report` | Full JSON report |

## Metadata management

### Adding a new API (static registry)

Add to `metadata/apis.json` → automatically shown as `unmapped`:

```json
{
  "module": "lab",
  "controller": "LabOrderController",
  "httpMethod": "POST",
  "apiPath": "/lab/insertLabOrder",
  "methodName": "insertLabOrder",
  "summary": "Create lab order"
}
```

### Adding an MCP Tool mapping

Add to `metadata/tools_metadata.json` (no code changes needed):

```json
"create_lab_order": {
  "apiPath": "/lab/insertLabOrder",
  "httpMethod": "POST",
  "controllerName": "LabOrderController",
  "methodName": "insertLabOrder"
}
```

Tools that call multiple APIs:

```json
"complete_payment": {
  "apis": [
    { "apiPath": "/reception/selectOverallCalculationInfo", "httpMethod": "GET" },
    { "apiPath": "/reception/insertPayData", "httpMethod": "POST" }
  ]
}
```

## Mapping priority

| Priority | Method | Result status |
|----------|--------|---------------|
| 1 | Explicit `tools_metadata.json` mapping | `mapped` |
| 2 | `apiPath` + `httpMethod` path match | `mapped` |
| 3 | Controller/method name similarity ≥ 0.5 | `mapped` |
| 3 | Controller/method name similarity ≥ 0.25 | `review_required` |
| — | No match | `unmapped` |

Explicit metadata always takes highest priority. `review_required` items need manual verification.

## Testing

```bash
go test ./...
```
