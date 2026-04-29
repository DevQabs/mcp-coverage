# mcp-coverage

MCP API Coverage Tracker — Spring Controller API가 MCP Tool로 얼마나 커버되고 있는지 측정합니다.

## Overview

Spring 백엔드에 새 API가 추가되었는데 대응하는 MCP Tool이 없으면 AI 에이전트가 해당 기능에 접근할 수 없습니다. 이 도구는 Spring 프로젝트 소스코드를 직접 파싱하여 전체 API 목록을 수집하고, MCP Tool과 비교해 커버리지를 측정하며 미매핑 API를 식별합니다.

- **API 탐색**: Java 소스 직접 파싱 (Swagger / Actuator 불필요)
- **MCP Tool 탐색**: stdio JSON-RPC `tools/list` (Claude 설정 파일 자동 참조)
- **3단계 매핑**: 명시적 메타데이터 → 경로 매칭 → 이름 유사도
- **리포트**: 컬러 터미널 테이블 + JSON 파일

## Installation

```bash
git clone http://git.duzon.com/wehaggo-h/mcp-coverage.git
cd mcp-coverage
go build -o bin/mcp-coverage ./cmd/
```

**Requirements:** Go 1.25+

---

## MCP 서버 등록 방법

mcp-coverage는 Claude Code / Claude Desktop 설정 파일에 등록된 MCP 서버를 자동으로 탐색합니다.  
MCP 서버를 등록하는 방법은 `claude mcp add` 명령어를 사용하거나 설정 파일을 직접 수정하는 두 가지 방법이 있습니다.

### 방법 1: `claude mcp add` 명령어 (권장)

```bash
# 기본 형태
claude mcp add <서버명> <실행파일 경로>

# 환경변수 함께 지정
claude mcp add <서버명> -e KEY=VALUE -e KEY2=VALUE2 -- <실행파일 경로>

# 전역(user) 스코프로 등록 — 모든 프로젝트에서 사용 가능
claude mcp add <서버명> --scope user <실행파일 경로>

# 프로젝트 스코프로 등록 — 현재 프로젝트에서만 사용
claude mcp add <서버명> --scope project <실행파일 경로>
```

**실제 예시:**

```bash
# emr-mcp 서버 등록 (전역)
claude mcp add emr-mcp \
  --scope user \
  -e EMR_ENV=local \
  -e EMR_CONFIG=/Users/user/.emr-mcp/config.json \
  -- /path/to/emr-mcp

# kibana MCP 서버 등록
claude mcp add kibana \
  --scope user \
  -e KIBANA_URL=http://10.70.127.203 \
  -e KIBANA_DEFAULT_SPACE=default \
  -- /usr/local/bin/mcp-server-kibana

# Node.js 기반 MCP 서버 등록
claude mcp add my-node-mcp \
  --scope user \
  -- node /path/to/mcp-server/index.js

# npx 기반 MCP 서버 등록
claude mcp add my-npx-mcp \
  --scope user \
  -- npx -y @myorg/mcp-server
```

등록 확인:

```bash
# Claude Code CLI로 확인
claude mcp list

# mcp-coverage로 확인 (탐색한 모든 설정 파일의 서버 목록 출력)
./bin/mcp-coverage -list-servers
```

### 방법 2: 설정 파일 직접 수정

mcp-coverage가 탐색하는 설정 파일 위치 (우선순위 순):

| 우선순위 | 파일 경로 | 용도 |
|---------|-----------|------|
| 1 | `~/.claude/settings.json` | 전역 Claude Code 설정 |
| 2 | `~/Library/Application Support/Claude/claude_desktop_config.json` | Claude Desktop (macOS) |
| 2 | `%APPDATA%\Claude\claude_desktop_config.json` | Claude Desktop (Windows) |
| 3 | `~/.claude.json` (projects 섹션) | 프로젝트별 설정 |
| 4 | `./.claude/settings.json` | 로컬 프로젝트 설정 |

**`~/.claude/settings.json` 예시:**

```json
{
  "mcpServers": {
    "emr-mcp": {
      "command": "/path/to/emr-mcp",
      "env": {
        "EMR_ENV": "local",
        "EMR_CONFIG": "/Users/user/.emr-mcp/config.json"
      }
    },
    "kibana": {
      "command": "/usr/local/bin/mcp-server-kibana",
      "env": {
        "KIBANA_URL": "http://10.70.127.203",
        "KIBANA_DEFAULT_SPACE": "default"
      }
    }
  }
}
```

