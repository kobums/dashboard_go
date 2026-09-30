package rest

// 웨이트(노션 운동 기록) 조회 API — /api/lift/*.
// 원본은 노션, liftday/liftexercise/liftset_tb 는 clients/liftsync.go 가 채우는 복제본.
// 행 수가 적어서(하루 10행 안팎) 전부 메모리에 올려 Go 에서 집계한다.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.
//
// 지표 정의
//   e1RM   : Epley — weight × (1 + reps/30), 1회는 무게 그대로. 본세트 중 최댓값
//   볼륨   : Σ weight × reps (본세트만). 맨몸(BW) 종목은 무게 대신 횟수로 비교한다
//   목표   : 각 회차에 적은 「목표」 = 다음 회차 목표. 다음 회차가 그 세트들을 몇 개 채웠는지가 달성률
//   PR     : 이전 모든 회차보다 e1RM(BW는 최다 횟수)이 높을 때. 첫 회차는 PR 아님

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"dashboard/clients"
	"dashboard/controllers"
	"dashboard/global/log"
	"dashboard/models"
)

type LiftController struct {
	controllers.Controller
}

// 4분할 기준 부위 — 주간 체크리스트와 부위별 집계 키
var liftParts = []string{"가슴", "등", "어깨·팔", "하체"}

const cardioPart = "유산소"

type liftSession struct {
	NotionId    string
	DayNotionId string
	Date        string
	Name        string
	Part        string
	WarmupText  string
	MainText    string
	Target      string
	Minutes     float64
	Speed       float64
	Incline     float64
	Distance    float64
	Condition   string
	Memo        string
	ParseError  string
	Warmup      []clients.LiftSet
	Main        []clients.LiftSet
}

type liftSessionStats struct {
	E1rm       float64
	TopWeight  float64
	TopReps    int
	Volume     float64
	TotalReps  int
	Bodyweight bool
}

// metric 은 PR·증감 비교용 단일 지표 (BW 종목은 최다 횟수)
func (s liftSessionStats) metric() float64 {
	if s.Bodyweight {
		return float64(s.TopReps)
	}
	return s.E1rm
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func notionUrl(id string) string {
	return "https://www.notion.so/" + strings.ReplaceAll(id, "-", "")
}

func epley(w float64, reps int) float64 {
	if reps <= 1 {
		return w
	}
	return w * (1 + float64(reps)/30)
}

func computeLiftStats(main []clients.LiftSet) liftSessionStats {
	st := liftSessionStats{Bodyweight: len(main) > 0}
	for _, s := range main {
		if !s.Bodyweight {
			st.Bodyweight = false
		}
	}
	for _, s := range main {
		st.TotalReps += s.Reps
		if st.Bodyweight {
			if s.Reps > st.TopReps {
				st.TopReps = s.Reps
			}
			continue
		}
		st.Volume += s.Weight * float64(s.Reps)
		if e := epley(s.Weight, s.Reps); e > st.E1rm {
			st.E1rm = e
		}
		if s.Weight > st.TopWeight || (s.Weight == st.TopWeight && s.Reps > st.TopReps) {
			st.TopWeight, st.TopReps = s.Weight, s.Reps
		}
	}
	if st.Bodyweight {
		st.Volume = float64(st.TotalReps)
	}
	st.E1rm = round1(st.E1rm)
	st.Volume = round1(st.Volume)
	return st
}

// achievement 는 목표 세트 각각을 채운 본세트가 있는지 1:1 로 맞춰 본다.
// 목표 50×12×4 → 50kg 이상·12회 이상 세트가 몇 개인지 (최대 4).
func achievement(goal string, main []clients.LiftSet) map[string]int {
	goalSets, err := clients.ParseSets(goal)
	if err != nil || len(goalSets) == 0 {
		return nil
	}
	used := make([]bool, len(main))
	hit := 0
	for _, g := range goalSets {
		for i, s := range main {
			if used[i] {
				continue
			}
			weightOk := g.Bodyweight || s.Weight >= g.Weight
			if weightOk && s.Reps >= g.Reps {
				used[i] = true
				hit++
				break
			}
		}
	}
	return map[string]int{"hit": hit, "total": len(goalSets)}
}

// loadLiftSessions 는 종목 기록 전부(또는 조건)를 날짜순으로 읽고 세트를 붙인다.
func loadLiftSessions(conn *models.Connection, where string, args ...interface{}) ([]liftSession, error) {
	rows, err := conn.Query(
		"SELECT le_notionid, le_daynotionid, le_date, le_name, le_part, le_warmup, le_mainset, le_target, "+
			"le_minutes, le_speed, le_incline, le_distance, le_condition, le_memo, le_parseerror "+
			"FROM liftexercise_tb "+where+" ORDER BY le_date, le_id", args...)
	if err != nil {
		return nil, err
	}
	var list []liftSession
	index := map[string]int{}
	for rows.Next() {
		var s liftSession
		if err := rows.Scan(&s.NotionId, &s.DayNotionId, &s.Date, &s.Name, &s.Part, &s.WarmupText, &s.MainText, &s.Target,
			&s.Minutes, &s.Speed, &s.Incline, &s.Distance, &s.Condition, &s.Memo, &s.ParseError); err != nil {
			rows.Close()
			return nil, err
		}
		s.Date = s.Date[:10]
		index[s.NotionId] = len(list)
		list = append(list, s)
	}
	rows.Close()
	if len(list) == 0 {
		return list, nil
	}

	srows, err := conn.Query(
		"SELECT ls_exercisenotionid, ls_warmup, ls_bodyweight, ls_weight, ls_reps FROM liftset_tb " +
			"ORDER BY ls_exercisenotionid, ls_order")
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var id string
		var warm, bw bool
		var set clients.LiftSet
		if err := srows.Scan(&id, &warm, &bw, &set.Weight, &set.Reps); err != nil {
			return nil, err
		}
		i, ok := index[id]
		if !ok {
			continue
		}
		set.Bodyweight = bw
		if warm {
			list[i].Warmup = append(list[i].Warmup, set)
		} else {
			list[i].Main = append(list[i].Main, set)
		}
	}
	return list, nil
}

