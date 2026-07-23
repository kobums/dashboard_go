package rest

// 운동 통계 — 같은 요일 비교(/api/fitness/compare)와 연도별 통계(/api/fitness/yearly).
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"time"

	"dashboard/controllers"
	"dashboard/global/log"
)

type FitnessController struct {
	controllers.Controller
}

var weekdayKo = []string{"일요일", "월요일", "화요일", "수요일", "목요일", "금요일", "토요일"}

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