**`claude_desktop_config.json` 예시 (Claude Desktop):**

```json
{
  "mcpServers": {
    "my-api-mcp": {
      "command": "node",
      "args": ["/path/to/mcp-server/index.js"],
      "env": {
        "API_BASE_URL": "http://localhost:8080"
      }
    }
  }
}
```

---

## Usage

`TARGET_MCP_NAME`과 `TARGET_PROJECT_PATH` 두 변수가 모두 **필수**입니다.

```bash
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  ./bin/mcp-coverage
```

### 환경 변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `TARGET_MCP_NAME` | **필수** | 분석할 MCP 서버 이름 |
| `TARGET_PROJECT_PATH` | **필수** | Spring 프로젝트 루트 경로 |
| `EXCLUDE_API_PATTERNS` | — | 제외할 API 경로 glob 패턴 (쉼표 구분, 예: `/actuator/**,/error`) |
| `EXCLUDE_CONTROLLER_PATTERNS` | — | 제외할 컨트롤러 glob 패턴 (예: `*HealthCheckController`) |
| `REPORT_FORMAT` | `BOTH` | `TABLE` / `JSON` / `BOTH` |
| `FILTER` | `ALL` | `UNMAPPED` / `REVIEW_REQUIRED` / `MAPPED` / `MODULE:<name>` / `CONTROLLER:<name>` |
| `ADMIN_HTTP` | `false` | HTTP Admin API 활성화 여부 |
| `ADMIN_PORT` | `8080` | Admin API 포트 |
| `METADATA_DIR` | `./metadata` | `tools_metadata.json` 경로 |
| `OUTPUT_DIR` | `./reports` | JSON 리포트 저장 경로 |
| `DEBUG` | `false` | 상세 스캔 진단 정보 stderr 출력 |

---

## MCP 활용 예제

### 예제 1: 기본 커버리지 측정

MCP 서버를 등록한 뒤 Spring 프로젝트 소스와 비교해 커버리지를 측정합니다.

```bash
# 1. MCP 서버 등록
claude mcp add emr-mcp \
  --scope user \
  -e EMR_ENV=local \
  -- /path/to/emr-mcp

# 2. 커버리지 측정
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  ./bin/mcp-coverage
```

출력 예시:
```
═══ MCP Coverage Report — emr-mcp

  Total APIs        : 2009
  Mapped            : 12
  Review Required   : 5
  Unmapped          : 1992
  Coverage Rate     : 0.6%
```

---

### 예제 2: 미매핑 API 발굴 → MCP Tool 추가 우선순위 결정

전체 API 중 MCP Tool이 없는 것만 추려 신규 Tool 개발 대상을 파악합니다.

```bash
# 미매핑 API만 필터링해서 JSON으로 저장
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  FILTER=UNMAPPED \
  REPORT_FORMAT=JSON \
  OUTPUT_DIR=./reports \
  ./bin/mcp-coverage

# unmappedApis 목록만 추출
cat reports/coverage-*.json | jq '.unmappedApis[] | {method: .httpMethod, path: .apiPath, controller: .controllerName}'
```

출력 예시:
```json
{ "method": "POST", "path": "/MED_010000/completeClinic",  "controller": "MED_010000Controller" }
{ "method": "GET",  "path": "/reception/getPatientList",   "controller": "ReceptionController" }
{ "method": "POST", "path": "/LAB_000000/saveLabResult",   "controller": "LAB_000000Controller" }
```

이를 기반으로 신규 MCP Tool 개발 백로그를 생성할 수 있습니다.

---

### 예제 3: 특정 모듈/컨트롤러 집중 분석

대규모 프로젝트에서 특정 진료과(모듈) 또는 컨트롤러만 분석합니다.

```bash
# 접수(reception) 모듈만 분석
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  FILTER=MODULE:reception \
  ./bin/mcp-coverage

# 특정 컨트롤러만 분석
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  FILTER=CONTROLLER:MED_010000Controller \
  ./bin/mcp-coverage

# 시스템 경로 제외
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  EXCLUDE_API_PATTERNS="/actuator/**,/error,/health" \
  EXCLUDE_CONTROLLER_PATTERNS="*HealthCheckController,*SwaggerController" \
  ./bin/mcp-coverage
```

---

### 예제 4: Admin HTTP API로 실시간 조회

커버리지 측정 후 HTTP API로 결과를 조회합니다. 대시보드 연동이나 다른 시스템과의 통합에 활용합니다.

