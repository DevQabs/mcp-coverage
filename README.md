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

## Usage

`TARGET_MCP_NAME`과 `TARGET_PROJECT_PATH` 두 변수가 모두 **필수**입니다.

```bash
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  ./bin/mcp-coverage
```

### 사용 예시

```bash
# 미매핑 API만 출력
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/project \
  FILTER=UNMAPPED \
  ./bin/mcp-coverage

# 특정 모듈 또는 컨트롤러만 필터
TARGET_MCP_NAME=emr-mcp TARGET_PROJECT_PATH=/path/to/project FILTER=MODULE:reception ./bin/mcp-coverage
TARGET_MCP_NAME=emr-mcp TARGET_PROJECT_PATH=/path/to/project FILTER=CONTROLLER:PatientController ./bin/mcp-coverage

# 불필요한 경로·컨트롤러 제외
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/project \
  EXCLUDE_API_PATTERNS="/actuator/**,/error" \
  EXCLUDE_CONTROLLER_PATTERNS="*HealthCheckController" \
  ./bin/mcp-coverage

# JSON 리포트만 출력 (CI 용)
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/project \
  REPORT_FORMAT=JSON \
  OUTPUT_DIR=./reports \
  ./bin/mcp-coverage

# HTTP Admin API 활성화
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/project \
  ADMIN_HTTP=true \
  ADMIN_PORT=8080 \
  ./bin/mcp-coverage

# 상세 스캔 진단 출력
TARGET_MCP_NAME=emr-mcp \
  TARGET_PROJECT_PATH=/path/to/project \
  DEBUG=true \
  ./bin/mcp-coverage

# 등록된 MCP 서버 목록 출력
./bin/mcp-coverage -list-servers
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

## 매핑 우선순위

| 우선순위 | 방식 | 결과 상태 |
|---------|------|-----------|
| 1 | `tools_metadata.json` 명시적 등록 | `mapped` |
| 2 | `apiPath` + `httpMethod` 경로 매칭 | `mapped` |
| 3 | 컨트롤러/메서드명 유사도 ≥ 0.5 | `mapped` |
| 4 | 컨트롤러/메서드명 유사도 ≥ 0.25 | `review_required` |
| — | 매칭 없음 | `unmapped` |

명시적 메타데이터가 항상 최우선입니다. `review_required` 항목은 수동 확인이 필요합니다.

## Testing

```bash
go test ./...
```
