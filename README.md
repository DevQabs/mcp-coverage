# mcp-coverage

MCP API Coverage Tracker — Spring Controller API가 MCP Tool로 얼마나 커버되는지 측정하는 Go 도구.

## 개요

Spring 백엔드에 새 API가 추가될 때 MCP Tool이 없으면 AI 에이전트가 해당 API에 접근할 수 없다. 이 도구는 전체 Spring API 목록과 MCP Tool 목록을 비교해 커버리지를 측정하고, 미매핑 API를 즉시 식별한다.

- **API 수집**: Swagger/OpenAPI 또는 정적 레지스트리(`metadata/apis.json`)
- **MCP Tool 수집**: 대상 MCP 서버에 직접 연결해 `tools/list` 호출
- **3단계 매핑**: 명시적 메타데이터 → 경로 매칭 → 이름 유사도
- **리포트**: 터미널 컬러 테이블 + JSON 파일

## 설치

```bash
git clone https://github.com/DevQabs/mcp-coverage
cd mcp-coverage
go build -o bin/mcp-coverage ./cmd/
```

**요구사항:** Go 1.25+

## 실행

`TARGET_MCP_NAME` 하나만 필수. 연결 정보(command, env 등)는 Claude 설정 파일에서 자동 조회.

```bash
TARGET_MCP_NAME=emr-mcp ./bin/mcp-coverage
```

### 주요 옵션

```bash
# 미매핑 API만 보기
TARGET_MCP_NAME=emr-mcp FILTER=UNMAPPED ./bin/mcp-coverage

# 검토 필요 항목만
TARGET_MCP_NAME=emr-mcp FILTER=REVIEW_REQUIRED ./bin/mcp-coverage

# 특정 모듈만
TARGET_MCP_NAME=emr-mcp FILTER=MODULE:reception ./bin/mcp-coverage

# Swagger로 실시간 Spring API 수집
TARGET_MCP_NAME=emr-mcp SWAGGER_URL=http://localhost:8080 ./bin/mcp-coverage

# JSON 리포트만 생성 (CI용)
TARGET_MCP_NAME=emr-mcp REPORT_FORMAT=JSON OUTPUT_DIR=./reports ./bin/mcp-coverage

# HTTP 관리 API 활성화
TARGET_MCP_NAME=emr-mcp ADMIN_HTTP=true ADMIN_PORT=8080 ./bin/mcp-coverage

# 사용 가능한 MCP 서버 목록 확인
./bin/mcp-coverage -list-servers
```

### 환경변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `TARGET_MCP_NAME` | **필수** | 분석할 MCP 서버 이름 |
| `SWAGGER_URL` | — | Spring 서버 베이스 URL (OpenAPI 스캔) |
| `REPORT_FORMAT` | `BOTH` | `TABLE` / `JSON` / `BOTH` |
| `FILTER` | `ALL` | `UNMAPPED` / `REVIEW_REQUIRED` / `MAPPED` / `MODULE:<name>` / `CONTROLLER:<name>` |
| `ADMIN_HTTP` | `false` | HTTP 관리 API 활성화 |
| `ADMIN_PORT` | `8080` | 관리 API 포트 |
| `METADATA_DIR` | `./metadata` | `apis.json`, `tools_metadata.json` 경로 |
| `OUTPUT_DIR` | `./reports` | JSON 리포트 출력 경로 |

## 출력 예시

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

### JSON 리포트 구조

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

## HTTP 관리 API

`ADMIN_HTTP=true` 설정 시:

| 엔드포인트 | 설명 |
|-----------|------|
| `GET /coverage` | 전체 커버리지 지표 |
| `GET /coverage/results?status=unmapped` | 상태별 필터 (`mapped` / `review_required` / `unmapped`) |
| `GET /coverage/unmapped` | 미매핑 API 목록 |
| `GET /coverage/modules` | 모듈별 커버리지 |
| `GET /coverage/controllers` | 컨트롤러별 커버리지 |
| `GET /coverage/report` | 전체 JSON 리포트 |

## 메타데이터 관리

### 새 API 추가

`metadata/apis.json`에 추가 → 자동으로 `unmapped` 표시:

```json
{
  "module": "lab",
  "controller": "LabOrderController",
  "httpMethod": "POST",
  "apiPath": "/lab/insertLabOrder",
  "methodName": "insertLabOrder",
  "summary": "검사 오더 등록"
}
```

### MCP Tool 매핑 추가

`metadata/tools_metadata.json`에 추가 (기존 코드 변경 없음):

```json
"create_lab_order": {
  "apiPath": "/lab/insertLabOrder",
  "httpMethod": "POST",
  "controllerName": "LabOrderController",
  "methodName": "insertLabOrder"
}
```

여러 API를 호출하는 Tool:

```json
"complete_payment": {
  "apis": [
    { "apiPath": "/reception/selectOverallCalculationInfo", "httpMethod": "GET", "note": "계산 조회" },
    { "apiPath": "/reception/insertPayData", "httpMethod": "POST", "note": "수납 처리" }
  ]
}
```

## 매핑 우선순위

| 우선순위 | 방식 | 결과 상태 |
|----------|------|----------|
| 1 | `tools_metadata.json` 명시적 매핑 | `mapped` |
| 2 | `apiPath` + `httpMethod` 경로 매칭 | `mapped` |
| 3 | 컨트롤러/메서드명 유사도 ≥ 0.5 | `mapped` |
| 3 | 컨트롤러/메서드명 유사도 ≥ 0.25 | `review_required` |
| — | 매칭 없음 | `unmapped` |

명시적 메타데이터가 항상 최우선. 유사도 매칭은 보조 수단이며 `review_required` 항목은 수동 확인 필요.

## 테스트

```bash
go test ./...
```

## 규칙

- 기존 API 비즈니스 로직 변경 금지
- MCP Tool 메타데이터 확장만 허용
- `tools_metadata.json` 명시적 매핑이 최우선
- 새 API는 MCP Tool 없으면 자동으로 `unmapped`