```bash
# Admin API 활성화 (백그라운드 실행)
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  ADMIN_HTTP=true \
  ADMIN_PORT=8080 \
  ./bin/mcp-coverage &

# 전체 커버리지 지표 조회
curl http://localhost:8080/coverage

# 미매핑 API 목록 조회
curl http://localhost:8080/coverage/unmapped

# 모듈별 커버리지 조회
curl http://localhost:8080/coverage/modules

# 특정 상태로 필터링
curl "http://localhost:8080/coverage/results?status=review_required"

# 전체 JSON 리포트 다운로드
curl http://localhost:8080/coverage/report -o report.json
```

---

### 예제 5: CI 파이프라인 통합

PR 빌드 시 커버리지 리포트를 자동 생성합니다.

```yaml
# .github/workflows/mcp-coverage.yml
- name: MCP Coverage Check
  run: |
    TARGET_MCP_NAME=emr-mcp \
      TARGET_PROJECT_PATH=${{ github.workspace }} \
      REPORT_FORMAT=JSON \
      OUTPUT_DIR=./reports \
      ./bin/mcp-coverage

    COVERAGE=$(cat reports/coverage-*.json | jq '.summary.coverageRate')
    echo "MCP Coverage: ${COVERAGE}%"

- name: Upload Coverage Report
  uses: actions/upload-artifact@v4
  with:
    name: mcp-coverage-report
    path: reports/coverage-*.json
```

---

### 예제 6: 매핑 메타데이터로 정확도 향상

자동 매핑이 어려운 Tool은 `metadata/tools_metadata.json`에 직접 등록해 정확도를 높입니다.

```bash
# 1. Review Required 상태 확인
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  FILTER=REVIEW_REQUIRED \
  ./bin/mcp-coverage
```

`metadata/tools_metadata.json` 작성:

```json
{
  "complete_clinic": {
    "apiPath": "/MED_010000/completeClinic",
    "httpMethod": "POST",
    "controllerName": "MED_010000Controller",
    "methodName": "completeClinic"
  },
  "complete_payment": {
    "apis": [
      { "apiPath": "/reception/selectOverallCalculationInfo", "httpMethod": "GET" },
      { "apiPath": "/reception/insertPayData", "httpMethod": "POST" }
    ]
  }
}
```

```bash
# 2. 메타데이터 반영 후 재측정
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  METADATA_DIR=./metadata \
  ./bin/mcp-coverage
```

---

## API 탐색 (Java Source Scanner)

`TARGET_PROJECT_PATH`를 설정하면 Java 소스 파일을 직접 파싱합니다. Swagger나 Actuator 없이 프로젝트 전체 API를 탐지합니다.

### 탐지 가능한 패턴

- `@RestController`, `@Controller` (패키지 경로 포함 fully-qualified 형태 포함)
- **커스텀 메타 어노테이션** — `@RestController`를 메타로 사용하는 어노테이션 자동 감지
  ```java
  // 어노테이션 정의
  @RestController
  @RequestMapping(method = {RequestMethod.GET, RequestMethod.POST})
  public @interface HealthRestController {
      @AliasFor(annotation = RequestMapping.class, attribute = "path")
      String[] value() default {};
  }

  // 실제 사용 — 자동 탐지됨
  @HealthRestController("/MED_010000")
  public class MED_010000Controller { ... }
  ```
- `@RequestMapping`, `@GetMapping`, `@PostMapping`, `@PutMapping`, `@DeleteMapping`, `@PatchMapping`
- 클래스 레벨 + 메서드 레벨 경로 조합
- 복수 경로: `@GetMapping({"/patients", "/members"})`
- 복수 HTTP 메서드: `@RequestMapping(method = {RequestMethod.GET, RequestMethod.POST})`
- `value` / `path` 속성: `@GetMapping(value = "/patients")`, `@GetMapping(path = "/patients")`
- 인터페이스 컨트롤러 (Feign Client 스타일 API 계약)
- 추상 기반 컨트롤러의 추상 메서드 매핑
- 멀티 모듈 Gradle/Maven 프로젝트 (중첩된 `src/main/java` 루트 전체 탐색)
- `src/test` 디렉토리 자동 제외

### 경로 상수 해석

컨트롤러가 `@GetMapping(ApiPaths.PATIENT_BASE)`처럼 상수를 사용하는 경우, 스캐너는 스캔 전 프로젝트 전체의 `final String` 선언으로 상수 레지스트리를 먼저 구축합니다.

- 해석 성공 → 일반 엔트리로 포함
- 해석 실패 → `UNRESOLVED:<ref>` 형태로 포함 (누락 없이 리포트에 표시)

