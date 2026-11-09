package rest

// 서비스 상세 — /api/signups/:key. 레지스트리(signups.go signupServices)의 서비스 하나에 대해
// 일별(30/90일)·주별(26주) 가입·활성, 활동 내역(라벨별 건수·고유 유저), 이번 주 신규/기존 활성,
// 가입 주 코호트 리텐션(12코호트 × 1~8주 뒤)을 낸다.
// 유저·이벤트 행을 받아 Go 에서 집계한다 — 서비스당 수천 행 수준이라 충분하고 SQL 이 단순해진다.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"encoding/json"
	"fmt"
	"time"

	"dashboard/clients"
	"dashboard/global/log"
	"dashboard/models"
)

const (
	detailWeeks   = 26
	cohortCount   = 12
	cohortHorizon = 8
)

type detailDay struct {
	Date    string `json:"date"`
	Signups int    `json:"signups"`
	Active  int    `json:"active"`
}

type detailWeek struct {
	Start   string `json:"start"`
	Signups int    `json:"signups"`
	Active  int    `json:"active"`
}

type detailSource struct {
	Label  string `json:"label"`
	Events int    `json:"events"`
	Users  int    `json:"users"`
}

type detailCohort struct {
	Start  string `json:"start"`
	Size   int    `json:"size"`
	Active []*int `json:"active"` // 1~cohortHorizon 주 뒤 활동한 코호트 인원, 아직 안 온 주는 null
}

type detailActivity struct {
	Today        int            `json:"today"`
	ThisWeek     int            `json:"thisWeek"`
	Window       int            `json:"window"` // days 기간 고유 활성
	Last30       int            `json:"last30"`
	NewThisWeek  int            `json:"newThisWeek"` // 이번 주 활성 중 이번 주 가입
	ReturnWeek   int            `json:"returningThisWeek"`
	LastActive   string         `json:"lastActive"`
	Sources      []detailSource `json:"sources"` // days 기간
	Cohorts      []detailCohort `json:"cohorts"`
	CohortsReady bool           `json:"cohortsReady"` // 유저 id 를 못 읽으면 false
}

type serviceDetail struct {
	Key        string          `json:"key"`
	Name       string          `json:"name"`
	Days       int             `json:"days"`
	AsOf       string          `json:"asOf"`
	WeekStart  string          `json:"weekStart"`
	Status     string          `json:"status"` // active(7일 내 활동/가입) | stale(30일 내) | dormant | none
	Total      int             `json:"total"`
	LastSignup string          `json:"lastSignup"`
	SignupsDay int             `json:"signupsToday"`
	SignupsWk  int             `json:"signupsThisWeek"`
	SignupsWin int             `json:"signupsWindow"`
	Daily      []detailDay     `json:"daily"`
	Weekly     []detailWeek    `json:"weekly"`
	Activity   *detailActivity `json:"activity"` // nil = 활동 기록 없는 서비스
}

type userRow struct {
	id      int64
	created time.Time
}

type eventRow struct {
	label string
	user  int64
	at    time.Time
}

func weekKey(t time.Time) string { return mondayOf(t).Format("2006-01-02") }

func parseKST(s string) (time.Time, bool) {
	t, err := time.ParseInLocation(dtFmt, s, time.Local)
	return t, err == nil
}

