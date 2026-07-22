package rest

// 알림 판정 — 기준일(오늘/어제)의 걸음·운동·커밋·독서를
// 전날/전주 같은 요일/4주 전/1년 전(364일)과 비교해 부족 여부와 요약 문장을 만든다.
// 채널(대시보드 배너·iOS 단축어)이 모두 조회형이라 크론 없이 이 API 하나로 동작한다.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dashboard/clients"
	"dashboard/controllers"
	"dashboard/global/log"
	"dashboard/models"
)

type NotifyController struct {
	controllers.Controller
}

type notifyCompare struct {
	Vs    string  `json:"vs"` // 전날 | 1주 전 | 4주 전 | 1년 전
	Other float64 `json:"other"`
	Pct   int     `json:"pct"` // 기준일이 비교일 대비 몇 % 인지 (음수 = 부족)
}

type notifyAlert struct {
	Area    string          `json:"area"` // steps | workout | dev | reading
	Label   string          `json:"label"`
	Value   float64         `json:"value"`
	Unit    string          `json:"unit"`
	Behind  []notifyCompare `json:"behind"`
	Ahead   []notifyCompare `json:"ahead"`
	Message string          `json:"message"`
}

var compareOffsets = []struct {
	label string
	days  int
}{
	{"전날", -1},
	{"1주 전", -7},
	{"4주 전", -28},
	{"1년 전", -364},
}

// Check 는 mode(evening|morning|auto)에 따라 판정한다.
func (c *NotifyController) Check(mode string) {
	now := time.Now()
	if mode != "evening" && mode != "morning" {
		if now.Hour() < 12 {
			mode = "morning"
		} else {
			mode = "evening"
		}
	}

	base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if mode == "morning" {
		base = base.AddDate(0, 0, -1)
	}

	conn := c.NewConnection()

	dates := make([]string, 0, 5)
	dates = append(dates, base.Format("2006-01-02"))
	for _, off := range compareOffsets {
		dates = append(dates, base.AddDate(0, 0, off.days).Format("2006-01-02"))
	}

	steps := queryMetricByDates(conn, "steps", dates)
	workoutMin := queryWorkoutMinutes(conn, dates)
	devCounts := queryDevByDates(dates)
	readingDone := checkReadingDone(dates[0])

	alerts := []notifyAlert{}
	alerts = append(alerts, buildAlert("steps", "걸음", "보", steps, false))
	alerts = append(alerts, buildAlert("workout", "운동", "분", workoutMin, true))
	alerts = append(alerts, buildAlert("dev", "커밋", "회", devCounts, true))

	// 독서 — 비교 데이터가 없어 단순 규칙: 기준일에 세션 없으면 경고
	readingAlert := notifyAlert{Area: "reading", Label: "독서", Unit: "", Behind: []notifyCompare{}, Ahead: []notifyCompare{}}
	if readingDone {
		readingAlert.Value = 1
		readingAlert.Message = "독서 ✓ 완료"
	} else {
		readingAlert.Value = 0
		readingAlert.Behind = append(readingAlert.Behind, notifyCompare{Vs: "오늘", Other: 1, Pct: -100})
		if mode == "evening" {
			readingAlert.Message = "오늘 아직 책을 안 읽었어요 — 연속 독서가 끊겨요"
		} else {
			readingAlert.Message = "어제 독서를 쉬었어요"
		}
	}
	alerts = append(alerts, readingAlert)

	// 유효 alert 만 (기준일 값 자체가 없어 판정 불가한 지표는 제외)
	valid := alerts[:0]
	for _, a := range alerts {
		if a.Message != "" {
			valid = append(valid, a)
		}
	}

	behindCount := 0
	for _, a := range valid {
		if len(a.Behind) > 0 {
			behindCount++
		}
	}

	notify := behindCount > 0
	if mode == "morning" {
		notify = true // 아침은 항상 보고
	}

	c.Set("mode", mode)
	c.Set("date", dates[0])
	c.Set("weekdayLabel", weekdayKo[int(base.Weekday())])
	c.Set("notify", notify)
	c.Set("alerts", valid)
	c.Set("summary", buildSummary(mode, valid, behindCount))
}