### 메타 어노테이션 자동 감지

스캔 전에 프로젝트 내 모든 `@interface` 선언을 탐색하여 `@RestController` 또는 `@Controller`가 붙어 있으면 자동으로 커스텀 컨트롤러 어노테이션으로 등록합니다. 별도 설정 없이 동작합니다.

---

## Debug 출력

`DEBUG=true` 시 스캔 진단 정보 출력:

```
[JavaSource Debug] ─────────────────────────────
  Project path          : /path/to/project
  Scanned .java files   : 5786
  Skipped files         : 5343
  Controllers found     : 443
    interfaces          : 12
    abstract classes    : 3
  Methods inspected     : 2341
  APIs detected         : 2009
  APIs excluded         : 47
  Unresolved paths      : 8
  Duplicate paths       : 2
    POST /api/patients
    GET  /api/common/code
────────────────────────────────────────────────
```

---

## 출력 예시

```
═══ MCP Coverage Report — emr-mcp

  Total APIs        : 2009
  Mapped            : 12
  Review Required   : 5
  Unmapped          : 1992
  Coverage Rate     : 0.6%

── Coverage by Module
  reception        75.0%  (6/8 mapped, 1 review, 1 unmapped)
  oneai           100.0%  (2/2 mapped, 0 review, 0 unmapped)
  lab               0.0%  (0/124 mapped, 0 review, 124 unmapped)
  nursing           0.0%  (0/98 mapped, 0 review, 98 unmapped)
  ...
```

### JSON 리포트 구조

```json
{
  "generatedAt": "2026-04-29T10:00:00Z",
  "targetMcp": "emr-mcp",
  "summary": {
    "totalApiCount": 2009,
    "mappedApiCount": 12,
    "reviewRequiredCount": 5,
    "unmappedApiCount": 1992,
    "coverageRate": 0.6
  },
  "unmappedApis": [
    {
      "httpMethod": "POST",
      "apiPath": "/MED_010000/completeClinic",
      "module": "MED_010000",
      "controllerName": "MED_010000Controller",
      "methodName": "completeClinic",
      "sourceFile": "emr-service/src/main/java/com/example/MED_010000Controller.java",
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

경로 상수 미해석 항목은 `"scanStatus": "partial"` 및 `"scanReason": "method path constant unresolved: ApiPaths.SEARCH"` 필드가 추가됩니다.

---

## HTTP Admin API

`ADMIN_HTTP=true` 설정 시:

| 엔드포인트 | 설명 |
|-----------|------|
| `GET /coverage` | 전체 커버리지 지표 |
| `GET /coverage/results?status=unmapped` | 상태별 필터 (`mapped` / `review_required` / `unmapped`) |
| `GET /coverage/unmapped` | 미매핑 API 목록 |
| `GET /coverage/modules` | 모듈별 커버리지 |
| `GET /coverage/controllers` | 컨트롤러별 커버리지 |
| `GET /coverage/report` | 전체 JSON 리포트 |

---

## 매핑 메타데이터 관리

MCP Tool이 어떤 API를 호출하는지 명시적으로 등록하면 자동 매핑보다 높은 우선순위로 처리됩니다.

`metadata/tools_metadata.json`에 추가 (코드 변경 불필요):

```json
{
  "complete_clinic": {
    "apiPath": "/MED_010000/completeClinic",
    "httpMethod": "POST",
    "controllerName": "MED_010000Controller",
    "methodName": "completeClinic"
  },
  "complete_payment": {
    "apis": [
      { "apiPath": "/reception/selectOverallCalculationInfo", "httpMethod": "GET" },
      { "apiPath": "/reception/insertPayData", "httpMethod": "POST" }
    ]
  }
}
```

복수 API를 호출하는 Tool은 `apis` 배열 형태로 등록합니다.

---

## 매핑 우선순위

| 우선순위 | 방식 | 결과 상태 |
|---------|------|-----------|
| 1 | `tools_metadata.json` 명시적 등록 | `mapped` |
| 2 | `apiPath` + `httpMethod` 경로 매칭 | `mapped` |
| 3 | 컨트롤러/메서드명 유사도 ≥ 0.5 | `mapped` |
| 4 | 컨트롤러/메서드명 유사도 ≥ 0.25 | `review_required` |
| — | 매칭 없음 | `unmapped` |

명시적 메타데이터가 항상 최우선입니다. `review_required` 항목은 수동 확인이 필요합니다.

---

## Testing

```bash
go test ./...
```
