# mcp-coverage

Spring Controller API가 MCP Tool로 얼마나 커버되고 있는지 측정하는 **CLI 분석 도구**입니다.

> **mcp-coverage는 MCP 서버가 아닙니다.**
> Claude에 MCP 서버로 등록하는 것이 아니라, 이미 Claude에 등록된 MCP 서버를 분석하는 CLI 도구입니다.
> 터미널에서 직접 실행하거나, Claude Code의 Bash 도구를 통해 활용합니다.

---

## 배경

Spring 백엔드에 새 API가 추가되었는데 대응하는 MCP Tool이 없으면 AI 에이전트가 해당 기능에 접근할 수 없습니다.
mcp-coverage는 Spring 프로젝트 소스코드를 직접 파싱하여 전체 API 목록을 수집하고, 등록된 MCP Tool과 비교해 커버리지를 측정하며 미매핑 API를 식별합니다.

- **API 탐색**: Java 소스 직접 파싱 (Swagger / Actuator 불필요)
- **MCP Tool 탐색**: stdio JSON-RPC `tools/list` (Claude 설정 파일 자동 참조)
- **3단계 매핑**: 명시적 메타데이터 → 경로 매칭 → 이름 유사도
- **리포트**: 컬러 터미널 테이블 + JSON 파일

---

## Installation

```bash
git clone http://git.duzon.com/wehaggo-h/mcp-coverage.git
cd mcp-coverage
go build -o bin/mcp-coverage ./cmd/
```

**Requirements:** Go 1.25+

---

## 동작 원리

```
1. Claude 설정 파일에서 TARGET_MCP_NAME 서버 설정 자동 탐색
        ↓
2. 해당 MCP 서버를 stdio로 실행 → tools/list 호출로 Tool 목록 수집
        ↓
3. TARGET_PROJECT_PATH Java 소스 파싱 → 전체 API 목록 수집
        ↓
4. API ↔ Tool 3단계 매핑 (메타데이터 → 경로 → 유사도)
        ↓
5. 커버리지 계산 + 리포트 출력 (터미널 테이블 / JSON)
```

### Claude 설정 파일 탐색 위치

mcp-coverage는 아래 파일들을 **자동으로 탐색**하여 등록된 MCP 서버 정보를 읽어옵니다.

| 우선순위 | 파일 경로 |
|---------|----------|
| 1 | `~/.claude/settings.json` |
| 2 | `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) |
| 2 | `%APPDATA%\Claude\claude_desktop_config.json` (Windows) |
| 3 | `~/.claude.json` (projects 섹션) |
| 4 | `./.claude/settings.json` |

**분석하려는 MCP 서버는 위 파일 중 하나에 미리 등록되어 있어야 합니다.**

---

## 분석 대상 MCP 서버 등록 방법

mcp-coverage를 실행하기 전에, 분석할 MCP 서버를 Claude에 먼저 등록해야 합니다.

### `claude mcp add` 명령어 (권장)

```bash
# 기본 형태
claude mcp add <서버명> -- <실행파일 경로>

# 환경변수 포함
claude mcp add <서버명> -e KEY=VALUE -e KEY2=VALUE2 -- <실행파일 경로>

# 전역 등록 (모든 프로젝트에서 사용 가능)
claude mcp add <서버명> --scope user -- <실행파일 경로>

# 특정 프로젝트에만 등록
claude mcp add <서버명> --scope project -- <실행파일 경로>
```

등록 확인:

```bash
# Claude CLI로 등록된 MCP 서버 목록 확인
claude mcp list