// buildAlert 는 values[0]=기준일, values[1..4]=비교일 값(-1 = 데이터 없음)으로 판정한다.
// zeroValid: 기준일 값 0 을 유효한 값으로 볼지 (운동/커밋은 0 = 실제 안 함, 걸음은 미전송)
func buildAlert(area, label, unit string, values []float64, zeroValid bool) notifyAlert {
	alert := notifyAlert{Area: area, Label: label, Unit: unit, Behind: []notifyCompare{}, Ahead: []notifyCompare{}}

	base := values[0]
	if base < 0 || (base == 0 && !zeroValid) {
		return alert // 기준일 데이터 없음 → 판정 불가 (Message 비움)
	}
	alert.Value = base

	for i, off := range compareOffsets {
		other := values[i+1]
		if other < 0 || other == 0 {
			continue // 비교일 데이터 없음(또는 0 — 비교 무의미)
		}
		pct := int((base - other) / other * 100)
		cmp := notifyCompare{Vs: off.label, Other: other, Pct: pct}
		if base < other {
			alert.Behind = append(alert.Behind, cmp)
		} else {
			alert.Ahead = append(alert.Ahead, cmp)
		}
	}

	if len(alert.Behind) > 0 {
		first := alert.Behind[0]
		alert.Message = fmt.Sprintf("%s %s%s — %s(%s%s)보다 %d%% 적어요",
			label, formatNum(base), unit, first.Vs, formatNum(first.Other), unit, -first.Pct)
		if len(alert.Behind) > 1 {
			alert.Message += fmt.Sprintf(" (외 %d개 시점 대비 부족)", len(alert.Behind)-1)
		}
	} else if len(alert.Ahead) > 0 {
		alert.Message = fmt.Sprintf("%s %s%s ✓ 모든 비교 시점보다 많아요", label, formatNum(base), unit)
	}

	return alert
}

func buildSummary(mode string, alerts []notifyAlert, behindCount int) string {
	var behind, ahead []string
	for _, a := range alerts {
		if len(a.Behind) > 0 {
			behind = append(behind, a.Label)
		} else if a.Message != "" {
			ahead = append(ahead, a.Label)
		}
	}

	if mode == "evening" {
		if behindCount == 0 {
			return "오늘은 모든 지표가 이전 기록을 넘었어요 💪"
		}
		joined := strings.Join(behind, "·")
		msg := joined + subjectParticle(joined) + " 이전 기록보다 부족해요."
		// 첫 부족 지표의 상세를 붙인다
		for _, a := range alerts {
			if len(a.Behind) > 0 {
				msg += " " + a.Message + "."
				break
			}
		}
		return msg + " 남은 시간에 채워보세요!"
	}

	// morning: 어제 결과 보고
	parts := []string{}
	if len(ahead) > 0 {
		parts = append(parts, "✓ "+strings.Join(ahead, "·"))
	}
	if len(behind) > 0 {
		parts = append(parts, "▼ "+strings.Join(behind, "·"))
	}
	if len(parts) == 0 {
		return "어제는 판정할 데이터가 없었어요"
	}
	return "어제 결과: " + strings.Join(parts, " / ")
}

// subjectParticle 은 마지막 글자의 받침 유무로 이/가 를 고른다.
func subjectParticle(word string) string {
	runes := []rune(word)
	if len(runes) == 0 {
		return "가"
	}
	last := runes[len(runes)-1]
	if last >= 0xAC00 && last <= 0xD7A3 && (last-0xAC00)%28 != 0 {
		return "이"
	}
	return "가"
}

func formatNum(v float64) string {
	n := int64(v + 0.5)
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	for i, ch := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, ch)
	}
	return string(out)
}

// --- 지표 수집 (반환: [기준일, 전날, 1주 전, 4주 전, 1년 전], -1 = 데이터 없음) ---

func queryMetricByDates(conn *models.Connection, name string, dates []string) []float64 {
	values := []float64{-1, -1, -1, -1, -1}
	rows, err := conn.Query(
		"SELECT hm_metricdate, hm_qty FROM healthmetric_tb WHERE hm_name = ? AND hm_metricdate IN (?, ?, ?, ?, ?)",
		name, dates[0], dates[1], dates[2], dates[3], dates[4])
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

func queryWorkoutMinutes(conn *models.Connection, dates []string) []float64 {
	// 운동은 기록 없음 = 0분 (실제 안 한 것)
	values := []float64{0, 0, 0, 0, 0}
	rows, err := conn.Query(
		"SELECT w_workoutdate, COALESCE(SUM(w_duration),0) FROM workout_tb WHERE w_workoutdate IN (?, ?, ?, ?, ?) GROUP BY w_workoutdate",
		dates[0], dates[1], dates[2], dates[3], dates[4])
	if err != nil {
		log.Error().Msg(err.Error())
		return values
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var dur float64
		if err := rows.Scan(&day, &dur); err != nil {
			continue
		}
		if len(day) > 10 {
			day = day[:10]
		}
		for i, d := range dates {
			if d == day {
				values[i] = dur / 60
			}
		}
	}
	return values
}

// queryDevByDates 는 5분 캐시된 dev summary(전체 기간)에서 날짜별 커밋 수를 뽑는다.
// 캐시 경유라 외부 API 최신성도 함께 확보된다.
func queryDevByDates(dates []string) []float64 {
	values := []float64{0, 0, 0, 0, 0}
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
