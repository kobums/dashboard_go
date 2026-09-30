-- dashboard DB 스키마 (gomachine 네이밍: {table}_tb, 컬럼 접두어)
-- 적용: 공용 MariaDB(go_mariadb). buildtool-model 이 이 스키마를 읽어 모델을 생성한다.

CREATE DATABASE IF NOT EXISTS dashboard CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 수동/Apple 운동 기록
CREATE TABLE IF NOT EXISTS dashboard.workout_tb (
  w_id         BIGINT NOT NULL AUTO_INCREMENT,
  w_type       VARCHAR(50)  NOT NULL DEFAULT '',      -- 'weight','running','cycling','swimming','etc'
  w_title      VARCHAR(200) NOT NULL DEFAULT '',
  w_workoutdate DATE        NOT NULL,
  w_starttime  DATETIME     NOT NULL DEFAULT '1000-01-01 00:00:00',
  w_duration   INT          NOT NULL DEFAULT 0,       -- seconds
  w_calories   INT          NOT NULL DEFAULT 0,       -- kcal
  w_distance   DOUBLE       NOT NULL DEFAULT 0,       -- km
  w_memo       TEXT         NOT NULL DEFAULT '',
  w_source     VARCHAR(20)  NOT NULL DEFAULT 'manual', -- 'manual' | 'apple'
  w_externalid VARCHAR(100) NOT NULL DEFAULT '',       -- Apple workout UUID, 중복 방지는 ingest 코드에서 SELECT 후 INSERT
  w_createddate DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (w_id),
  KEY idx_workout_date (w_workoutdate),
  KEY idx_workout_external (w_externalid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Health Auto Export 일별 지표 (metric/day 당 1행, upsert 멱등)
CREATE TABLE IF NOT EXISTS dashboard.healthmetric_tb (
  hm_id         BIGINT NOT NULL AUTO_INCREMENT,
  hm_metricdate DATE NOT NULL,
  hm_name       VARCHAR(50) NOT NULL,   -- 'steps','active_energy','exercise_minutes','weight','resting_hr'
  hm_qty        DOUBLE NOT NULL DEFAULT 0,
  hm_unit       VARCHAR(20) NOT NULL DEFAULT '',
  hm_createddate DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (hm_id),
  UNIQUE KEY uk_metric_day (hm_metricdate, hm_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 개발 활동: 소스/일 당 1행 (컨트리뷰션 히트맵 그레인)
CREATE TABLE IF NOT EXISTS dashboard.devstat_tb (
  ds_id       BIGINT NOT NULL AUTO_INCREMENT,
  ds_source   VARCHAR(20) NOT NULL,   -- 'github' | 'gitlab'
  ds_statdate DATE NOT NULL,
  ds_count    INT NOT NULL DEFAULT 0,
  ds_createddate DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (ds_id),
  UNIQUE KEY uk_devstat (ds_source, ds_statdate)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 외부 API 응답 캐시 (stale-while-revalidate)
CREATE TABLE IF NOT EXISTS dashboard.fetchcache_tb (
  fc_id        BIGINT NOT NULL AUTO_INCREMENT,
  fc_cachekey  VARCHAR(100) NOT NULL,  -- 'github_recent','gitlab_recent','reading_summary_2026_7'
  fc_payload   LONGTEXT NOT NULL,
  fc_fetchedat DATETIME NOT NULL,
  fc_createddate DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (fc_id),
  UNIQUE KEY uk_cachekey (fc_cachekey)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── 노션 운동 기록 복제본 (웨이트) ─────────────────────────────────────────
-- 원본은 노션 「운동 캘린더」「운동 일지」 DB. services/liftsync.go 가 주기적으로 전체 재동기화
-- (트랜잭션 안에서 DELETE → INSERT) 하므로 이 테이블들은 직접 수정하지 않는다.
-- buildtool-model 대상 아님 — 생성 CRUD 없이 수기 SQL(clients/liftsync.go, controllers/rest/lift.go)로만 다룬다.

-- 운동 캘린더: 하루 1행
CREATE TABLE IF NOT EXISTS dashboard.liftday_tb (
  ld_id         BIGINT NOT NULL AUTO_INCREMENT,
  ld_notionid   VARCHAR(40)  NOT NULL,
  ld_date       DATE         NOT NULL,
  ld_title      VARCHAR(200) NOT NULL DEFAULT '',
  ld_parts      VARCHAR(200) NOT NULL DEFAULT '',   -- 부위 multi-select, 콤마 구분
  ld_summary    TEXT         NOT NULL DEFAULT '',
  ld_cardio     VARCHAR(500) NOT NULL DEFAULT '',
  ld_condition  VARCHAR(10)  NOT NULL DEFAULT '',   -- 좋음/보통/나쁨
  ld_createddate DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (ld_id),
  UNIQUE KEY uk_liftday_notion (ld_notionid),
  KEY idx_liftday_date (ld_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 운동 일지: 종목 1개 = 1행 (웜업/본세트/목표 원문 보존 — 세트는 liftset_tb 로 파싱)
CREATE TABLE IF NOT EXISTS dashboard.liftexercise_tb (
  le_id         BIGINT NOT NULL AUTO_INCREMENT,
  le_notionid   VARCHAR(40)  NOT NULL,
  le_daynotionid VARCHAR(40) NOT NULL DEFAULT '',   -- 운동일 relation (liftday_tb.ld_notionid)
  le_date       DATE         NOT NULL,
  le_name       VARCHAR(100) NOT NULL,              -- 종목 select (비면 제목)
  le_part       VARCHAR(20)  NOT NULL DEFAULT '',   -- 가슴/등/어깨·팔/하체/유산소
  le_warmup     VARCHAR(500) NOT NULL DEFAULT '',
  le_mainset    VARCHAR(500) NOT NULL DEFAULT '',
  le_target     VARCHAR(200) NOT NULL DEFAULT '',   -- 다음 회차 목표
  le_minutes    DOUBLE       NOT NULL DEFAULT 0,    -- 유산소
  le_speed      DOUBLE       NOT NULL DEFAULT 0,
  le_incline    DOUBLE       NOT NULL DEFAULT 0,
  le_distance   DOUBLE       NOT NULL DEFAULT 0,
  le_condition  VARCHAR(10)  NOT NULL DEFAULT '',
  le_memo       TEXT         NOT NULL DEFAULT '',
  le_parseerror VARCHAR(300) NOT NULL DEFAULT '',   -- 웜업/본세트/목표 형식 오류 (화면에 노출)
  le_createddate DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (le_id),
  UNIQUE KEY uk_liftexercise_notion (le_notionid),
  KEY idx_liftexercise_date (le_date),
  KEY idx_liftexercise_name (le_name, le_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 파싱된 세트: 세트 1개 = 1행
CREATE TABLE IF NOT EXISTS dashboard.liftset_tb (
  ls_id         BIGINT NOT NULL AUTO_INCREMENT,
  ls_exercisenotionid VARCHAR(40) NOT NULL,          -- liftexercise_tb.le_notionid
  ls_date       DATE         NOT NULL,
  ls_order      INT          NOT NULL DEFAULT 0,
  ls_warmup     TINYINT      NOT NULL DEFAULT 0,
  ls_bodyweight TINYINT      NOT NULL DEFAULT 0,     -- BW(맨몸) — 무게 대신 횟수로 비교
  ls_weight     DOUBLE       NOT NULL DEFAULT 0,     -- kg
  ls_reps       INT          NOT NULL DEFAULT 0,
  ls_createddate DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (ls_id),
  KEY idx_liftset_exercise (ls_exercisenotionid),
  KEY idx_liftset_date (ls_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── 노션 식단 일지 복제본 ────────────────────────────────────────────────
-- 원본은 노션 「식단 일지」 DB(음식 1개 = 1행). clients/dietsync.go 가 lift 와 같은 주기로 전체 교체.
-- buildtool-model 대상 아님 — 수기 SQL(clients/dietsync.go, controllers/rest/diet.go)로만 다룬다.
CREATE TABLE IF NOT EXISTS dashboard.diet_tb (
  dt_id         BIGINT NOT NULL AUTO_INCREMENT,
  dt_notionid   VARCHAR(40)  NOT NULL,
  dt_date       DATE         NOT NULL,
  dt_meal       VARCHAR(10)  NOT NULL DEFAULT '',   -- 아침/점심/운동 전/저녁/간식
  dt_food       VARCHAR(200) NOT NULL DEFAULT '',
  dt_grams      DOUBLE       NOT NULL DEFAULT 0,
  dt_kcal       DOUBLE       NOT NULL DEFAULT 0,
  dt_protein    DOUBLE       NOT NULL DEFAULT 0,
  dt_carbs      DOUBLE       NOT NULL DEFAULT 0,
  dt_fat        DOUBLE       NOT NULL DEFAULT 0,
  dt_foodcode   VARCHAR(40)  NOT NULL DEFAULT '',   -- 식약처 식품코드
  dt_memo       TEXT         NOT NULL DEFAULT '',
  dt_createddate DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (dt_id),
  UNIQUE KEY uk_diet_notion (dt_notionid),
  KEY idx_diet_date (dt_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