func fetchServiceDetail(s *signupService, days int) ([]byte, error) {
	conn := models.NewConnection()
	defer conn.Close()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	week := mondayOf(now)
	firstWeek := week.AddDate(0, 0, -7*(detailWeeks-1))
	firstDay := today.AddDate(0, 0, -(days - 1))

	out := serviceDetail{
		Key: s.Key, Name: s.Name, Days: days, AsOf: now.Format(dtFmt), WeekStart: week.Format("2006-01-02"),
		Daily: make([]detailDay, days), Weekly: make([]detailWeek, detailWeeks),
	}
	dayIdx := map[string]int{}
	for i := range out.Daily {
		out.Daily[i].Date = firstDay.AddDate(0, 0, i).Format("2006-01-02")
		dayIdx[out.Daily[i].Date] = i
	}
	weekIdx := map[string]int{}
	for i := range out.Weekly {
		out.Weekly[i].Start = firstWeek.AddDate(0, 0, 7*i).Format("2006-01-02")
		weekIdx[out.Weekly[i].Start] = i
	}

	// 유저 (id, 가입일) — id 권한이 없는 서비스는 0
	idCol := "0"
	if s.IDColumn != "" {
		idCol = s.IDColumn
	}
	rows, err := conn.Query(fmt.Sprintf("SELECT %s, DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s') FROM %s WHERE %s IS NOT NULL",
		idCol, s.Column, s.Table, s.Column))
	if err != nil {
		return nil, err
	}
	var users []userRow
	signupOf := map[int64]time.Time{}
	for rows.Next() {
		var id int64
		var ts string
		if rows.Scan(&id, &ts) != nil {
			continue
		}
		t, ok := parseKST(ts)
		if !ok || t.After(now) {
			continue
		}
		users = append(users, userRow{id, t})
		if s.IDColumn != "" {
			signupOf[id] = t
		}
	}
	rows.Close()

	var last time.Time
	for _, u := range users {
		out.Total++
		if u.created.After(last) {
			last = u.created
		}
		if !u.created.Before(today) {
			out.SignupsDay++
		}
		if !u.created.Before(week) {
			out.SignupsWk++
		}
		if !u.created.Before(firstDay) {
			out.SignupsWin++
			out.Daily[dayIdx[u.created.Format("2006-01-02")]].Signups++
		}
		if i, ok := weekIdx[weekKey(u.created)]; ok {
			out.Weekly[i].Signups++
		}
	}
	if !last.IsZero() {
		out.LastSignup = last.Format(dtFmt)
	}
	latest := last

	if len(s.Activity) > 0 {
		act, lastActive, err := detailActivityOf(conn, s, now, today, week, firstWeek, firstDay, signupOf, &out, dayIdx, weekIdx)
		if err != nil {
			log.Error().Str("service", s.Key).Str("part", "detail-activity").Msg(err.Error())
		} else {
			out.Activity = act
			if lastActive.After(latest) {
				latest = lastActive
			}
		}
	}

	switch {
	case latest.IsZero():
		out.Status = "none"
	case now.Sub(latest) <= 7*24*time.Hour:
		out.Status = "active"
	case now.Sub(latest) <= 30*24*time.Hour:
		out.Status = "stale"
	default:
		out.Status = "dormant"
	}

	return json.Marshal(out)
}

