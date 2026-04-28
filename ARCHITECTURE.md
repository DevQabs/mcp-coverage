# mcp-coverage — 아키텍처 분석 문서

> Spring Controller API와 MCP Tool 간의 커버리지를 측정하는 Go CLI 도구

---

## 목차

1. [프로젝트 개요](#1-프로젝트-개요)
2. [전체 아키텍처](#2-전체-아키텍처)
3. [패키지 구조](#3-패키지-구조)
4. [실행 흐름 (8단계)](#4-실행-흐름-8단계)
5. [핵심 컴포넌트 상세](#5-핵심-컴포넌트-상세)
6. [Java Source Scanner 내부 동작](#6-java-source-scanner-내부-동작)
7. [3-Tier 매핑 알고리즘](#7-3-tier-매핑-알고리즘)
8. [데이터 흐름 다이어그램](#8-데이터-흐름-다이어그램)
9. [핵심 타입 관계도](#9-핵심-타입-관계도)
10. [확장 포인트](#10-확장-포인트)

---

## 1. 프로젝트 개요

`mcp-coverage`는 **Spring 백엔드 API와 MCP(Model Context Protocol) Tool 사이의 커버리지 갭을 자동으로 감지**하는 Go 기반 CLI 도구다.

### 해결하는 문제

AI 에이전트(Claude 등)는 MCP Tool을 통해서만 백엔드 API에 접근할 수 있다. Spring 프로젝트에 새 API가 추가되었는데 대응하는 MCP Tool이 없으면, AI는 그 기능을 활용하지 못한다. `mcp-coverage`는 이 갭을 자동으로 측정한다.

```
Spring Controller API  ──→  [mcp-coverage]  ──→  Coverage Report
MCP Tool List          ──→                  ──→  Unmapped APIs
```

### 기술 스택

| 항목 | 내용 |
|------|------|
| 언어 | Go 1.25+ |
| 외부 의존성 | 없음 (표준 라이브러리만 사용) |
| 통신 방식 | MCP stdio JSON-RPC 2.0 |
| 빌드 | `go build -o bin/mcp-coverage ./cmd/` |

---

## 2. 전체 아키텍처

```
┌──────────────────────────────────────────────────────────────────┐
│                         cmd/main.go                              │
│  (진입점 · 설정 로드 · 8단계 오케스트레이션)                      │
└───────────┬──────────────────────────────────────────────────────┘
            │
     ┌──────┴──────┐
     ▼             ▼
┌─────────┐  ┌────────────────────────────────────────────────────┐
│ config  │  │                    파이프라인                        │
│ 패키지   │  │                                                      │
│ (환경변수│  │  ┌───────────┐    ┌────────────┐    ┌──────────┐   │
│  로드)  │  │  │ mcpconfig │    │  mcpclient │    │apiscanner│   │
└─────────┘  │  │ (서버 설정 │───▶│ (MCP 연결  │    │ (API 수집 │   │
             │  │  자동 탐지) │    │  tool 목록) │    │ Java/OAS/ │   │
             │  └───────────┘    └─────┬──────┘    │  Static)  │   │
             │                         │            └────┬──────┘   │
             │                    MCP Tools          APIs│           │
             │                         └────────┬────────┘           │
             │                                  ▼                    │
             │                           ┌──────────┐               │
             │                           │ mapping  │               │
             │                           └────┬─────┘               │
             │                                │  MappingResults      │
             │                           ┌────┴─────┐               │
             │                           │ coverage │               │
             │                           └────┬─────┘               │
             │                                │  Metrics             │
             │                           ┌────┴─────┐               │
             │                           │  report  │               │
             │                           └────┬─────┘               │
             │                                │                      │
             │                           ┌────┴─────┐               │
             │                           │   api    │  (선택적)      │
             │                           └──────────┘               │
             └────────────────────────────────────────────────────┘
```

---

## 3. 패키지 구조

```
mcp-coverage/
├── cmd/
│   └── main.go                  # 진입점, 8단계 파이프라인 오케스트레이션
├── api/
│   └── admin.go                 # HTTP Admin API 서버 (선택적)
├── internal/
│   ├── config/
│   │   └── config.go            # 환경변수 → Config 구조체 로드
│   ├── mcpconfig/
│   │   └── resolver.go          # Claude 설정 파일에서 MCP 서버 설정 탐지
│   ├── mcpclient/
│   │   └── client.go            # MCP stdio 클라이언트 (JSON-RPC 2.0)
│   ├── apiscanner/
│   │   ├── types.go             # APIEntry, Scanner 인터페이스 정의
│   │   ├── openapi.go           # OpenAPI/Swagger 스캐너
│   │   ├── static.go            # Static JSON 스캐너 (metadata/apis.json)
│   │   └── javasource/          # Java 소스 직접 파싱 스캐너
│   │       ├── scanner.go       # 스캐너 진입점, 파일 워킹
│   │       ├── annotation.go    # Spring 어노테이션 파서
│   │       ├── parser.go        # Java 클래스/메서드 파서
│   │       ├── constants.go     # 경로 상수 레지스트리 빌더
│   │       ├── exclusion.go     # 제외 패턴 매처
│   │       ├── actuator.go      # Spring Actuator /mappings 병합
│   │       └── scanner_test.go  # 통합 테스트
│   ├── mapping/
│   │   ├── types.go             # ToolMetadata, MappingResult 타입
│   │   ├── engine.go            # 3-tier 매핑 엔진
│   │   ├── similarity.go        # 토큰 기반 유사도 계산 (Jaccard)
│   │   └── engine_test.go       # 매핑 엔진 테스트
│   ├── coverage/
│   │   ├── calculator.go        # 커버리지 메트릭 계산
│   │   └── calculator_test.go   # 계산기 테스트
│   └── report/
│       ├── filter.go            # 결과 필터링
│       ├── table.go             # ANSI 컬러 터미널 테이블 출력
│       └── json_report.go       # JSON 리포트 구조/파일 저장
└── metadata/                    # (선택) 명시적 매핑 메타데이터
    ├── apis.json                # Static 스캐너용 API 목록
    └── tools_metadata.json      # Tool→API 명시적 매핑
```

---

## 4. 실행 흐름 (8단계)

`cmd/main.go`는 다음 8단계를 순서대로 실행한다.

```
Step 1: config.Load()
        └─ 환경변수 검증 및 Config 구조체 생성
           (TARGET_MCP_NAME 미설정 시 즉시 종료)

Step 2: mcpconfig.Resolve(name)
        └─ Claude 설정 파일들을 순서대로 탐색하여
           MCP 서버 연결 정보(ServerConfig) 반환

Step 3: mcpclient.ListTools()
        └─ MCP 서버 subprocess 시작
        └─ JSON-RPC: initialize → notifications/initialized → tools/list
        └─ []ToolEntry 반환 후 subprocess 종료

Step 4: scanner.Scan()
        └─ 스캐너 우선순위에 따라 선택:
           TARGET_PROJECT_PATH → JavaSource 스캐너
           SWAGGER_URL         → OpenAPI 스캐너
           (없음)              → Static 스캐너
        └─ []APIEntry 반환

Step 5: mapping.NewEngine().Map(apis)
        └─ tools_metadata.json 로드 및 인덱스 빌드
        └─ 3-tier 매핑 수행 (명시적 → 경로 → 유사도)
        └─ []MappingResult 반환

Step 6: coverage.Calculate(results)
        └─ 전체 Metrics 계산
        └─ 모듈별 map[string]*ModuleMetrics 계산
        └─ 컨트롤러별 map[string]*ControllerMetrics 계산

Step 7: report.Filter() + 출력
        └─ FILTER 환경변수 적용
        └─ REPORT_FORMAT=TABLE  → report.PrintTable (stdout)
        └─ REPORT_FORMAT=JSON   → report.WriteJSON (파일)
        └─ REPORT_FORMAT=BOTH   → 둘 다 실행

Step 8: api.New().Run() (ADMIN_HTTP=true 시에만)
        └─ HTTP 서버 블로킹 실행 (기본 포트 8080)
```

---

## 5. 핵심 컴포넌트 상세

### 5.1 config

**파일:** `internal/config/config.go`

환경변수를 읽어 `Config` 구조체를 생성한다. `TARGET_MCP_NAME`만 필수이며 나머지는 모두 기본값을 가진다.

```go
type Config struct {
    TargetMCPName             string   // 분석 대상 MCP 서버 이름 (필수)
    TargetProjectPath         string   // Spring 소스 루트 경로
    SwaggerURL                string   // OpenAPI 스펙 URL
    ActuatorURL               string   // /actuator/mappings 병합용 URL
    ExcludeAPIPatterns        []string // 제외할 API 경로 glob 패턴 (콤마 구분)
    ExcludeControllerPatterns []string // 제외할 컨트롤러 이름 glob 패턴
    ReportFormat              string   // TABLE | JSON | BOTH (기본: BOTH)
    Filter                    string   // ALL | UNMAPPED | ... (기본: ALL)
    OutputDir                 string   // JSON 리포트 저장 디렉토리 (기본: ./reports)
    MetadataDir               string   // metadata/ 디렉토리 경로
    AdminHTTP                 bool     // HTTP Admin API 활성화 여부
    AdminPort                 string   // Admin API 포트 (기본: 8080)
    Debug                     bool     // 디버그 출력 활성화
}
```

**스캐너 선택 로직 (main.go newScanner 함수):**

```
TARGET_PROJECT_PATH 설정됨  →  JavaSource 스캐너 (최우선)
SWAGGER_URL 설정됨          →  OpenAPI 스캐너
둘 다 없음                  →  Static 스캐너 (fallback)
```

---

### 5.2 mcpconfig

**파일:** `internal/mcpconfig/resolver.go`

Claude 설정 파일들을 탐색하여 MCP 서버 연결 정보를 자동으로 찾는다. 사용자가 별도의 연결 설정 없이 `TARGET_MCP_NAME`만 지정하면 된다.

**탐색 순서:**

| 우선순위 | 파일 경로 |
|----------|----------|
| 1 | `~/.claude/settings.json` (Claude Code 전역 설정) |
| 2 | `~/Library/Application Support/Claude/claude_desktop_config.json` (Claude Desktop, macOS) |
| 3 | `~/.claude.json` projects[cwd].mcpServers (프로젝트별 설정) |
| 4 | `./.claude/settings.json` (로컬 프로젝트 설정) |

```go
type ServerConfig struct {
    Type    string            // "stdio" | "sse" (기본: stdio)
    Command string            // 실행할 명령어 (stdio 방식)
    Args    []string          // 명령 인자 목록
    Env     map[string]string // 서버별 환경변수 오버라이드
    URL     string            // SSE 방식 연결 URL
}
```

`-list-servers` 플래그로 발견된 모든 MCP 서버 목록을 출력할 수 있다.

---

### 5.3 mcpclient

**파일:** `internal/mcpclient/client.go`

MCP 서버를 subprocess로 실행하고 stdio를 통한 JSON-RPC 2.0 통신으로 tool 목록을 가져온다.

**통신 시퀀스:**

```
[mcp-coverage]                    [MCP Server subprocess]
      │                                     │
      │─── initialize (JSON-RPC) ─────────▶│
      │◀── initialize result ──────────────│
      │─── notifications/initialized ─────▶│  (응답 없음)
      │─── tools/list ─────────────────────▶│
      │◀── tools/list result ([]ToolEntry)──│
      │    (stdin 닫기 → subprocess 종료)    │
```

**구현 특징:**

| 항목 | 내용 |
|------|------|
| 읽기 버퍼 | `bufio.Scanner` 라인 단위, 4MB 버퍼 |
| 응답 타임아웃 | 15초 |
| Graceful shutdown | stdin 닫기 후 2초 대기, 초과 시 강제 Kill |
| 비JSON 라인 | 서버 부트 메시지 등 자동 스킵 |
| 환경변수 | 부모 환경 + ServerConfig.Env 병합 |

---

### 5.4 apiscanner

**파일:** `internal/apiscanner/types.go`

**Scanner 인터페이스:**

```go
type Scanner interface {
    Scan() ([]APIEntry, error)
    Name() string
}
```

**APIEntry — API 엔드포인트 하나를 표현하는 타입:**

```go
type APIEntry struct {
    Module     string  // 모듈명 (경로 첫 세그먼트 기준)
    Controller string  // 컨트롤러 클래스명
    HTTPMethod string  // GET | POST | PUT | DELETE | PATCH
    APIPath    string  // /api/patients 등
    MethodName string  // Java 메서드명
    Summary    string  // OpenAPI summary (해당되는 경우)
    SourceFile string  // 소스 파일 상대 경로 (JavaSource 스캐너 전용)
    LineNumber int     // 소스 라인 번호 (JavaSource 스캐너 전용)
    ScanStatus string  // "partial" — 경로 상수 미해결 시
    ScanReason string  // ScanStatus 설명 문자열
}
```

**스캐너 종류 비교:**

| 스캐너 | 소스 | 특징 |
|--------|------|------|
| JavaSource | `.java` 파일 직접 파싱 | 가장 완전, Swagger 불필요, 소스 위치 정보 포함 |
| OpenAPI | Swagger/OpenAPI JSON/YAML | 실행 중인 서버 필요 |
| Static | `metadata/apis.json` | 가장 단순, 수동 관리 필요 |

---

### 5.5 mapping

**파일:** `internal/mapping/engine.go`, `types.go`, `similarity.go`

API → MCP Tool 매핑을 수행하는 핵심 엔진. 상세 내용은 [7. 3-Tier 매핑 알고리즘](#7-3-tier-매핑-알고리즘) 참조.

**핵심 타입:**

```go
// tools_metadata.json의 한 항목
type ToolMetadata struct {
    APIPath    string   // 단일 API 매핑: API 경로
    HTTPMethod string   // 단일 API 매핑: HTTP 메서드
    Controller string   // 컨트롤러명 (선택)
    MethodName string   // 메서드명 (선택)
    APIs       []APIRef // 다중 API 매핑: Tool이 여러 API를 호출하는 경우
}

// 매핑 결과 한 행
type MappingResult struct {
    apiscanner.APIEntry             // 원본 API 정보 (임베딩)
    MCPToolName   string            // 매핑된 Tool 이름 ("" → 미매핑)
    MappingStatus string            // mapped | review_required | unmapped
    Remark        string            // 매핑 근거 설명 (예: "explicit metadata")
}
```

---

### 5.6 coverage

**파일:** `internal/coverage/calculator.go`

`[]MappingResult`를 입력받아 세 수준의 메트릭을 동시에 계산한다.

| 수준 | 반환 타입 | 설명 |
|------|----------|------|
| 전체 | `Metrics` | totalApiCount, mappedApiCount, reviewRequiredCount, unmappedApiCount, coverageRate |
| 모듈별 | `map[string]*ModuleMetrics` | 모듈명 → 메트릭 |
| 컨트롤러별 | `map[string]*ControllerMetrics` | 컨트롤러명 → 메트릭 |

**커버리지 계산 공식:**
```
coverageRate = (mapped / total) * 100
```
> `review_required`는 mapped에 포함되지 않음

---

### 5.7 report

**파일:** `internal/report/`

| 파일 | 역할 |
|------|------|
| `filter.go` | FILTER 환경변수에 따른 `[]MappingResult` 필터링 |
| `table.go` | ANSI 컬러 코드를 사용한 터미널 테이블 출력 |
| `json_report.go` | `CoverageReport` 구조체 조립 및 JSON 파일 저장 |

**지원 필터:**

```
ALL                → 전체 결과 (기본값)
UNMAPPED           → 미매핑 API만
REVIEW_REQUIRED    → 검토 필요 항목만
MAPPED             → 매핑 완료 API만
MODULE:<name>      → 특정 모듈만 (대소문자 무관)
CONTROLLER:<name>  → 특정 컨트롤러만 (대소문자 무관)
```

**JSON 리포트 최상위 구조:**

```json
{
  "generatedAt": "2026-04-28T04:24:44Z",
  "targetMcp": "emr-mcp",
  "scannerUsed": "JavaSource",
  "summary": {
    "totalApiCount": 36,
    "mappedApiCount": 16,
    "reviewRequiredCount": 3,
    "unmappedApiCount": 17,
    "coverageRate": 44.44
  },
  "unmappedApis": [...],
  "moduleCoverage": [...],
  "controllerCoverage": [...],
  "results": [...],
  "mcpTools": [...]
}
```

---

### 5.8 api (Admin HTTP)

**파일:** `api/admin.go`

`ADMIN_HTTP=true` 환경변수로 활성화되는 선택적 HTTP API 서버. 스캔 결과를 외부 시스템(CI, 대시보드 등)에서 쿼리할 수 있다.

| 엔드포인트 | 설명 |
|-----------|------|
| `GET /coverage` | 전체 커버리지 메트릭 |
| `GET /coverage/results` | 매핑 결과 목록 (`?status=mapped\|review_required\|unmapped`) |
| `GET /coverage/unmapped` | 미매핑 API 목록 (shortcut) |
| `GET /coverage/modules` | 모듈별 커버리지 메트릭 |
| `GET /coverage/controllers` | 컨트롤러별 커버리지 메트릭 |
| `GET /coverage/report` | 전체 JSON 리포트 |

---

## 6. Java Source Scanner 내부 동작

`TARGET_PROJECT_PATH`가 설정된 경우 사용하는 가장 강력한 스캐너. Swagger 노출 여부나 서버 실행 여부와 무관하게 모든 API를 탐지한다.

### 6.1 동작 단계

```
Phase 1: BuildConstantRegistry(projectPath)
         ├─ 프로젝트 전체 .java 파일 순회 (build/target/node_modules 제외)
         ├─ `final String FIELD = "..."` 선언 추출
         └─ ConstantRegistry 맵 생성
              키 형식: "ClassName.FIELD" (완전 참조) + "FIELD" (단순 참조 fallback)

Phase 2: Java 소스 파일 파싱
         ├─ 멀티 모듈 Gradle/Maven 지원 (중첩 src/main/java 탐색)
         ├─ @RestController / @Controller 클래스 탐지
         │   (완전 정규화명 org.springframework.web.bind.annotation.* 포함)
         ├─ 클래스 레벨 @RequestMapping 경로 추출
         ├─ 메서드 레벨 어노테이션 파싱
         │   (@GetMapping, @PostMapping, @PutMapping, @DeleteMapping, @PatchMapping)
         ├─ 경로 상수 참조 → ConstantRegistry 조회 후 값으로 치환
         │   미해결 시: "UNRESOLVED:<ref>" 값으로 포함 (silent drop 없음)
         └─ 최종 APIEntry 목록 생성

Phase 3: Actuator 병합 (ACTUATOR_URL 설정 시)
         ├─ GET {ACTUATOR_URL}/actuator/mappings 호출
         └─ Phase 2 결과와 병합 (중복 제거, source는 Phase 2 우선)
```

### 6.2 탐지 가능한 Java 패턴

```java
// 기본 형태: 클래스 + 메서드 경로 조합
@RestController
@RequestMapping("/api")
public class PatientController {
    @GetMapping("/patients")
    public List<Patient> list() {}
}
// → GET /api/patients

// 다중 경로 배열
@GetMapping({"/patients", "/members"})
// → GET /api/patients, GET /api/members (각각 별도 항목)

// value/path 속성 형태
@GetMapping(value = "/patients")
@GetMapping(path = "/patients")

// RequestMapping + method 속성
@RequestMapping(path = "/data", method = {RequestMethod.GET, RequestMethod.POST})
// → GET /api/data, POST /api/data (각각 별도 항목)

// 경로 상수 참조
@GetMapping(ApiPaths.PATIENT_BASE)
// → ConstantRegistry에서 "ApiPaths.PATIENT_BASE" 조회 → 실제 경로 치환

// 인터페이스 컨트롤러 (Feign 스타일)
public interface PatientApi {
    @GetMapping("/patients/{id}")
    Patient getById(@PathVariable Long id);
}

// 추상 클래스
public abstract class BaseController {
    @GetMapping("/health")
    public abstract String health();
}
```

### 6.3 제외 패턴 지원

```bash
# API 경로 패턴 제외
EXCLUDE_API_PATTERNS="/actuator/**,/error,/swagger-ui/**"

# 컨트롤러 이름 패턴 제외
EXCLUDE_CONTROLLER_PATTERNS="*HealthCheckController,*InternalController"
```

### 6.4 Debug 통계 출력 (`DEBUG=true`)

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

---

## 7. 3-Tier 매핑 알고리즘

`mapping.Engine`은 각 API에 대해 우선순위 순으로 매핑을 시도하고, 첫 번째로 성공한 Tier의 결과를 사용한다.

### Tier 1: 명시적 메타데이터 매핑 (Priority 1)

`metadata/tools_metadata.json` 파일에서 `"METHOD /path"` 형태로 인덱스를 빌드한 뒤 정확히 일치하는 Tool 이름을 반환한다.

```json
{
  "create_patient": {
    "apiPath": "/reception/insertPatient",
    "httpMethod": "POST"
  },
  "complete_payment": {
    "apis": [
      { "apiPath": "/reception/selectOverallCalculationInfo", "httpMethod": "GET" },
      { "apiPath": "/reception/insertPayData", "httpMethod": "POST" }
    ]
  }
}
```

> Tool 하나가 여러 API를 호출하는 경우 `apis` 배열로 다중 매핑 가능

### Tier 2: 경로+메서드 Fuzzy 매치 (Priority 2)

쿼리 스트링(`?key=value`)을 제거한 후 `METHOD /path` 기준으로 인덱스를 검색한다.

```go
// 쿼리 스트링 제거
func stripQuery(path string) string {
    if idx := strings.Index(path, "?"); idx >= 0 {
        return path[:idx]
    }
    return path
}
```

### Tier 3: 이름 유사도 매칭 (Priority 3)

컨트롤러명 + 메서드명을 토큰화하여 모든 MCP Tool 이름과 **Jaccard 유사도**를 계산한다. 가장 높은 점수의 Tool을 선택.

**토큰화 규칙:**

```
입력: "PatientController insertPatient"
처리:
  1. 구분자 → 공백: _ - / . → 공백
  2. camelCase 분리: "PatientController" → "Patient Controller"
  3. 소문자 변환: "patient controller insert patient"
  4. stop word 제거: "controller" 제거
  5. 결과 토큰: ["patient", "insert", "patient"] → {"patient", "insert"}
```

**stop word 목록:** `controller`, `service`, `handler`, `api`, `the`, `a`, `an`

**유사도 계산 (Jaccard):**

```
shared  = 두 토큰 셋의 교집합 크기
union   = 두 토큰 셋의 합집합 크기
score   = shared / union   (0.0 ~ 1.0)
```

**임계값 및 결과:**

| 유사도 점수 | MappingStatus | 설명 |
|------------|---------------|------|
| ≥ 0.5 | `mapped` | 높은 신뢰도로 매핑 |
| 0.25 ≤ score < 0.5 | `review_required` | 수동 검토 필요 |
| < 0.25 | `unmapped` | 매핑 불가 |

**예시 계산:**

```
API:  "PatientController" + "insertPatient"
      → 토큰: {"patient", "insert"}

Tool: "create_patient"
      → 토큰: {"create", "patient"}

교집합: {"patient"} → shared = 1
합집합: {"patient", "insert", "create"} → union = 3
유사도: 1/3 ≈ 0.33 → review_required
```

---

## 8. 데이터 흐름 다이어그램

```
[환경변수]
    │
    ▼
Config ─────────────────────────┐
    │                           │
    │                    ┌──────┴──────────────────────────┐
    ▼                    ▼                                  ▼
mcpconfig.Resolve    apiscanner.Scanner.Scan()          (ADMIN_HTTP)
    │                    │
    ▼                    │
ServerConfig             │
    │                    │
    ▼                    │
mcpclient.ListTools()    │
    │                    │
    ▼                    ▼
[]ToolEntry ─────► mapping.Engine.Map([]APIEntry)
                         │
               tools_metadata.json
                         │
                         ▼
                  []MappingResult
                         │
                         ├──────────────────────┐
                         ▼                      ▼
               coverage.Calculate()        report.Filter(FILTER)
                         │                      │
               ┌─────────┴───────┐              ▼
               ▼                 ▼         필터된 결과
            Metrics       byModule /         │
                         byController        │
               │               │             │
               └───────┬───────┘             │
                       │◄────────────────────┘
                       │
              ┌────────┴───────────┐
              ▼                    ▼
        PrintTable()           WriteJSON()
          (stdout)          (OUTPUT_DIR/*.json)
```

---

## 9. 핵심 타입 관계도

```
Config
  ├─ 사용: cmd/main.go, internal/mcpclient, internal/apiscanner/javasource
  └─ 생성: internal/config.Load()

ServerConfig (mcpconfig)
  ├─ 사용: mcpclient.Client
  └─ 생성: mcpconfig.Resolve()

ToolEntry (mcpclient)
  ├─ 필드: Name, Description, InputSchema
  ├─ 사용: mapping.Engine (buildIndexes, toolNames 추출)
  └─ 포함: report.CoverageReport.MCPTools

APIEntry (apiscanner)
  ├─ 임베딩: mapping.MappingResult
  └─ 생성: JavaSourceScanner / OpenAPIScanner / StaticScanner

ToolMetadata (mapping)
  ├─ 로드: metadata/tools_metadata.json
  └─ 사용: mapping.Engine.pathMethodIndex 빌드

MappingResult (mapping)
  ├─ 포함: APIEntry (임베딩) + MCPToolName + MappingStatus + Remark
  ├─ 사용: coverage.Calculate(), report.PrintTable(), report.WriteJSON()
  └─ 노출: api.Server (HTTP Admin API)

Metrics / ModuleMetrics / ControllerMetrics (coverage)
  ├─ 사용: report.PrintTable(), report.BuildReport(), api.Server
  └─ 생성: coverage.Calculate()

CoverageReport (report)
  ├─ 포함: Summary + UnmappedAPIs + ModuleCoverage + ControllerCoverage
  │         + Results + MCPTools
  ├─ 저장: JSON 파일 (OUTPUT_DIR/coverage_report_<timestamp>.json)
  └─ 노출: api.Server GET /coverage/report
```

---

## 10. 확장 포인트

### 새 API 스캐너 추가

`apiscanner.Scanner` 인터페이스를 구현한 뒤, `cmd/main.go`의 `newScanner()` 함수에 분기를 추가한다.

```go
// 인터페이스 구현
type MyScanner struct { ... }
func (s *MyScanner) Scan() ([]apiscanner.APIEntry, error) { ... }
func (s *MyScanner) Name() string { return "MyScanner" }

// cmd/main.go newScanner() 함수에 분기 추가
if cfg.MyNewEnvVar != "" {
    return mypackage.NewMyScanner(cfg.MyNewEnvVar)
}
```

### 새 매핑 전략 추가

`internal/mapping/engine.go`의 `mapOne()` 메서드에 새 Tier를 추가한다. 기존 Priority 3 이후 Priority 4로 삽입.

```go
func (e *Engine) mapOne(api apiscanner.APIEntry) MappingResult {
    // ... 기존 Priority 1, 2, 3 ...

    // Priority 4: 커스텀 매핑 로직 추가
    if toolName := e.myCustomMatch(api); toolName != "" {
        return MappingResult{ ..., MappingStatus: StatusMapped }
    }

    return MappingResult{ ..., MappingStatus: StatusUnmapped }
}
```

### 정적 API 수동 등록 (코드 변경 없음)

`metadata/apis.json`에 항목 추가:

```json
[
  {
    "module": "lab",
    "controller": "LabOrderController",
    "httpMethod": "POST",
    "apiPath": "/lab/insertLabOrder",
    "methodName": "insertLabOrder",
    "summary": "Create lab order"
  }
]
```

### Tool-API 명시적 매핑 추가 (코드 변경 없음)

`metadata/tools_metadata.json`에 항목 추가:

```json
{
  "my_tool_name": {
    "apiPath": "/my/api/path",
    "httpMethod": "GET",
    "controllerName": "MyController",
    "methodName": "myMethod"
  }
}
```

---

*분석 기준일: 2026-04-28*
