package rest

// 공용 지표 수집 — 같은 요일 비교(/api/compare)와 알림(notify)이 함께 쓴다.
// 각 함수는 dates(YYYY-MM-DD 목록)에 대한 값 슬라이스를 반환한다. -1 = 데이터 없음.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"encoding/json"
	"fmt"
	"time"

	"dashboard/clients"
	"dashboard/global/log"
	"dashboard/models"
)

// queryMetricByDates 는 healthmetric_tb 에서 지표(name)의 날짜별 값을 뽑는다.
func queryMetricByDates(conn *models.Connection, name string, dates []string) []float64 {
	values := make([]float64, len(dates))
	for i := range values {
		values[i] = -1
	}
	args := make([]interface{}, 0, len(dates)+1)
	args = append(args, name)
	placeholders := ""
	for i, d := range dates {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args = append(args, d)
	}
	rows, err := conn.Query(
		"SELECT hm_metricdate, hm_qty FROM healthmetric_tb WHERE hm_name = ? AND hm_metricdate IN ("+placeholders+")",
		args...)
	if err != nil {
		log.Error().Msg(err.Error())
		return values
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var qty float64
		if err := rows.Scan(&day, &qty); err != nil {
			continue
		}
		if len(day) > 10 {
			day = day[:10]
		}
		for i, d := range dates {
			if d == day {
				values[i] = qty
			}
		}
	}
	return values
}

// queryWorkoutMinutes 는 workout_tb 일별 운동 분·건수를 뽑는다 (기록 없음 = 0).
func queryWorkoutMinutes(conn *models.Connection, dates []string) (minutes []float64, counts []int) {
	minutes = make([]float64, len(dates))
	counts = make([]int, len(dates))
	args := make([]interface{}, 0, len(dates))
	placeholders := ""
	for i, d := range dates {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args = append(args, d)
	}
	rows, err := conn.Query(
		"SELECT w_workoutdate, COUNT(*), COALESCE(SUM(w_duration),0) FROM workout_tb WHERE w_workoutdate IN ("+placeholders+") GROUP BY w_workoutdate",
		args...)
	if err != nil {
		log.Error().Msg(err.Error())
		return minutes, counts
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var cnt int
		var dur float64
		if err := rows.Scan(&day, &cnt, &dur); err != nil {
			continue
		}
		if len(day) > 10 {
			day = day[:10]
		}
		for i, d := range dates {
			if d == day {
				minutes[i] = dur / 60
				counts[i] = cnt
			}
		}
	}
	return minutes, counts
}

// queryDevByDates 는 5분 캐시된 dev summary(전체 기간)에서 날짜별 커밋 수를 뽑는다.
func queryDevByDates(dates []string) []float64 {
	values := make([]float64, len(dates))
	buf, err := clients.GetCached("dev_summary_0", 5*time.Minute, func() ([]byte, error) {
		return clients.FetchDevSummary(0)
	})
	if err != nil {
		log.Error().Msg(err.Error())
		values[0] = -1
		return values
	}
	var parsed struct {
		Calendar []struct {
			Date  string `json:"date"`
			Total int    `json:"total"`
		} `json:"calendar"`
	}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		values[0] = -1
		return values
	}
	for _, day := range parsed.Calendar {
		for i, d := range dates {
			if d == day.Date {
				values[i] = float64(day.Total)
			}
		}
	}
	return values
}

// checkReadingDone 은 reading summary 캐시의 streak.lastReadDate 로 기준일 독서 여부를 본다.
func checkReadingDone(baseDate string) bool {
	now := time.Now()
	key := fmt.Sprintf("reading_summary_%v_%v", now.Year(), int(now.Month()))
	buf, err := clients.GetCached(key, 10*time.Minute, func() ([]byte, error) {
		return clients.FetchReadingSummary(now.Year(), int(now.Month()))
	})
	if err != nil {
		return false
	}
	var parsed struct {
		Streak struct {
			LastReadDate string `json:"lastReadDate"`
		} `json:"streak"`
	}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return false
	}
	last := parsed.Streak.LastReadDate
	if len(last) > 10 {
		last = last[:10]
	}
	return last >= baseDate
}

// queryReadingByDates 는 10분 캐시된 reading daily 에서 날짜별 독서 분·페이지를 뽑는다.
// 세션 없는 날 = 안 읽은 날 = 0 (유효한 값).
func queryReadingByDates(dates []string) (minutes []float64, pages []float64) {
	minutes = make([]float64, len(dates))
	pages = make([]float64, len(dates))
	buf, err := clients.GetCached("reading_daily", 10*time.Minute, clients.FetchReadingDaily)
	if err != nil {
		log.Error().Msg(err.Error())
		return minutes, pages
	}
	var parsed []struct {
		Date    string `json:"date"`
		Minutes int    `json:"minutes"`
		Pages   int    `json:"pages"`
	}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return minutes, pages
	}
	for _, day := range parsed {
		for i, d := range dates {
			if d == day.Date {
				minutes[i] = float64(day.Minutes)
				pages[i] = float64(day.Pages)
			}
		}
	}
	return minutes, pages
}