func detailActivityOf(conn *models.Connection, s *signupService, now, today, week, firstWeek, firstDay time.Time,
	signupOf map[int64]time.Time, out *serviceDetail, dayIdx, weekIdx map[string]int) (*detailActivity, time.Time, error) {

	act := &detailActivity{Sources: []detailSource{}, Cohorts: []detailCohort{}, CohortsReady: s.IDColumn != ""}
	var lastActive time.Time

	var maxT *string
	if err := conn.Conn.QueryRow("SELECT DATE_FORMAT(MAX(t), '%Y-%m-%d %H:%i:%s') FROM "+activityUnion(s.Activity)+
		" WHERE u IS NOT NULL AND u > 0 AND t <= ?", now.Format(dtFmt)).Scan(&maxT); err != nil {
		return nil, lastActive, err
	}
	if maxT != nil {
		act.LastActive = *maxT
		lastActive, _ = parseKST(*maxT)
	}

	// 26주·30/90일·30일 중 가장 이른 시점부터 이벤트를 받는다 (firstWeek 가 항상 가장 이름: 26주 > 90일)
	from := firstWeek
	var events []eventRow
	for _, src := range s.Activity {
		rows, err := conn.Query("SELECT u, DATE_FORMAT(t, '%Y-%m-%d %H:%i:%s') FROM "+activityUnion([]activitySource{src})+
			" WHERE u IS NOT NULL AND u > 0 AND t >= ? AND t <= ?", from.Format(dtFmt), now.Format(dtFmt))
		if err != nil {
			return nil, lastActive, err
		}
		for rows.Next() {
			var u int64
			var ts string
			if rows.Scan(&u, &ts) != nil {
				continue
			}
			if t, ok := parseKST(ts); ok {
				events = append(events, eventRow{src.Label, u, t})
			}
		}
		rows.Close()
	}

	type set = map[int64]bool
	dayUsers := map[string]set{}
	weekUsers := map[string]set{}
	todaySet, weekSet, windowSet, last30Set := set{}, set{}, set{}, set{}
	last30 := today.AddDate(0, 0, -29)

	srcOrder := []string{}
	srcEvents := map[string]int{}
	srcUsers := map[string]set{}
	for _, src := range s.Activity {
		if _, ok := srcUsers[src.Label]; !ok {
			srcOrder = append(srcOrder, src.Label)
			srcUsers[src.Label] = set{}
		}
	}

	for _, e := range events {
		d, w := e.at.Format("2006-01-02"), weekKey(e.at)
		if weekUsers[w] == nil {
			weekUsers[w] = set{}
		}
		weekUsers[w][e.user] = true
		if !e.at.Before(firstDay) {
			if dayUsers[d] == nil {
				dayUsers[d] = set{}
			}
			dayUsers[d][e.user] = true
			windowSet[e.user] = true
			srcEvents[e.label]++
			srcUsers[e.label][e.user] = true
		}
		if !e.at.Before(today) {
			todaySet[e.user] = true
		}
		if !e.at.Before(week) {
			weekSet[e.user] = true
		}
		if !e.at.Before(last30) {
			last30Set[e.user] = true
		}
	}

	for d, us := range dayUsers {
		if i, ok := dayIdx[d]; ok {
			out.Daily[i].Active = len(us)
		}
	}
	for w, us := range weekUsers {
		if i, ok := weekIdx[w]; ok {
			out.Weekly[i].Active = len(us)
		}
	}
	act.Today, act.ThisWeek, act.Window, act.Last30 = len(todaySet), len(weekSet), len(windowSet), len(last30Set)
	for _, label := range srcOrder {
		act.Sources = append(act.Sources, detailSource{Label: label, Events: srcEvents[label], Users: len(srcUsers[label])})
	}

	if !act.CohortsReady {
		return act, lastActive, nil
	}

	for u := range weekSet {
		if t, ok := signupOf[u]; ok && !t.Before(week) {
			act.NewThisWeek++
		} else {
			act.ReturnWeek++
		}
	}

	// 코호트: 가입 주별 유저 → k(1~8)주 뒤 그 주에 활동한 인원. 이번 주(진행 중)까지 채우고 그 뒤는 null.
	members := map[string][]int64{}
	for id, t := range signupOf {
		w := weekKey(t)
		members[w] = append(members[w], id)
	}
	for c := 0; c < cohortCount; c++ {
		start := week.AddDate(0, 0, -7*(cohortCount-1-c))
		key := start.Format("2006-01-02")
		co := detailCohort{Start: key, Size: len(members[key]), Active: make([]*int, cohortHorizon)}
		for k := 1; k <= cohortHorizon; k++ {
			wk := start.AddDate(0, 0, 7*k)
			if wk.After(week) {
				continue
			}
			n := 0
			active := weekUsers[wk.Format("2006-01-02")]
			for _, id := range members[key] {
				if active[id] {
					n++
				}
			}
			co.Active[k-1] = &n
		}
		act.Cohorts = append(act.Cohorts, co)
	}

	return act, lastActive, nil
}

// Detail — days 는 30 또는 90 (그 외는 30).
func (c *SignupsController) Detail(s *signupService, days int) {
	if days != 90 {
		days = 30
	}
	key := fmt.Sprintf("signup_detail_%s_%d", s.Key, days)
	buf, err := clients.GetCached(key, 5*time.Minute, func() ([]byte, error) {
		return fetchServiceDetail(s, days)
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("item", json.RawMessage(buf))
}