// ── /api/lift/overview ─────────────────────────────────────────────────────

func sessionSummary(s liftSession, st liftSessionStats) map[string]interface{} {
	return map[string]interface{}{
		"date":      s.Date,
		"e1rm":      st.E1rm,
		"topWeight": st.TopWeight,
		"topReps":   st.TopReps,
		"volume":    st.Volume,
		"sets":      len(s.Main),
		"mainText":  s.MainText,
	}
}

// mondayOf 는 date 가 속한 주의 월요일.
func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, time.Local)
}

func (c *LiftController) Overview() {
	conn := c.NewConnection()
	sessions, err := loadLiftSessions(conn, "")
	if err != nil {
		log.Error().Msg(err.Error())
		c.Error(err)
		return
	}

	// 종목별로 묶기 (sessions 는 날짜 오름차순)
	byName := map[string][]liftSession{}
	var order []string
	for _, s := range sessions {
		if _, ok := byName[s.Name]; !ok {
			order = append(order, s.Name)
		}
		byName[s.Name] = append(byName[s.Name], s)
	}

	exercises := []map[string]interface{}{}
	cardio := []map[string]interface{}{}
	prs := []map[string]interface{}{}
	for _, name := range order {
		list := byName[name]
		part := list[len(list)-1].Part

		if part == cardioPart {
			minutes, distance := 0.0, 0.0
			for _, s := range list {
				minutes += s.Minutes
				distance += s.Distance
			}
			cardio = append(cardio, map[string]interface{}{
				"name": name, "sessions": len(list), "lastDate": list[len(list)-1].Date,
				"totalMinutes": round1(minutes), "totalDistance": round1(distance),
			})
			continue
		}

		// 본세트가 있는 회차만 성장 지표에 쓴다
		var done []liftSession
		var stats []liftSessionStats
		for _, s := range list {
			if len(s.Main) > 0 {
				done = append(done, s)
				stats = append(stats, computeLiftStats(s.Main))
			}
		}
		if len(done) == 0 {
			continue
		}

		bodyweight := stats[len(stats)-1].Bodyweight
		bestIdx, bestMetric := 0, -1.0
		for i, st := range stats {
			m := st.metric()
			if i > 0 && m > bestMetric {
				prs = append(prs, prRecord(done[i], st))
			}
			if m > bestMetric {
				bestIdx, bestMetric = i, m
			}
		}
		best := map[string]interface{}{
			"e1rm": stats[bestIdx].E1rm, "weight": stats[bestIdx].TopWeight,
			"reps": stats[bestIdx].TopReps, "date": done[bestIdx].Date,
		}

		lastI := len(done) - 1
		item := map[string]interface{}{
			"name": name, "part": part, "bodyweight": bodyweight,
			"sessions": len(done), "firstDate": done[0].Date, "lastDate": done[lastI].Date,
			"best":            best,
			"last":            sessionSummary(done[lastI], stats[lastI]),
			"prev":            nil,
			"deltaE1rm":       nil,
			"deltaVolumePct":  nil,
			"nextTarget":      done[lastI].Target,
			"lastAchievement": nil,
		}
		if lastI > 0 {
			prev, pst, lst := done[lastI-1], stats[lastI-1], stats[lastI]
			item["prev"] = sessionSummary(prev, pst)
			item["deltaE1rm"] = round1(lst.metric() - pst.metric())
			if pst.Volume > 0 {
				item["deltaVolumePct"] = round1((lst.Volume/pst.Volume - 1) * 100)
			}
			if a := achievement(prev.Target, done[lastI].Main); a != nil {
				item["lastAchievement"] = a
			}
		}
		exercises = append(exercises, item)
	}
	sort.SliceStable(exercises, func(i, j int) bool {
		return exercises[i]["lastDate"].(string) > exercises[j]["lastDate"].(string)
	})
	sort.SliceStable(prs, func(i, j int) bool { return prs[i]["date"].(string) > prs[j]["date"].(string) })
	if len(prs) > 10 {
		prs = prs[:10]
	}

	// 최근 12주 (월요일 시작)
	thisMonday := mondayOf(time.Now())
	type weekAgg struct {
		days          map[string]bool
		sets, volume  map[string]float64
		cardioMinutes float64
		cardioCount   int
	}
	weeks := make([]*weekAgg, 12)
	weekStarts := make([]string, 12)
	for i := range weeks {
		weeks[i] = &weekAgg{days: map[string]bool{}, sets: map[string]float64{}, volume: map[string]float64{}}
		weekStarts[i] = thisMonday.AddDate(0, 0, -7*(11-i)).Format("2006-01-02")
	}
	weekOf := func(date string) *weekAgg {
		for i := 11; i >= 0; i-- {
			if date >= weekStarts[i] {
				if i == 11 || date < weekStarts[i+1] {
					return weeks[i]
				}
				return nil
			}
		}
		return nil
	}
	for _, s := range sessions {
		w := weekOf(s.Date)
		if w == nil {
			continue
		}
		w.days[s.Date] = true
		if s.Part == cardioPart {
			w.cardioMinutes += s.Minutes
			w.cardioCount++
			continue
		}
		if len(s.Main) == 0 {
			continue
		}
		w.sets[s.Part] += float64(len(s.Main))
		if st := computeLiftStats(s.Main); !st.Bodyweight {
			w.volume[s.Part] += st.Volume
		}
	}
	weekly := make([]map[string]interface{}, 12)
	for i, w := range weeks {
		sets, volume := map[string]int{}, map[string]float64{}
		splitDone := []string{}
		for _, p := range liftParts {
			sets[p] = int(w.sets[p])
			volume[p] = round1(w.volume[p])
			if w.sets[p] > 0 {
				splitDone = append(splitDone, p)
			}
		}
		weekly[i] = map[string]interface{}{
			"weekStart": weekStarts[i], "days": len(w.days), "sets": sets, "volume": volume,
			"splitDone": splitDone, "cardioMinutes": round1(w.cardioMinutes), "cardioSessions": w.cardioCount,
		}
	}

	parseErrors := []map[string]interface{}{}
	for i := len(sessions) - 1; i >= 0; i-- {
		s := sessions[i]
		if s.ParseError != "" {
			parseErrors = append(parseErrors, map[string]interface{}{
				"date": s.Date, "name": s.Name, "error": s.ParseError, "notionUrl": notionUrl(s.NotionId),
			})
		}
	}

	bodyweight := []map[string]interface{}{}
	from := time.Now().AddDate(0, 0, -180).Format("2006-01-02")
	if rows, err := conn.Query("SELECT hm_metricdate, hm_qty FROM healthmetric_tb WHERE hm_name = 'weight' AND hm_metricdate >= ? ORDER BY hm_metricdate", from); err == nil {
		for rows.Next() {
			var d string
			var q float64
			if rows.Scan(&d, &q) == nil {
				bodyweight = append(bodyweight, map[string]interface{}{"date": d[:10], "weight": round1(q)})
			}
		}
		rows.Close()
	}

	c.Set("sync", clients.GetLiftSyncStatus())
	c.Set("exercises", exercises)
	c.Set("cardio", cardio)
	c.Set("weekly", weekly)
	c.Set("prs", prs)
	c.Set("parseErrors", parseErrors)
	c.Set("bodyweight", bodyweight)
}

