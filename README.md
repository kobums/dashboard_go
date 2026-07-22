# dashboard_go

대시보드 백엔드 — Go(Fiber v2) + raw SQL. `/api/*` 와 SPA(`dist/`) 정적 서빙을 한 컨테이너에서 담당한다.

## 스택

- Go 1.26 · Fiber v2 · zerolog · `database/sql`(MariaDB, ORM 없음)
- 모델/CRUD는 **gomachine `buildtool-model`** 코드 생성 (라이브 DB 스키마 기반)
- 이미지: `golang:1.26-alpine` 멀티스테이지 → alpine (+tzdata, TZ=Asia/Seoul)

## 디렉토리

```
main.go               진입점 (config → cache → HTTP)
services/http.go      Fiber 셋업 · SPA 정적 서빙 + index.html 폴백
router/router.go      라우트 등록 (인증 구간 구분)
router/routers/       라우터 — *생성*: workout 등 CRUD / *수기*: reading, health, dev, fitnessstats, auth_middleware
controllers/rest/     컨트롤러 — *생성*: CRUD / *수기*: reading, healthingest, dev, fitnessstats
clients/              외부 연동 (수기): snippet, github, gitlab, dev(집계), cache(SWR), backfill
models/               *생성* Manager + db.go(쿼리 빌더)
global/config/        .env.yml 파싱 + 환경변수 오버라이드
cmd/backfill/         개발 컨트리뷰션 과거 전체 백필 (일회성 CLI)
dashboard_go.sql      DDL 원본 (dashboard DB)
.env.yml.docker       이미지에 포함되는 시크릿 없는 설정 (실값은 서버 .env)
```

### ⚠️ 코드 생성기 규칙

- `models/*`, `controllers/rest/{테이블명}.go`, `router/routers/{테이블명}.go` 는 **`buildtool-model` 이 재생성하므로 직접 수정 금지**
- 커스텀 로직은 반드시 별도 파일 + 별도 `Setup*Routes` 함수로
- 재생성: 라이브 DB 스키마 변경 → `~/bin/buildtool-model .` (`config/model.json` 이 접속 정보)
- **라우팅 함정**: 생성 라우터의 `GET /workout/:id` 가 먼저 등록되므로 `/workout/xxx` 형태의 커스텀 GET 을 추가하면 `:id` 에 잡힌다 → 커스텀 통계는 `/api/fitness/*` 처럼 경로를 분리할 것

## API

인증: 별도 표기 없으면 `Authorization: Bearer <DASH_TOKEN>`.

| Method | Path | 설명 |
|---|---|---|
| GET | `/api/ping` | 헬스체크 (인증 없음) |
| POST | `/api/health/ingest` | Health Auto Export 형식 수신 — `api-key: <HEALTH_INGEST_TOKEN>` |
| POST | `/api/health/shortcut` | **iOS 단축어용** 평평한 JSON `{date?, steps, weight, ...}` — `api-key` 헤더. 값 0/누락 스킵, `"72.4 kg"` 같은 문자열도 파싱, (date,name) upsert 멱등 |
| GET | `/api/health/metrics?from=&to=&name=` | 일별 지표 시계열 |
| CRUD | `/api/workout(/:id)` | 운동 기록 (생성 CRUD, `startworkoutdate`/`endworkoutdate` 필터) |
| GET | `/api/fitness/compare?date=` | **같은 요일 비교** — 기준일/−7d/−28d/−364d(52주). 기준일 데이터 없으면 어제 폴백. null=데이터 없음, 0과 구분 |
| GET | `/api/fitness/yearly` | 연도별 운동: 세션·시간·거리·칼로리·타입별·월별 + 연평균 걸음 |
| GET | `/api/reading/summary?year=&month=` | snippetapi 프록시 집계 (10분 캐시) |
| GET | `/api/dev/summary?days=` | 병합 히트맵+통계. `days=0`(기본): **전체 기간**(백필 포함), 1~400: 해당 일수. 60분 캐시 |
| GET | `/api/dev/recent` | GitHub+GitLab 최근 활동 병합 상위 20 (60분 캐시) |
| GET | `/api/dev/yearly` | 연도별 컨트리뷰션 (devstat_tb 로컬 집계, 외부 API 안 씀) |

## DB (dashboard @ 공용 MariaDB)

| 테이블 | 용도 | 멱등 키 |
|---|---|---|
| `workout_tb` (w_*) | 운동 기록. source=`manual`/`apple`, externalid=Apple UUID/`hk-*` | externalid 존재 검사 |
| `healthmetric_tb` (hm_*) | 일별 건강 지표 (steps/weight/...) | UNIQUE(metricdate, name) upsert |
| `devstat_tb` (ds_*) | 소스·일별 컨트리뷰션 (백필 포함 2019~) | UNIQUE(source, statdate) upsert |
| `fetchcache_tb` (fc_*) | 외부 API 응답 캐시 (SWR) | UNIQUE(cachekey) |

## 설정

로컬: `.env.yml` (gitignore). 운영: 이미지의 `.env.yml.docker`(시크릿 없음) + 서버 `/data/dashboard/.env` 환경변수 오버라이드 — 키 목록은 `.env.production.example`.

핵심 키: `DASH_TOKEN`, `HEALTH_INGEST_TOKEN`, `GITHUB_TOKEN`(read:user), `GITLAB_TOKEN`(read_api), `GITLAB_USERNAME`, `SNIPPET_EMAIL/PASSWORD`, `DB_*`

## 개발 · 배포

```bash
make run                 # 로컬 실행 (:8010)
go run ./cmd/backfill    # 개발 컨트리뷰션 과거 전체 백필 (멱등, 재실행 안전)
make push                # SPA 빌드 + docker build + Docker Hub push
```

외부 연동 요약:
- **snippet**: 서비스 계정 로그인 → JWT 메모리 캐시 → 401 시 재로그인 (`clients/snippet.go`)
- **GitHub**: GraphQL `contributionCalendar` 1콜 1년 (제한). 전체 히스토리는 백필이 연도별 반복 호출
- **GitLab**: 공식 events API 일별 버킷팅 (undocumented calendar.json 안 씀). 페이지네이션 시간 상한 있음
- 갱신은 항상 최근 1년만, 캘린더 조립은 devstat_tb 전체 — 과거는 백필 데이터가 소스