# mcp-coverage가 탐색한 서버 목록 확인
./bin/mcp-coverage --list-servers
```

### 설정 파일 직접 수정

`~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "my-mcp-server": {
      "command": "/path/to/my-mcp-server",
      "args": ["--some-option"],
      "env": {
        "ENV_KEY": "value"
      }
    }
  }
}
```

---

## Usage

`TARGET_MCP_NAME`과 `TARGET_PROJECT_PATH` 두 변수가 모두 **필수**입니다.

```bash
TARGET_MCP_NAME=<서버명> \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  ./bin/mcp-coverage
```

### 환경 변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `TARGET_MCP_NAME` | **필수** | 분석할 MCP 서버 이름 (Claude 설정에 등록된 이름과 일치해야 함) |
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

## Claude Code에서 활용하기

mcp-coverage는 CLI 도구이므로 Claude Code에서는 **Bash 도구**를 통해 실행합니다.
Claude Code에 자연어로 요청하면 Claude가 아래 명령들을 직접 실행하고 결과를 해석해 줍니다.

---

### 예제 1: 기본 커버리지 측정

Claude Code에 요청:

```
my-mcp-server의 커버리지를 측정해줘.
Spring 프로젝트 경로는 /path/to/spring-project야.
```

Claude Code가 내부적으로 실행하는 명령:

```bash
TARGET_MCP_NAME=my-mcp-server \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  ./bin/mcp-coverage
```

출력 예시:

```
Connecting to MCP server "my-mcp-server"...
  Found 17 MCP tools
Scanning APIs from project: /path/to/spring-project
  Found 2009 backend APIs

═══ MCP Coverage Report — my-mcp-server

  Total APIs        : 2009
  Mapped            : 12
  Review Required   : 5
  Unmapped          : 1992
  Coverage Rate     : 0.6%

── Coverage by Module
  reception        75.0%  (6/8 mapped)
  orders          100.0%  (2/2 mapped)
  lab               0.0%  (0/124 mapped)
  ...
```

---

### 예제 2: 미매핑 API 목록 추출 → Tool 개발 우선순위 결정

Claude Code에 요청:

```
아직 MCP Tool이 없는 API 목록을 JSON으로 뽑아줘.
우선순위 높은 것부터 정리해줘.
```

Claude Code가 내부적으로 실행:

```bash
TARGET_MCP_NAME=my-mcp-server \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  FILTER=UNMAPPED \
  REPORT_FORMAT=JSON \
  OUTPUT_DIR=./reports \
  ./bin/mcp-coverage
```

생성된 `reports/coverage_*.json`에서 `unmappedApis` 배열을 읽어 Claude가 분석하고 우선순위를 정리해줍니다.

---

### 예제 3: Admin HTTP API로 상시 조회

스캔 결과를 서버로 상시 올려두고 여러 번 조회할 때 유용합니다.

```bash
# 백그라운드로 실행
TARGET_MCP_NAME=my-mcp-server \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  ADMIN_HTTP=true \
  ADMIN_PORT=8080 \
  ./bin/mcp-coverage &
```

Claude Code에 요청:

```
미매핑 API 목록 가져와줘.
모듈별 커버리지도 보여줘.
```

Claude Code가 내부적으로 호출:

```bash
curl http://localhost:8080/coverage            # 전체 지표
curl http://localhost:8080/coverage/unmapped   # 미매핑 API
curl http://localhost:8080/coverage/modules    # 모듈별 커버리지
curl "http://localhost:8080/coverage/results?status=review_required"
```

Admin API 전체 엔드포인트:

| 엔드포인트 | 설명 |
|-----------|------|
| `GET /coverage` | 전체 커버리지 지표 |
| `GET /coverage/results?status=mapped\|review_required\|unmapped` | 상태별 매핑 결과 |
| `GET /coverage/unmapped` | 미매핑 API 목록 |
| `GET /coverage/modules` | 모듈별 커버리지 |
| `GET /coverage/controllers` | 컨트롤러별 커버리지 |
| `GET /coverage/report` | 전체 JSON 리포트 |

---

### 예제 4: Review Required 항목 수동 확인 후 메타데이터 등록

자동 매핑이 불확실한 항목을 확인해 `tools_metadata.json`에 등록하면 정확도가 높아집니다.

Claude Code에 요청:

```
Review Required 상태인 API들 보여줘.
확인하고 tools_metadata.json에 등록해줘.
```

Claude Code가 내부적으로 실행:

```bash
TARGET_MCP_NAME=my-mcp-server \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  FILTER=REVIEW_REQUIRED \
  REPORT_FORMAT=JSON \
  ./bin/mcp-coverage
