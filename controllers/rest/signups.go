package rest

// 서비스별 신규 가입 현황 — 공용 MariaDB 의 다른 서비스 DB 유저 테이블을 직접 집계한다.
// dashboard DB 계정에는 각 테이블의 "가입일 컬럼 하나"에만 컬럼 단위 SELECT 권한을 줬다
// (이메일·비밀번호 해시는 읽을 수 없음). 서비스 추가 시 signupServices 에 한 줄 + 서버에서
//   GRANT SELECT (<col>) ON <db>.<table> TO 'dashboard'@'172.%';
// 가입일은 전 서비스가 KST naive datetime 으로 저장한다(2026-11 확인: gomachine 계열은 UTC+9 문자열,
// tomelater 는 loc=Local+TZ=Asia/Seoul, Spring 계열은 JVM TZ=Asia/Seoul). 컨테이너 TZ=Asia/Seoul 이라
// time.Now() 경계를 그대로 비교한다. 주는 월요일 시작.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"encoding/json"
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
}

var signupServices = []signupService{
	{"fotstat", "Fotstat", "fotstat.user_tb", "u_createddate"},
	{"snippet", "Snippet", "snippet.user_tb", "u_createdate"},
	{"gym", "Gym", "gym.user_tb", "u_date"},
	{"tomelater", "Tomelater", "tomelater.user_tb", "u_createdat"},
	{"project", "Project", "project.user_tb", "u_date"},
	{"risksync", "RiskSync", "risksync.RS_USER", "CREATED_AT"},
	{"safelog", "Safelog", "safelog.user_tb", "u_date"},
	{"dday", "D-Day", "dday.users_tb", "u_created_at"},
	{"selfmanagement", "Selfmanagement", "selfmanagement.`user`", "created_at"},
}

const signupWeeks = 8

type signupWeek struct {
	Start string `json:"start"` // 월요일 YYYY-MM-DD
	Count int    `json:"count"`
}

type signupStat struct {
	Key           string       `json:"key"`
	Name          string       `json:"name"`
	Today         int          `json:"today"`
	Yesterday     int          `json:"yesterday"`
	ThisWeek      int          `json:"thisWeek"`
	LastWeekSoFar int          `json:"lastWeekSoFar"` // 지난주 월요일 ~ 정확히 7일 전 지금
	LastWeek      int          `json:"lastWeek"`
	Total         int          `json:"total"`
	LastSignup    string       `json:"lastSignup"` // 없으면 ""
	Weekly        []signupWeek `json:"weekly"`     // 오래된 주 → 이번 주, signupWeeks 개
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

type signupSummary struct {
	AsOf      string       `json:"asOf"`
	WeekStart string       `json:"weekStart"`
	Totals    signupTotals `json:"totals"`
	Services  []signupStat `json:"services"`
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

func (c *SignupsController) Summary() {
	buf, err := clients.GetCached("signups_summary", 5*time.Minute, fetchSignupSummary)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("item", json.RawMessage(buf))
}
