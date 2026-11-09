package rest

// 서비스별 신규 가입 현황 — 공용 MariaDB 의 다른 서비스 DB 유저 테이블을 직접 집계한다.
// dashboard DB 계정에는 각 테이블의 "가입일 컬럼 하나"에만 컬럼 단위 SELECT 권한을 줬다
// (이메일·비밀번호 해시는 읽을 수 없음). 서비스 추가 시 signupServices 에 한 줄 + 서버에서
//   GRANT SELECT (<col>) ON <db>.<table> TO 'dashboard'@'172.%';
// 가입일은 전 서비스가 KST naive datetime 으로 저장한다(2026-11 확인: gomachine 계열은 UTC+9 문자열,
// tomelater 는 loc=Local+TZ=Asia/Seoul, Spring 계열은 JVM TZ=Asia/Seoul). 컨테이너 TZ=Asia/Seoul 이라
// time.Now() 경계를 그대로 비교한다. 주는 월요일 시작.
// 활성 유저 = 기간 안에 활동 흔적(로그인 토큰·콘텐츠 생성/수정)이 있는 고유 유저 — 소스는 레지스트리의 Activity.
// 활동 소스도 같은 방식으로 필요한 컬럼(유저 id·시각·조인 키)만 GRANT 한다.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"encoding/json"
	"strings"
	"time"

	"dashboard/clients"
	"dashboard/controllers"
	"dashboard/global/log"
	"dashboard/models"
)

type SignupsController struct {
	controllers.Controller
}

type signupService struct {
	Key    string
	Name   string
	Table  string // db.table — 코드 상수만 (사용자 입력 결합 금지)
	Column string
	// Activity — 활동 흔적 SELECT 들. 각각 (u = 유저 id, t = KST datetime) 두 컬럼을 낸다.
	// UTC 로 저장된 컬럼은 DATE_ADD(.., INTERVAL 9 HOUR) 로 KST 로 맞춘다. 비어 있으면 활동 집계 없음.
	// 이력(행)이 쌓이는 것만 넣는다 — "마지막 접속" 처럼 덮어쓰는 값은 지난주·주별 추이를 왜곡하므로 제외.
	Activity []string
}

