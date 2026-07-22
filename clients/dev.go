package clients

// 개발 현황 집계 — github + gitlab 을 병합해 병합 히트맵 JSON 을 만든다.
// 일별 카운트는 devstat_tb 에 upsert 해 히스토리를 남기고(API 장애 대비),
// 히트맵은 devstat_tb 에서 조립한다.

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"dashboard/global/log"
	"dashboard/models"
)

type calendarDay struct {
	Date   string `json:"date"`
	Github int    `json:"github"`
	Gitlab int    `json:"gitlab"`
	Total  int    `json:"total"`
}

// upsertDevstat 은 (source, statdate) 기준 일별 카운트를 배치 upsert 한다.
// 개별 INSERT 를 순차로 날리면 원격 DB 왕복 지연이 누적되므로 멀티행 한 방으로 처리.
func upsertDevstat(conn *models.Connection, source string, days []DayCount) {
	if len(days) == 0 {
		return
	}
	placeholders := make([]string, 0, len(days))
	args := make([]interface{}, 0, len(days)*3)
	for _, d := range days {
		placeholders = append(placeholders, "(?, ?, ?, NOW())")
		args = append(args, source, d.Date, d.Count)
	}
	query := "INSERT INTO devstat_tb (ds_source, ds_statdate, ds_count, ds_createddate) VALUES " +
		strings.Join(placeholders, ",") +
		" ON DUPLICATE KEY UPDATE ds_count = VALUES(ds_count)"
	if _, err := conn.Exec(query, args...); err != nil {
		log.Error().Str("source", source).Msg(err.Error())
	}
}

// FetchDevSummary 는 병합 히트맵 + 소스별 합계 + 스트릭을 JSON 으로 만든다.
// days <= 0 이면 전체 기간(devstat_tb 의 최초 기록일부터 — 백필 데이터 포함).
// 외부 API 갱신은 항상 최근 1년만 한다 (GitHub GraphQL 이 1콜 최대 1년 제한,
// 과거는 백필로 이미 devstat_tb 에 있음).
func FetchDevSummary(days int) ([]byte, error) {
	to := time.Now()
	fetchFrom := to.AddDate(0, 0, -365)
	from := fetchFrom
	if days > 0 {
		from = to.AddDate(0, 0, -days)
	}

	// 외부 fetch — github/gitlab 병렬 (토큰 없으면 각각 빈 결과)
	var ghDays, glDays []DayCount
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		t0 := time.Now()
		var err error
		ghDays, err = FetchGithubCalendar(fetchFrom, to)
		if err != nil {
			log.Error().Str("source", "github").Msg(err.Error())
		}
		log.Info().Str("source", "github").Str("elapsed", time.Since(t0).String()).Int("days", len(ghDays)).Msg("dev fetch")
	}()
	go func() {
		defer wg.Done()
		t0 := time.Now()
		var err error
		glDays, err = FetchGitlabCalendar(fetchFrom, to)
		if err != nil {
			log.Error().Str("source", "gitlab").Msg(err.Error())
		}
		log.Info().Str("source", "gitlab").Str("elapsed", time.Since(t0).String()).Int("days", len(glDays)).Msg("dev fetch")
	}()
	wg.Wait()

	conn := models.NewConnection()
	defer conn.Close()

	if len(ghDays) > 0 {
		upsertDevstat(conn, "github", ghDays)
	}
	if len(glDays) > 0 {
		upsertDevstat(conn, "gitlab", glDays)
	}

	// 전체 모드: 캘린더 시작을 devstat_tb 최초 기록일로
	if days <= 0 {
		if minRows, err := conn.Query("SELECT MIN(ds_statdate) FROM devstat_tb WHERE ds_count > 0"); err == nil {
			var minDate *string
			if minRows.Next() {
				_ = minRows.Scan(&minDate)
			}
			minRows.Close()
			if minDate != nil && len(*minDate) >= 10 {
				if parsed, err := time.ParseInLocation("2006-01-02", (*minDate)[:10], time.Local); err == nil {
					from = parsed
				}
			}
		}
	}

	// devstat_tb 에서 구간 데이터를 읽어 조립 (fetch 실패 시에도 마지막 저장분 사용)
	fromStr := from.Format("2006-01-02")
	manager := models.NewDevstatManager(conn)
	rows := manager.Find([]interface{}{
		models.Where{Column: "statdate", Value: fromStr, Compare: ">="},
	})

	ghMap := map[string]int{}
	glMap := map[string]int{}
	ghTotal, glTotal := 0, 0
	for _, r := range rows {
		day := r.Statdate
		if len(day) > 10 {
			day = day[:10] // DATE 가 datetime 문자열로 올 경우 대비
		}
		if r.Source == "github" {
			ghMap[day] = r.Count
			ghTotal += r.Count
		} else if r.Source == "gitlab" {
			glMap[day] = r.Count
			glTotal += r.Count
		}
	}

	// from~to 전 구간을 하루 단위로 채운다(빈 날은 0).
	var calendar []calendarDay
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		gh := ghMap[key]
		gl := glMap[key]
		calendar = append(calendar, calendarDay{Date: key, Github: gh, Gitlab: gl, Total: gh + gl})
	}

	current, max := computeStreak(calendar)

	summary := map[string]interface{}{
		"days":     days,
		"from":     fromStr,
		"to":       to.Format("2006-01-02"),
		"calendar": calendar,
		"total": map[string]int{
			"github": ghTotal,
			"gitlab": glTotal,
			"all":    ghTotal + glTotal,
		},
		"week":   sumWindow(calendar, 7),
		"month":  sumWindow(calendar, 30),
		"streak": map[string]int{"current": current, "max": max},
	}
	return json.Marshal(summary)
}

// computeStreak 은 오늘부터 거꾸로 total>0 연속일(current)과 전체 최장(max)을 센다.
func computeStreak(calendar []calendarDay) (current, max int) {
	run := 0
	for _, day := range calendar {
		if day.Total > 0 {
			run++
			if run > max {
				max = run
			}
		} else {
			run = 0
		}
	}
	// current: 마지막(오늘)부터 역방향 연속
	for i := len(calendar) - 1; i >= 0; i-- {
		if calendar[i].Total > 0 {
			current++
		} else {
			break
		}
	}
	return current, max
}

// sumWindow 는 최근 n일 github/gitlab 합계.
func sumWindow(calendar []calendarDay, n int) map[string]int {
	gh, gl := 0, 0
	start := len(calendar) - n
	if start < 0 {
		start = 0
	}
	for _, day := range calendar[start:] {
		gh += day.Github
		gl += day.Gitlab
	}
	return map[string]int{"github": gh, "gitlab": gl, "all": gh + gl}
}

// FetchDevRecent 는 github(최대 10)+gitlab(최대 10)을 가져와
// 합친 뒤 시간순 내림차순 최근 20건을 반환한다.
func FetchDevRecent() ([]byte, error) {
	ghAct, err := FetchGithubRecent()
	if err != nil {
		log.Error().Str("source", "github").Msg(err.Error())
	}
	glAct, err := FetchGitlabRecent()
	if err != nil {
		log.Error().Str("source", "gitlab").Msg(err.Error())
	}

	if len(ghAct) > 10 {
		ghAct = ghAct[:10]
	}
	if len(glAct) > 10 {
		glAct = glAct[:10]
	}

	merged := append(ghAct, glAct...)
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Date > merged[j].Date
	})
	if len(merged) > 20 {
		merged = merged[:20]
	}
	if merged == nil {
		merged = []Activity{}
	}
	return json.Marshal(merged)
}