func prRecord(s liftSession, st liftSessionStats) map[string]interface{} {
	if st.Bodyweight {
		return map[string]interface{}{
			"date": s.Date, "name": s.Name, "kind": "reps", "value": st.TopReps,
			"text": fmt.Sprintf("최다 %d회", st.TopReps),
		}
	}
	return map[string]interface{}{
		"date": s.Date, "name": s.Name, "kind": "e1rm", "value": st.E1rm,
		"text": fmt.Sprintf("e1RM %gkg (%g×%d)", st.E1rm, st.TopWeight, st.TopReps),
	}
}

// ── /api/lift/exercise?name= ───────────────────────────────────────────────

func (c *LiftController) Exercise(name string) {
	if name == "" {
		c.Error(fmt.Errorf("name 필요"))
		return
	}
	conn := c.NewConnection()
	list, err := loadLiftSessions(conn, "WHERE le_name = ?", name)
	if err != nil {
		log.Error().Msg(err.Error())
		c.Error(err)
		return
	}

	part, bodyweight := "", false
	best := -1.0
	prevTarget := ""
	sessions := []map[string]interface{}{}
	for _, s := range list {
		part = s.Part
		st := computeLiftStats(s.Main)
		isPR := false
		if len(s.Main) > 0 {
			bodyweight = st.Bodyweight
			if best >= 0 && st.metric() > best {
				isPR = true
			}
			if st.metric() > best {
				best = st.metric()
			}
		}
		var ach interface{}
		if a := achievement(prevTarget, s.Main); a != nil && len(s.Main) > 0 {
			ach = a
		}
		sessions = append(sessions, map[string]interface{}{
			"date": s.Date, "notionUrl": notionUrl(s.NotionId),
			"warmup": nonNilSets(s.Warmup), "main": nonNilSets(s.Main),
			"warmupText": s.WarmupText, "mainText": s.MainText,
			"e1rm": st.E1rm, "topWeight": st.TopWeight, "topReps": st.TopReps,
			"volume": st.Volume, "totalReps": st.TotalReps,
			"goal": prevTarget, "achievement": ach, "target": s.Target,
			"isPR": isPR, "memo": s.Memo, "condition": s.Condition,
		})
		if len(s.Main) > 0 || s.Target != "" {
			prevTarget = s.Target
		}
	}

	c.Set("name", name)
	c.Set("part", part)
	c.Set("bodyweight", bodyweight)
	c.Set("sessions", sessions)
}