var signupServices = []signupService{
	{Key: "fotstat", Name: "Fotstat", Table: "fotstat.user_tb", Column: "u_createddate", Activity: []string{
		// 리프레시 토큰은 로그인 때만 생성(비회전), DB 기본값이라 UTC
		"SELECT rt_user, DATE_ADD(rt_createddate, INTERVAL 9 HOUR) FROM fotstat.refresh_token_tb",
		// 경기·선수·기록은 팀 소유자 활동
		"SELECT tm.t_user, m.m_createddate FROM fotstat.match_tb m JOIN fotstat.team_tb tm ON tm.t_id = m.m_team",
		"SELECT tm.t_user, p.p_createddate FROM fotstat.player_tb p JOIN fotstat.team_tb tm ON tm.t_id = p.p_team",
		"SELECT tm.t_user, r.r_createddate FROM fotstat.record_tb r JOIN fotstat.quarter_tb q ON q.q_id = r.r_quarter " +
			"JOIN fotstat.match_tb m ON m.m_id = q.q_match JOIN fotstat.team_tb tm ON tm.t_id = m.m_team",
	}},
	{Key: "snippet", Name: "Snippet", Table: "snippet.user_tb", Column: "u_createdate", Activity: []string{
		"SELECT rt_user_id, rt_createdate FROM snippet.refreshtoken_tb", // 토큰 갱신마다 새 행
		"SELECT ub_user, ub_createdate FROM snippet.userbook_tb",
		"SELECT ub_user, ub_updatedate FROM snippet.userbook_tb",
		"SELECT rs_user, rs_createdate FROM snippet.readingsession_tb",
		"SELECT rl_user, rl_createdate FROM snippet.readinglog_tb",
		"SELECT r_user, r_createdate FROM snippet.record_tb",
		"SELECT us_user, us_createdate FROM snippet.usersnippet_tb",
		"SELECT sdv_user, sdv_date FROM snippet.snippet_daily_view_tb",
		"SELECT ss_user, ss_seendate FROM snippet.snippet_seen_tb",
		"SELECT s_user, s_createdate FROM snippet.suggestion_tb",
	}},
	{Key: "gym", Name: "Gym", Table: "gym.user_tb", Column: "u_date", Activity: []string{
		"SELECT at_user, at_date FROM gym.attendance_tb",
		"SELECT wl_user, wl_date FROM gym.workoutlog_tb",
		"SELECT pr_member, pr_createddate FROM gym.ptreservation_tb",
	}},
	{Key: "tomelater", Name: "Tomelater", Table: "tomelater.user_tb", Column: "u_createdat", Activity: []string{
		"SELECT a_uid, a_createdat FROM tomelater.answer_tb",
		"SELECT a_uid, a_updatedat FROM tomelater.answer_tb",
	}},
	{Key: "project", Name: "Project", Table: "project.user_tb", Column: "u_date", Activity: []string{
		"SELECT b_user, b_date FROM project.board_tb",
	}},
	{Key: "risksync", Name: "RiskSync", Table: "risksync.RS_USER", Column: "CREATED_AT"}, // 유저별 활동 기록 없음
	{Key: "safelog", Name: "Safelog", Table: "safelog.user_tb", Column: "u_date", Activity: []string{
		"SELECT r_user, r_date FROM safelog.report_tb",
	}},
	{Key: "dday", Name: "D-Day", Table: "dday.users_tb", Column: "u_created_at"}, // ddays_tb 에 유저 컬럼 없음
	{Key: "selfmanagement", Name: "Selfmanagement", Table: "selfmanagement.`user`", Column: "created_at", Activity: []string{
		// Prisma — UTC
		"SELECT user_id, DATE_ADD(created_at, INTERVAL 9 HOUR) FROM selfmanagement.routine",
		"SELECT r.user_id, DATE_ADD(i.created_at, INTERVAL 9 HOUR) FROM selfmanagement.routine_instance i " +
			"JOIN selfmanagement.routine r ON r.routine_id = i.routine_id",
	}},
}

const signupWeeks = 8

type signupWeek struct {
	Start string `json:"start"` // 월요일 YYYY-MM-DD
	Count int    `json:"count"`
}

type signupStat struct {
	Key           string        `json:"key"`
	Name          string        `json:"name"`
	Today         int           `json:"today"`
	Yesterday     int           `json:"yesterday"`
	ThisWeek      int           `json:"thisWeek"`
	LastWeekSoFar int           `json:"lastWeekSoFar"` // 지난주 월요일 ~ 정확히 7일 전 지금
	LastWeek      int           `json:"lastWeek"`
	Total         int           `json:"total"`
	LastSignup    string        `json:"lastSignup"` // 없으면 ""
	Weekly        []signupWeek  `json:"weekly"`     // 오래된 주 → 이번 주, signupWeeks 개
	Error         string        `json:"error,omitempty"`
	Activity      *activityStat `json:"activity"` // nil = 활동 기록 없는 서비스
}

// activityStat — 기간 안에 활동 흔적이 하나라도 있는 고유 유저 수.
type activityStat struct {
	Today         int          `json:"today"`
	ThisWeek      int          `json:"thisWeek"`
	LastWeekSoFar int          `json:"lastWeekSoFar"`
	LastWeek      int          `json:"lastWeek"`
	Last30        int          `json:"last30"`
	LastActive    string       `json:"lastActive"`
	Weekly        []signupWeek `json:"weekly"` // 주별 활성 유저
	Error         string       `json:"error,omitempty"`
}

type signupTotals struct {
	Today         int `json:"today"`
	Yesterday     int `json:"yesterday"`
	ThisWeek      int `json:"thisWeek"`
	LastWeekSoFar int `json:"lastWeekSoFar"`
	LastWeek      int `json:"lastWeek"`
	Total         int `json:"total"`
}