```

확인 후 `metadata/tools_metadata.json`에 등록 (단일 API):

```json
{
  "tool_name_a": {
    "apiPath": "/module/someAction",
    "httpMethod": "POST"
  }
}
```

복수 API를 호출하는 Tool인 경우 `apis` 배열 형태로 등록:

```json
{
  "tool_name_b": {
    "apis": [
      { "apiPath": "/module/step1", "httpMethod": "GET" },
      { "apiPath": "/module/step2", "httpMethod": "POST" }
    ]
  }
}
```

재측정:

```bash
TARGET_MCP_NAME=my-mcp-server \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  METADATA_DIR=./metadata \
  ./bin/mcp-coverage
```

---

### 예제 5: 특정 모듈 / 컨트롤러 집중 분석

```bash
# 특정 모듈만 분석
TARGET_MCP_NAME=my-mcp-server \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  FILTER=MODULE:reception \
  ./bin/mcp-coverage

# 특정 컨트롤러만 분석
TARGET_MCP_NAME=my-mcp-server \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  FILTER=CONTROLLER:PatientController \
  ./bin/mcp-coverage

# 시스템 경로 및 특정 컨트롤러 제외
TARGET_MCP_NAME=my-mcp-server \
  TARGET_PROJECT_PATH=/path/to/spring-project \
  EXCLUDE_API_PATTERNS="/actuator/**,/error,/health" \
  EXCLUDE_CONTROLLER_PATTERNS="*HealthCheckController,*TestController" \
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
  public @interface MyRestController {
      @AliasFor(annotation = RequestMapping.class, attribute = "path")
      String[] value() default {};
  }

  // 실제 사용 — 자동 탐지됨
  @MyRestController("/some/base/path")
  public class SomeController { ... }
  ```

- `@RequestMapping`, `@GetMapping`, `@PostMapping`, `@PutMapping`, `@DeleteMapping`, `@PatchMapping`
- 클래스 레벨 + 메서드 레벨 경로 조합
- 복수 경로: `@GetMapping({"/patients", "/members"})`
- 복수 HTTP 메서드: `@RequestMapping(method = {RequestMethod.GET, RequestMethod.POST})`
- `value` / `path` 속성: `@GetMapping(value = "/patients")`
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
────────────────────────────────────────────────
```

---

## 출력 예시

```
═══ MCP Coverage Report — my-mcp-server

  Total APIs        : 2009
  Mapped            : 12
  Review Required   : 5
  Unmapped          : 1992
  Coverage Rate     : 0.6%

── Coverage by Module
  reception        75.0%  (6/8 mapped, 1 review, 1 unmapped)
  orders          100.0%  (2/2 mapped, 0 review, 0 unmapped)
  lab               0.0%  (0/124 mapped, 0 review, 124 unmapped)
  nursing           0.0%  (0/98 mapped, 0 review, 98 unmapped)
  ...
```

### JSON 리포트 구조

```json
{
  "generatedAt": "2026-04-29T10:00:00Z",
  "targetMcp": "my-mcp-server",
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
      "apiPath": "/some/module/someAction",
      "module": "some-module",
      "controllerName": "SomeController",
      "methodName": "someAction",
      "sourceFile": "module/src/main/java/com/example/SomeController.java",
      "lineNumber": 42,
      "mcpToolName": null,
      "status": "unmapped",
      "reason": "No matching MCP Tool found"
    }
  ]
}
```

경로 상수 미해석 항목은 `"scanStatus": "partial"` 및 `"scanReason": "method path constant unresolved: ApiPaths.SEARCH"` 필드가 추가됩니다.

---

## 매핑 우선순위

| 우선순위 | 방식 | 결과 상태 |
|---------|------|----------|
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
