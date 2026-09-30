# dashboard_go

대시보드 백엔드 — Go(Fiber v2) + raw SQL. `/api/*` 와 SPA(`dist/`) 정적 서빙을 한 컨테이너에서 담당한다.

## 스택

- Go 1.26 · Fiber v2 · zerolog · `database/sql`(MariaDB, ORM 없음)
- 모델/CRUD는 **gomachine `buildtool-model`** 코드 생성 (라이브 DB 스키마 기반)
- 이미지: `golang:1.26-alpine` 멀티스테이지 → alpine (+tzdata, TZ=Asia/Seoul)

## 디렉토리

```
main.go               진입점 (config → cache → 알림 스케줄러 → HTTP)
services/http.go      Fiber 셋업 · SPA 정적 서빙 + index.html 폴백
services/scheduler.go 알림 스케줄러 — 아침 9시/저녁 8시 판정 후 ntfy·웹푸시 발송 (채널 미설정이면 비활성)
router/router.go      라우트 등록 (인증 구간 구분)
router/routers/       라우터 — *생성*: workout 등 CRUD / *수기*: reading, health, dev, fitnessstats, lift, compare, notify, push, auth_middleware
controllers/rest/     컨트롤러 — *생성*: CRUD / *수기*: reading, healthingest, dev, fitnessstats, lift, compare, metrics(공용 수집기), notify, push
clients/              외부 연동 (수기): snippet, github, gitlab, dev(집계), cache(SWR), backfill, ntfy(발송), webpush(발송+구독 저장)
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
| GET | `/api/compare?date=` | **통합 같은 요일 비교** (운동·독서·개발 공용) — 기준일/전날/−7d/−28d/−364d(52주). 지표: steps·activeEnergy·exerciseMinutes·workout(분·건수)·devCommits·readingMinutes·readingPages. 기준일 데이터 전무하면 어제 폴백. null=데이터 없음, 0과 구분. 응답은 기준일별 5분 메모이즈(stale 반환+백그라운드 갱신) |
| GET | `/api/fitness/yearly` | 연도별 운동: 세션·시간·거리·칼로리·타입별·월별 + 연평균 걸음 |
| GET | `/api/lift/overview` | 웨이트 요약 — 종목별(e1RM·볼륨·증감·최고·다음 목표·달성), 유산소, 최근 12주 부위별 세트/볼륨·4분할, PR, 형식 오류, 체중 180일, 동기화 상태 |
| GET | `/api/lift/exercise?name=` | 종목 회차별 상세 (웜업/본세트 세트, e1RM, 볼륨, 목표→달성, PR) |
| GET | `/api/lift/calendar?month=YYYY-MM` | 월 전체 날짜 — 노션 운동일·종목 + Apple 운동(workout_tb)·걸음·체중 날짜 조인 |
| POST | `/api/lift/sync` | 노션 전체 재동기화 즉시 실행 |
| GET | `/api/lift/form?date=` | 입력 시트 컨텍스트 — 그날 기록, 종목 목록(노션 select 옵션 + 지난 기록·목표), 4분할 추천 부위 |
| POST | `/api/lift/log` | 종목 1개 저장 → **노션에 생성/수정**(`id` 있으면 수정) + 그날 캘린더 페이지 찾기/생성·부위/요약/유산소 재계산 + 동기화. 세트 텍스트는 파서로 검증 |
| POST | `/api/lift/log/delete` | `{id}` 노션 휴지통으로 (마지막 행이면 그날 캘린더 페이지도) + 동기화 |
| POST | `/api/lift/day` | `{date, condition}` 그날 캘린더 컨디션 |
| GET | `/api/diet/day?date=` | 그날 식단 — 운동일/휴식일 판정·목표·합계·끼니별 항목·자주 먹은 음식(60일) |
| GET | `/api/diet/summary?days=` | 일별 섭취·추정 소모(BMR+활동+10%)·적자·체중 + 주간(월요일 시작) 평균·단백질 달성일·체중 변화 |
| GET | `/api/diet/search?q=` | 식약처 식품영양성분DB 검색(1일 메모리 캐시, `FOOD_API_KEY`) — 기준량당 kcal/단백질/탄수/지방 |
| POST | `/api/diet/log` · `/diet/log/delete` · `/diet/copy` | 음식 저장·삭제(노션 휴지통)·다른 날(끼니) 복사 → 노션 먼저 쓰고 동기화 |
| GET/POST | `/api/diet/targets` | 운동일/휴식일 목표 매크로 + BMR (`fetchcache_tb` diet_targets) |
| GET | `/api/reading/summary?year=&month=` | snippetapi 프록시 집계 (10분 캐시) |
| GET | `/api/reading/daily` | 독서 세션 일별 집계 `[{date, minutes, pages, sessions}]` (10분 캐시) — 잔디·일별/주별 차트용 |
| GET | `/api/reading/books` | 완독 책 목록 `[{title, author, coverUrl, rating, endDate}]` (10분 캐시) — 연도별 표지 그리드용 |
| GET | `/api/dev/summary?days=` | 병합 히트맵+통계. `days=0`(기본): **전체 기간**(백필 포함), 1~400: 해당 일수. 60분 캐시 |
| GET | `/api/dev/recent` | GitHub+GitLab 최근 활동 병합 상위 20 (60분 캐시) |
| GET | `/api/dev/yearly` | 연도별 컨트리뷰션 (devstat_tb 로컬 집계, 외부 API 안 씀) |
| GET | `/api/notify/check?mode=` | **알림 판정** — 걸음·운동·커밋·독서(분, 세션 없으면 당일 읽음 여부 폴백)를 전날/전주(같은 요일)/4주 전/1년 전과 비교. `evening`=오늘 경고(부족 시만 notify), `morning`=어제 보고(항상), `auto`=시각 기준. 인증: Bearer **또는** api-key |
| GET | `/api/notify/text?mode=` | 〃 의 **iOS 단축어용 텍스트판** — 알림 불필요면 빈 응답, 필요하면 알림 문장만(plain text). 단축어는 "가져오기→값 있으면→알림 표시" 3액션 |
| GET | `/api/push/status` | 웹푸시 상태 — VAPID 공개키, 채널 구성 여부, 구독 수 |
| POST | `/api/push/subscribe` | 브라우저 PushSubscription 저장 (endpoint 기준 중복 갱신) — `fetchcache_tb` 의 `push_subscriptions` 키에 JSON 배열로 보관 |
| POST | `/api/push/unsubscribe` | `{endpoint}` 로 구독 제거 |
| POST | `/api/push/test` | 구성된 모든 채널(웹푸시+ntfy)로 시험 알림 발송 |

**알림 발송(푸시)**: `services/scheduler.go` 가 아침 9시(어제 보고)/저녁 8시(부족 경고)에 notify 판정을 돌려
ntfy(`clients/ntfy.go`, JSON publish — 한글 헤더 문제 없음)와 웹푸시(`clients/webpush.go`, VAPID)로 보낸다.
만료된 웹푸시 구독(404/410)은 발송 시 자동 제거. 조회형 `/api/notify/*` (홈 배너·iOS 단축어)는 그대로 유지된다.

**웨이트(노션) 동기화**: `services/liftsync.go` 가 기동 직후 + 3시간마다 `clients.SyncLift()` 실행 — 노션 data source 두 개를
전부 읽어(`clients/notion.go`) 트랜잭션 안에서 lift* 테이블을 DELETE→INSERT 로 갈아끼운다(노션 수정·삭제 자동 반영, 노션 호출 실패 시 DB 미변경).
입력 쓰기는 `clients/liftwrite.go` — 노션에 먼저 쓰고 동기화까지 마친 뒤 응답(쓰기는 직렬화). 노션 integration 에 「콘텐츠 업데이트·입력」 권한 필요.
세트 표기(`50×15,11,10 / 60×4,4`, `40×15×5`, `BW×10,10`, 목표 `55×12~15×4`)는 `clients/liftparse.go` 가 파싱하고, 형식 오류는 행에 남겨 화면에 노출한다.

## DB (dashboard @ 공용 MariaDB)

| 테이블 | 용도 | 멱등 키 |
|---|---|---|
| `workout_tb` (w_*) | 운동 기록. source=`manual`/`apple`, externalid=Apple UUID/`hk-*` | externalid 존재 검사 |
| `healthmetric_tb` (hm_*) | 일별 건강 지표 (steps/weight/...) | UNIQUE(metricdate, name) upsert |
| `devstat_tb` (ds_*) | 소스·일별 컨트리뷰션 (백필 포함 2019~) | UNIQUE(source, statdate) upsert |
| `liftday_tb` (ld_*) / `liftexercise_tb` (le_*) / `liftset_tb` (ls_*) | 노션 운동 캘린더(하루)·운동 일지(종목)·파싱된 세트 **복제본** — buildtool-model 대상 아님, 직접 수정 금지 | 동기화마다 전체 교체 |
| `diet_tb` (dt_*) | 노션 「식단 일지」 복제본 — buildtool-model 대상 아님 | 동기화마다 전체 교체 |
| `fetchcache_tb` (fc_*) | 외부 API 응답 캐시 (SWR) — 앞단에 프로세스 인메모리 캐시가 있어 TTL 안 반복 조회는 DB 왕복 없음 | UNIQUE(cachekey) |

## 설정

로컬: `.env.yml` (gitignore). 운영: 이미지의 `.env.yml.docker`(시크릿 없음) + 서버 `/data/dashboard/.env` 환경변수 오버라이드 — 키 목록은 `.env.production.example`.

핵심 키: `DASH_TOKEN`, `HEALTH_INGEST_TOKEN`, `GITHUB_TOKEN`(read:user), `GITLAB_TOKEN`(read_api), `GITLAB_USERNAME`, `SNIPPET_EMAIL/PASSWORD`, `DB_*`

알림 푸시 키(둘 다 없으면 스케줄러 비활성):
- `NOTION_TOKEN` — 노션 내부 integration 시크릿 (「운동 계획 — 4분할」 페이지에 연결 필요). 비우면 웨이트 동기화 비활성. `NOTION_LOG_DS`/`NOTION_DAY_DS` 로 data source id 변경 가능(기본값 내장)
- `FOOD_API_KEY` — 공공데이터포털 「식품의약품안전처_식품영양성분DB정보」 일반 인증키(Decoding). 비우면 식단 음식 검색만 비활성(자주 먹은 음식·직접 입력은 동작). `NOTION_DIET_DS` 로 식단 data source 변경 가능. `FOOD_API_URL` — 요청주소(기본 `…/FoodNtrCpntDbInfo03/getFoodNtrCpntDbInq03`), 서비스 버전이 바뀌면 활용신청 상세의 요청주소로
- `NTFY_TOPIC` — 공개 ntfy.sh 는 토픽 이름이 곧 비밀번호, 추측 불가능하게. `NTFY_SERVER`(기본 https://ntfy.sh)
- `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` — 웹푸시. 생성: `webpush.GenerateVAPIDKeys()` (SherClockHolmes/webpush-go)

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