// activityTotals — 서비스별 활성 유저의 단순 합 (서비스 간 같은 사람은 구분 불가).
type activityTotals struct {
	Today         int `json:"today"`
	ThisWeek      int `json:"thisWeek"`
	LastWeekSoFar int `json:"lastWeekSoFar"`
	LastWeek      int `json:"lastWeek"`
	Last30        int `json:"last30"`
}

type signupSummary struct {
	AsOf           string         `json:"asOf"`
	WeekStart      string         `json:"weekStart"`
	Totals         signupTotals   `json:"totals"`
	ActivityTotals activityTotals `json:"activityTotals"`
	Services       []signupStat   `json:"services"`
}

const dtFmt = "2006-01-02 15:04:05"

func fetchSignupSummary() ([]byte, error) {
	conn := models.NewConnection()
	defer conn.Close()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	week := mondayOf(now)
	lastWeek := week.AddDate(0, 0, -7)
	lastWeekNow := now.AddDate(0, 0, -7)
	firstWeek := week.AddDate(0, 0, -7*(signupWeeks-1))

	out := signupSummary{AsOf: now.Format(dtFmt), WeekStart: week.Format("2006-01-02"), Services: []signupStat{}}

	for _, s := range signupServices {
		st := signupStat{Key: s.Key, Name: s.Name, Weekly: make([]signupWeek, signupWeeks)}
		for i := range st.Weekly {
			st.Weekly[i].Start = firstWeek.AddDate(0, 0, 7*i).Format("2006-01-02")
		}

		col := s.Column
		var last *string
		err := conn.Conn.QueryRow(
			"SELECT COUNT("+col+"), "+
				"COALESCE(SUM("+col+" >= ?), 0), COALESCE(SUM("+col+" >= ? AND "+col+" < ?), 0), "+
				"COALESCE(SUM("+col+" >= ?), 0), COALESCE(SUM("+col+" >= ? AND "+col+" < ?), 0), "+
				"COALESCE(SUM("+col+" >= ? AND "+col+" < ?), 0), "+
				"DATE_FORMAT(MAX("+col+"), '%Y-%m-%d %H:%i:%s') FROM "+s.Table,
			today.Format(dtFmt), yesterday.Format(dtFmt), today.Format(dtFmt),
			week.Format(dtFmt), lastWeek.Format(dtFmt), lastWeekNow.Format(dtFmt),
			lastWeek.Format(dtFmt), week.Format(dtFmt),
		).Scan(&st.Total, &st.Today, &st.Yesterday, &st.ThisWeek, &st.LastWeekSoFar, &st.LastWeek, &last)
		if err != nil {
			// 권한 누락·테이블 변경 등 — 서비스 하나 실패가 전체를 막지 않게 표시만 한다.
			log.Error().Str("service", s.Key).Msg(err.Error())
			st.Error = "집계 실패"
			out.Services = append(out.Services, st)
			continue
		}
		if last != nil {
			st.LastSignup = *last
		}

		rows, err := conn.Query(
			"SELECT DATE_FORMAT(DATE_SUB(DATE("+col+"), INTERVAL WEEKDAY("+col+") DAY), '%Y-%m-%d'), COUNT("+col+") "+
				"FROM "+s.Table+" WHERE "+col+" >= ? GROUP BY 1",
			firstWeek.Format(dtFmt),
		)
		if err == nil {
			idx := map[string]int{}
			for i, w := range st.Weekly {
				idx[w.Start] = i
			}
			for rows.Next() {
				var start string
				var n int
				if rows.Scan(&start, &n) == nil {
					if i, ok := idx[start]; ok {
						st.Weekly[i].Count = n
					}
				}
			}
			rows.Close()
		} else {
			log.Error().Str("service", s.Key).Msg(err.Error())
		}

		if len(s.Activity) > 0 {
			st.Activity = fetchActivity(conn, s, now)
			if st.Activity.Error == "" {
				out.ActivityTotals.Today += st.Activity.Today
				out.ActivityTotals.ThisWeek += st.Activity.ThisWeek
				out.ActivityTotals.LastWeekSoFar += st.Activity.LastWeekSoFar
				out.ActivityTotals.LastWeek += st.Activity.LastWeek
				out.ActivityTotals.Last30 += st.Activity.Last30
			}
		}

		out.Totals.Today += st.Today
		out.Totals.Yesterday += st.Yesterday
		out.Totals.ThisWeek += st.ThisWeek
		out.Totals.LastWeekSoFar += st.LastWeekSoFar
		out.Totals.LastWeek += st.LastWeek
		out.Totals.Total += st.Total
		out.Services = append(out.Services, st)
	}

	return json.Marshal(out)
}

