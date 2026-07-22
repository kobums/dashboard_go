package rest

// 운동 통계 — 같은 요일 비교(/api/fitness/compare)와 연도별 통계(/api/fitness/yearly).
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"time"

	"dashboard/controllers"
	"dashboard/global/log"
	"dashboard/models"
)

type FitnessController struct {
	controllers.Controller
}

// comparePoint 는 한 시점의 하루치 운동 지표.
// 지표 부재(nil)와 0 을 구분하기 위해 포인터를 쓴다.
type comparePoint struct {
	Label           string   `json:"label"` // base | week | month | year
	Date            string   `json:"date"`
	Steps           *float64 `json:"steps"`
	ActiveEnergy    *float64 `json:"activeEnergy"`
	ExerciseMinutes *float64 `json:"exerciseMinutes"`
	WorkoutCount    int      `json:"workoutCount"`
	WorkoutMinutes  int      `json:"workoutMinutes"`
}

var weekdayKo = []string{"일요일", "월요일", "화요일", "수요일", "목요일", "금요일", "토요일"}

// Compare 는 기준일과 7/28/364일 전(전부 같은 요일)의 지표를 비교한다.
// 기준일에 지표가 하나도 없으면 어제로 폴백한다.
func (c *FitnessController) Compare(dateStr string) {
	base, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		base = time.Now()
	}
	base = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, time.Local)

	conn := c.NewConnection()

	points := buildComparePoints(conn, base)

	// 기준일 폴백: 지표·운동이 전무하면 어제 기준으로 다시
	basePoint := points[0]
	if basePoint.Steps == nil && basePoint.ActiveEnergy == nil &&
		basePoint.ExerciseMinutes == nil && basePoint.WorkoutCount == 0 {
		base = base.AddDate(0, 0, -1)
		points = buildComparePoints(conn, base)
	}

	c.Set("baseDate", base.Format("2006-01-02"))
	c.Set("weekdayLabel", weekdayKo[int(base.Weekday())])
	c.Set("points", points)
}

func buildComparePoints(conn *models.Connection, base time.Time) []comparePoint {
	offsets := []struct {
		label string
		days  int
	}{
		{"base", 0},
		{"week", -7},
		{"month", -28}, // 4주 전 — 같은 요일 보존
		{"year", -364}, // 52주 전 — "작년 같은 요일"
	}

	dates := make([]string, len(offsets))
	points := make([]comparePoint, len(offsets))
	for i, off := range offsets {
		d := base.AddDate(0, 0, off.days).Format("2006-01-02")
		dates[i] = d
		points[i] = comparePoint{Label: off.label, Date: d}
	}
	index := map[string]int{}
	for i, d := range dates {
		index[d] = i
	}

	// 일별 지표
	rows, err := conn.Query(
		"SELECT hm_metricdate, hm_name, hm_qty FROM healthmetric_tb WHERE hm_metricdate IN (?, ?, ?, ?)",
		dates[0], dates[1], dates[2], dates[3])
	if err != nil {
		log.Error().Msg(err.Error())
	} else {
		defer rows.Close()
		for rows.Next() {
			var day, name string
			var qty float64
			if err := rows.Scan(&day, &name, &qty); err != nil {
				continue
			}
			if len(day) > 10 {
				day = day[:10]
			}
			i, ok := index[day]
			if !ok {
				continue
			}
			v := qty
			switch name {
			case "steps":
				points[i].Steps = &v
			case "active_energy":
				points[i].ActiveEnergy = &v
			case "exercise_minutes":
				points[i].ExerciseMinutes = &v
			}
		}
	}

	// 운동 세션
	wrows, err := conn.Query(
		"SELECT w_workoutdate, COUNT(*), COALESCE(SUM(w_duration),0) FROM workout_tb WHERE w_workoutdate IN (?, ?, ?, ?) GROUP BY w_workoutdate",
		dates[0], dates[1], dates[2], dates[3])
	if err != nil {
		log.Error().Msg(err.Error())
	} else {
		defer wrows.Close()
		for wrows.Next() {
			var day string
			var cnt, dur int
			if err := wrows.Scan(&day, &cnt, &dur); err != nil {
				continue
			}
			if len(day) > 10 {
				day = day[:10]
			}
			if i, ok := index[day]; ok {
				points[i].WorkoutCount = cnt
				points[i].WorkoutMinutes = dur / 60
			}
		}
	}

	return points
}

type monthlyFitness struct {
	Month    int `json:"month"`
	Sessions int `json:"sessions"`
	Minutes  int `json:"minutes"`
}

type yearFitness struct {
	Year     int              `json:"year"`
	Sessions int              `json:"sessions"`
	Minutes  int              `json:"minutes"`
	Distance float64          `json:"distance"`
	Calories int              `json:"calories"`
	AvgSteps int              `json:"avgSteps"`
	ByType   map[string]int   `json:"byType"`
	Monthly  []monthlyFitness `json:"monthly"`
}

// Yearly 는 연도별 운동 요약(+월별·타입별)과 연평균 걸음을 반환한다.
func (c *FitnessController) Yearly() {
	conn := c.NewConnection()

	years := map[int]*yearFitness{}
	getYear := func(y int) *yearFitness {
		if _, ok := years[y]; !ok {
			monthly := make([]monthlyFitness, 12)
			for m := range monthly {
				monthly[m].Month = m + 1
			}
			years[y] = &yearFitness{Year: y, ByType: map[string]int{}, Monthly: monthly}
		}
		return years[y]
	}

	// 연도·월·타입별 운동 집계 (한 쿼리로)
	rows, err := conn.Query(
		"SELECT YEAR(w_workoutdate), MONTH(w_workoutdate), w_type, COUNT(*), COALESCE(SUM(w_duration),0), COALESCE(SUM(w_distance),0), COALESCE(SUM(w_calories),0) " +
			"FROM workout_tb GROUP BY YEAR(w_workoutdate), MONTH(w_workoutdate), w_type")
	if err != nil {
		log.Error().Msg(err.Error())
		c.Error(err)
		return
	}
	for rows.Next() {
		var y, m, cnt, dur, cal int
		var wtype string
		var dist float64
		if err := rows.Scan(&y, &m, &wtype, &cnt, &dur, &dist, &cal); err != nil {
			continue
		}
		yf := getYear(y)
		yf.Sessions += cnt
		yf.Minutes += dur / 60
		yf.Distance += dist
		yf.Calories += cal
		yf.ByType[wtype] += cnt
		if m >= 1 && m <= 12 {
			yf.Monthly[m-1].Sessions += cnt
			yf.Monthly[m-1].Minutes += dur / 60
		}
	}
	rows.Close()

	// 연평균 걸음
	srows, err := conn.Query(
		"SELECT YEAR(hm_metricdate), ROUND(AVG(hm_qty)) FROM healthmetric_tb WHERE hm_name = 'steps' GROUP BY YEAR(hm_metricdate)")
	if err == nil {
		for srows.Next() {
			var y int
			var avg float64
			if err := srows.Scan(&y, &avg); err != nil {
				continue
			}
			getYear(y).AvgSteps = int(avg)
		}
		srows.Close()
	}

	// 연도 내림차순 정렬
	list := make([]*yearFitness, 0, len(years))
	for y := time.Now().Year(); y >= 2000; y-- {
		if yf, ok := years[y]; ok {
			yf.Distance = float64(int(yf.Distance*10)) / 10
			list = append(list, yf)
		}
	}

	c.Set("years", list)
}