func nonNilSets(s []clients.LiftSet) []clients.LiftSet {
	if s == nil {
		return []clients.LiftSet{}
	}
	return s
}

// ── /api/lift/calendar?month=YYYY-MM ───────────────────────────────────────

func (c *LiftController) Calendar(month string) {
	start, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		start = time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
	}
	end := start.AddDate(0, 1, -1)
	from, to := start.Format("2006-01-02"), end.Format("2006-01-02")

	conn := c.NewConnection()

	type liftDay struct {
		NotionId, Title, Parts, Summary, Cardio, Condition string
	}
	liftDays := map[string]liftDay{}
	rows, err := conn.Query("SELECT ld_notionid, ld_date, ld_title, ld_parts, ld_summary, ld_cardio, ld_condition FROM liftday_tb WHERE ld_date BETWEEN ? AND ? ORDER BY ld_date", from, to)
	if err != nil {
		log.Error().Msg(err.Error())
		c.Error(err)
		return
	}
	for rows.Next() {
		var d liftDay
		var date string
		if rows.Scan(&d.NotionId, &date, &d.Title, &d.Parts, &d.Summary, &d.Cardio, &d.Condition) == nil {
			liftDays[date[:10]] = d // 같은 날 페이지가 여럿이면 마지막 것
		}
	}
	rows.Close()

	sessions, err := loadLiftSessions(conn, "WHERE le_date BETWEEN ? AND ?", from, to)
	if err != nil {
		log.Error().Msg(err.Error())
		c.Error(err)
		return
	}
	exByDate := map[string][]map[string]interface{}{}
	partsByDate := map[string][]string{}
	for _, s := range sessions {
		st := computeLiftStats(s.Main)
		exByDate[s.Date] = append(exByDate[s.Date], map[string]interface{}{
			"name": s.Name, "part": s.Part, "warmupText": s.WarmupText, "mainText": s.MainText,
			"main": nonNilSets(s.Main), "volume": st.Volume, "e1rm": st.E1rm,
			"minutes": s.Minutes, "speed": s.Speed, "incline": s.Incline, "distance": s.Distance, "memo": s.Memo,
		})
		if s.Part != "" && !containsStr(partsByDate[s.Date], s.Part) {
			partsByDate[s.Date] = append(partsByDate[s.Date], s.Part)
		}
	}

	type appleDay struct {
		sessions, seconds, calories int
		types                       string
	}
	apple := map[string]appleDay{}
	if rows, err := conn.Query("SELECT w_workoutdate, COUNT(*), COALESCE(SUM(w_duration),0), COALESCE(SUM(w_calories),0), COALESCE(GROUP_CONCAT(DISTINCT w_type),'') "+
		"FROM workout_tb WHERE w_workoutdate BETWEEN ? AND ? GROUP BY w_workoutdate", from, to); err == nil {
		for rows.Next() {
			var d string
			var a appleDay
			if rows.Scan(&d, &a.sessions, &a.seconds, &a.calories, &a.types) == nil {
				apple[d[:10]] = a
			}
		}
		rows.Close()
	}

	metrics := map[string]map[string]float64{} // date → name → qty
	if rows, err := conn.Query("SELECT hm_metricdate, hm_name, hm_qty FROM healthmetric_tb WHERE hm_name IN ('steps','weight') AND hm_metricdate BETWEEN ? AND ?", from, to); err == nil {
		for rows.Next() {
			var d, name string
			var q float64
			if rows.Scan(&d, &name, &q) == nil {
				if metrics[d[:10]] == nil {
					metrics[d[:10]] = map[string]float64{}
				}
				metrics[d[:10]][name] = q
			}
		}
		rows.Close()
	}

	days := []map[string]interface{}{}
	for t := start; !t.After(end); t = t.AddDate(0, 0, 1) {
		date := t.Format("2006-01-02")
		var lift interface{}
		if d, ok := liftDays[date]; ok {
			parts := []string{}
			if d.Parts != "" {
				parts = strings.Split(d.Parts, ",")
			}
			lift = map[string]interface{}{
				"title": d.Title, "parts": parts, "condition": d.Condition, "summary": d.Summary,
				"cardio": d.Cardio, "notionUrl": notionUrl(d.NotionId),
			}
		} else if len(exByDate[date]) > 0 {
			lift = map[string]interface{}{
				"title": "", "parts": partsByDate[date], "condition": "", "summary": "", "cardio": "", "notionUrl": "",
			}
		}
		exercises := exByDate[date]
		if exercises == nil {
			exercises = []map[string]interface{}{}
		}
		a := apple[date]
		types := []string{}
		if a.types != "" {
			types = strings.Split(a.types, ",")
		}
		days = append(days, map[string]interface{}{
			"date": date, "lift": lift, "exercises": exercises,
			"apple": map[string]interface{}{
				"sessions": a.sessions, "minutes": a.seconds / 60, "calories": a.calories, "types": types,
			},
			"steps":  int(metrics[date]["steps"]),
			"weight": round1(metrics[date]["weight"]),
		})
	}

	c.Set("month", start.Format("2006-01"))
	c.Set("days", days)
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ── POST /api/lift/sync ────────────────────────────────────────────────────

