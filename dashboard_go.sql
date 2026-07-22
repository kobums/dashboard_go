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