// fetchActivity — 활동 소스들을 UNION ALL 로 합쳐 기간별 고유 유저를 센다. 테이블이 작아(수천 행) 인덱스 없이 충분.
func fetchActivity(conn *models.Connection, s signupService, now time.Time) *activityStat {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	week := mondayOf(now)
	lastWeek := week.AddDate(0, 0, -7)
	firstWeek := week.AddDate(0, 0, -7*(signupWeeks-1))

	st := &activityStat{Weekly: make([]signupWeek, signupWeeks)}
	idx := map[string]int{}
	for i := range st.Weekly {
		st.Weekly[i].Start = firstWeek.AddDate(0, 0, 7*i).Format("2006-01-02")
		idx[st.Weekly[i].Start] = i
	}

	// MariaDB 10.11 은 파생 테이블 컬럼 목록 "a (u, t)" 미지원 — 빈 첫 SELECT 로 컬럼 이름을 정한다.
	union := "(SELECT 0 AS u, CAST(NULL AS DATETIME) AS t FROM DUAL WHERE 1 = 0 UNION ALL " +
		strings.Join(s.Activity, " UNION ALL ") + ") a"
	var last *string
	err := conn.Conn.QueryRow(
		"SELECT COUNT(DISTINCT CASE WHEN t >= ? THEN u END), COUNT(DISTINCT CASE WHEN t >= ? THEN u END), "+
			"COUNT(DISTINCT CASE WHEN t >= ? AND t < ? THEN u END), COUNT(DISTINCT CASE WHEN t >= ? AND t < ? THEN u END), "+
			"COUNT(DISTINCT CASE WHEN t >= ? THEN u END), DATE_FORMAT(MAX(t), '%Y-%m-%d %H:%i:%s') "+
			"FROM "+union+" WHERE u IS NOT NULL AND u > 0 AND t <= ?",
		today.Format(dtFmt), week.Format(dtFmt),
		lastWeek.Format(dtFmt), now.AddDate(0, 0, -7).Format(dtFmt), lastWeek.Format(dtFmt), week.Format(dtFmt),
		today.AddDate(0, 0, -29).Format(dtFmt), now.Format(dtFmt),
	).Scan(&st.Today, &st.ThisWeek, &st.LastWeekSoFar, &st.LastWeek, &st.Last30, &last)
	if err != nil {
		log.Error().Str("service", s.Key).Str("part", "activity").Msg(err.Error())
		return &activityStat{Error: "집계 실패", Weekly: st.Weekly}
	}
	if last != nil {
		st.LastActive = *last
	}

	rows, err := conn.Query(
		"SELECT DATE_FORMAT(DATE_SUB(DATE(t), INTERVAL WEEKDAY(t) DAY), '%Y-%m-%d'), COUNT(DISTINCT u) "+
			"FROM "+union+" WHERE u IS NOT NULL AND u > 0 AND t >= ? AND t <= ? GROUP BY 1",
		firstWeek.Format(dtFmt), now.Format(dtFmt),
	)
	if err != nil {
		log.Error().Str("service", s.Key).Str("part", "activity-weekly").Msg(err.Error())
		return st
	}
	defer rows.Close()
	for rows.Next() {
		var start string
		var n int
		if rows.Scan(&start, &n) == nil {
			if i, ok := idx[start]; ok {
				st.Weekly[i].Count = n
			}
		}
	}
	return st
}

func (c *SignupsController) Summary() {
	buf, err := clients.GetCached("signups_summary_v2", // v2: activity 추가
		5*time.Minute, fetchSignupSummary)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("item", json.RawMessage(buf))
}