func (c *LiftController) Sync() {
	status, err := clients.SyncLift()
	c.Set("sync", status)
	if err != nil {
		c.Error(err)
	}
}

// ── 입력 (대시보드 → 노션) ─────────────────────────────────────────────────
// 쓰기는 clients.SaveLiftLog 등이 노션에 먼저 쓰고 전체 동기화까지 끝낸 뒤 돌아온다.

// liftNextPart 는 4분할 순환에서 part 다음 부위.
func liftNextPart(part string) string {
	for i, p := range liftParts {
		if p == part {
			return liftParts[(i+1)%len(liftParts)]
		}
	}
	return ""
}

func liftSessionView(s liftSession) map[string]interface{} {
	return map[string]interface{}{
		"date": s.Date, "warmupText": s.WarmupText, "mainText": s.MainText, "target": s.Target,
		"warmup": nonNilSets(s.Warmup), "main": nonNilSets(s.Main),
		"minutes": s.Minutes, "speed": s.Speed, "incline": s.Incline, "distance": s.Distance,
	}
}

// Form 은 입력 시트가 쓰는 그날 기록 + 종목 목록(지난 기록·목표) + 오늘 추천 부위.
func (c *LiftController) Form(date string) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		date = time.Now().Format("2006-01-02")
	}
	conn := c.NewConnection()
	sessions, err := loadLiftSessions(conn, "")
	if err != nil {
		log.Error().Msg(err.Error())
		c.Error(err)
		return
	}

	entries := []map[string]interface{}{}
	lastByName := map[string]liftSession{} // date 를 뺀 가장 최근 기록
	suggested, lastLiftDate := "", ""
	for _, s := range sessions {
		if s.Date == date {
			v := liftSessionView(s)
			v["id"], v["name"], v["part"], v["condition"], v["memo"] = s.NotionId, s.Name, s.Part, s.Condition, s.Memo
			entries = append(entries, v)
			continue
		}
		lastByName[s.Name] = s
		// 추천 부위: date 이전 마지막 웨이트 기록의 다음 부위
		if s.Date < date && s.Part != cardioPart && len(s.Main) > 0 && s.Date >= lastLiftDate {
			lastLiftDate = s.Date
			suggested = liftNextPart(s.Part)
		}
	}

	// 종목 목록: 노션 select 옵션 순서 + 옵션에 없는 기록 이름
	catalog := []map[string]interface{}{}
	seen := map[string]bool{}
	addCatalog := func(name, part string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		item := map[string]interface{}{"name": name, "part": part, "bodyweight": false, "last": nil, "goal": []clients.LiftSet{}}
		if s, ok := lastByName[name]; ok {
			if s.Part != "" {
				item["part"] = s.Part
			}
			item["last"] = liftSessionView(s)
			item["bodyweight"] = len(s.Main) > 0 && computeLiftStats(s.Main).Bodyweight
			if goal, err := clients.ParseSets(s.Target); err == nil && goal != nil {
				item["goal"] = goal
			}
		}
		catalog = append(catalog, item)
	}
	for _, o := range clients.LiftCatalogOptions() {
		addCatalog(o.Name, o.Part)
	}
	for i := len(sessions) - 1; i >= 0; i-- {
		addCatalog(sessions[i].Name, sessions[i].Part)
	}

	var day interface{}
	rows, err := conn.Query("SELECT ld_notionid, ld_title, ld_parts, ld_condition FROM liftday_tb WHERE ld_date = ? ORDER BY ld_id LIMIT 1", date)
	if err == nil {
		if rows.Next() {
			var id, title, parts, condition string
			if rows.Scan(&id, &title, &parts, &condition) == nil {
				list := []string{}
				if parts != "" {
					list = strings.Split(parts, ",")
				}
				day = map[string]interface{}{"id": id, "title": title, "parts": list, "condition": condition}
			}
		}
		rows.Close()
	}

	c.Set("date", date)
	c.Set("suggestedPart", suggested)
	c.Set("day", day)
	c.Set("entries", entries)
	c.Set("catalog", catalog)
	c.Set("parts", append(append([]string{}, liftParts...), cardioPart))
	c.Set("conditions", []string{"좋음", "보통", "나쁨"})
}

func (c *LiftController) SaveLog(in clients.LiftLogInput) {
	id, err := clients.SaveLiftLog(in)
	c.Set("id", id)
	c.Set("sync", clients.GetLiftSyncStatus())
	if err != nil {
		log.Error().Str("service", "lift-write").Msg(err.Error())
		c.Error(err)
	}
}

func (c *LiftController) DeleteLog(id string) {
	err := clients.DeleteLiftLog(id)
	c.Set("sync", clients.GetLiftSyncStatus())
	if err != nil {
		log.Error().Str("service", "lift-write").Msg(err.Error())
		c.Error(err)
	}
}

func (c *LiftController) SetDay(date, condition string) {
	err := clients.SetLiftDayCondition(date, condition)
	c.Set("sync", clients.GetLiftSyncStatus())
	if err != nil {
		log.Error().Str("service", "lift-write").Msg(err.Error())
		c.Error(err)
	}
}
