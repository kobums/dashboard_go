package rest

// Health Auto Export(iOS) 수신 엔드포인트.
// 페이로드 형태가 버전에 따라 조금씩 달라서 관대하게 파싱한다 — 모르는 지표는 건너뛴다.
// 멱등성: 지표는 (metricdate, name) upsert, 운동은 externalid(애플 UUID) 로 중복 스킵.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dashboard/controllers"
	"dashboard/global/log"
	"dashboard/models"
)

type HealthIngestController struct {
	controllers.Controller
}

type haeQuantity struct {
	Qty   float64 `json:"qty"`
	Units string  `json:"units"`
}

type haeMetricPoint struct {
	Date string  `json:"date"`
	Qty  float64 `json:"qty"`
}

type haeMetric struct {
	Name  string           `json:"name"`
	Units string           `json:"units"`
	Data  []haeMetricPoint `json:"data"`
}

type haeWorkout struct {
	Id                 string      `json:"id"`
	Name               string      `json:"name"`
	Start              string      `json:"start"`
	End                string      `json:"end"`
	Duration           float64     `json:"duration"`
	ActiveEnergyBurned haeQuantity `json:"activeEnergyBurned"`
	ActiveEnergy       haeQuantity `json:"activeEnergy"` // 구버전(v1) 형식 — activeEnergyBurned 가 없을 때
	Distance           haeQuantity `json:"distance"`
}

type haePayload struct {
	Data struct {
		Metrics  []haeMetric  `json:"metrics"`
		Workouts []haeWorkout `json:"workouts"`
	} `json:"data"`
}

// Health Auto Export 지표명 → healthmetric_tb hm_name 정규화 매핑.
// Health Auto Export 의 정확한 identifier 문자열은 앱/버전마다 다를 수 있어
// 공식 문서로 확정할 수 없다. 그래서 매핑에 없는 name 도 버리지 않고 원본 그대로
// 저장한다(데이터 유실 방지). 배포 후 첫 export 의 hm_name 을 보고 매핑을 확정한다.
var metricNameMap = map[string]string{
	"step_count":          "steps",
	"active_energy":       "active_energy",
	"apple_exercise_time": "exercise_minutes",
	"weight_body_mass":    "weight",
	"resting_heart_rate":  "resting_hr",
}

// 하루 누적 지표 — 한 번 보낸 데이터 안에 같은 날 점이 여러 개면(시간 단위 그룹핑·원본 샘플) 더해야 하루 값이 된다.
// 그 외(체중·심박 등)는 그날 마지막 값을 쓴다.
var cumulativeMetrics = map[string]bool{
	"steps":                    true,
	"active_energy":            true,
	"exercise_minutes":         true,
	"basal_energy_burned":      true,
	"walking_running_distance": true,
	"flights_climbed":          true,
}

// normalizeMetricName 은 알려진 이름은 정규화하고, 모르는 이름은 원본을 소문자로 반환한다.
func normalizeMetricName(raw string) string {
	if mapped, ok := metricNameMap[raw]; ok {
		return mapped
	}
	return strings.ToLower(raw)
}

// Ingest 는 raw body 를 직접 받아 관대하게 파싱한다.
// (BodyParser 대신 json.Unmarshal — 모르는 필드는 무시된다)
func (c *HealthIngestController) Ingest(body []byte) {
	payload := &haePayload{}
	if err := json.Unmarshal(body, payload); err != nil {
		log.Debug().Str("body", truncate(string(body), 500)).Msg("health ingest parse failed")
		c.Error(err)
		return
	}

	// 배포 후 실제 metric identifier 확정을 위해 수신한 name 목록을 로그에 남긴다.
	names := make([]string, 0, len(payload.Data.Metrics))
	for _, m := range payload.Data.Metrics {
		names = append(names, m.Name)
	}
	log.Info().
		Str("metricNames", strings.Join(names, ",")).
		Int("workouts", len(payload.Data.Workouts)).
		Msg("health ingest received")

	conn := c.NewConnection()

	metricCount := c.ingestMetrics(conn, payload.Data.Metrics)
	workoutCount := c.ingestWorkouts(conn, payload.Data.Workouts)

	c.Set("metrics", metricCount)
	c.Set("workouts", workoutCount)
}

// upsertMetric 은 (metricdate, name) 기준 지표 한 건을 멱등 저장한다.
func upsertMetric(conn *models.Connection, date, name string, qty float64, unit string) error {
	_, err := conn.Exec(
		"INSERT INTO healthmetric_tb (hm_metricdate, hm_name, hm_qty, hm_unit, hm_createddate) VALUES (?, ?, ?, ?, NOW()) "+
			"ON DUPLICATE KEY UPDATE hm_qty = VALUES(hm_qty), hm_unit = VALUES(hm_unit)",
		date, name, qty, unit,
	)
	return err
}

func (c *HealthIngestController) ingestMetrics(conn *models.Connection, metrics []haeMetric) int {
	count := 0
	for _, metric := range metrics {
		name := normalizeMetricName(metric.Name)
		daily, unit := dailyMetricValues(name, metric)

		for date, qty := range daily {
			if err := upsertMetric(conn, date, name, qty, unit); err != nil {
				log.Error().Str("metric", name).Msg(err.Error())
				continue
			}
			count++
		}
		log.Info().Str("metric", name).Int("points", len(metric.Data)).Int("days", len(daily)).Msg("health ingest metric")
	}

	return count
}

// dailyMetricValues 는 한 지표의 점들을 날짜별 하루 값으로 모은다.
// 예전엔 점마다 upsert 해서 시간 단위로 보내면 하루 걸음이 「마지막 한 시간」 값으로 덮였다.
func dailyMetricValues(name string, metric haeMetric) (map[string]float64, string) {
	unit := metric.Units
	scale := 1.0
	if strings.EqualFold(unit, "kJ") {
		scale, unit = 1/4.184, "kcal" // 에너지는 kcal 로 통일
	}

	daily := map[string]float64{}
	for _, point := range metric.Data {
		date := haeDate(point.Date)
		if date == "" {
			continue
		}
		qty := point.Qty * scale
		if cumulativeMetrics[name] {
			daily[date] += qty
		} else {
			daily[date] = qty
		}
	}
	return daily, unit
}

// IngestShortcut 은 iOS 기본 단축어(Shortcuts)용 수신 엔드포인트다.
// 단축어의 "사전" 액션으로 만들기 쉬운 평평한 JSON 을 받는다:
//
//	{ "date": "2026-07-22", "steps": 8532, "weight": 72.4, "active_energy": 410,
//	  "exercise_minutes": 35, "resting_hr": 58 }
//
// date 는 선택(없으면 서버의 오늘 날짜). 나머지 키는 자유 — 값이 있는 것만 저장한다.
// 값은 숫자 또는 숫자 문자열 모두 허용(단축어가 문자열로 보내는 경우 대비).
func (c *HealthIngestController) IngestShortcut(body []byte) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		log.Debug().Str("body", truncate(string(body), 500)).Msg("shortcut ingest parse failed")
		c.Error(err)
		return
	}

	date := time.Now().Format("2006-01-02")
	if d, ok := raw["date"].(string); ok && strings.TrimSpace(d) != "" {
		if parsed, ok := parseShortcutDate(d); ok {
			date = parsed
		} else {
			log.Error().Str("date", d).Msg("shortcut ingest: 날짜 형식을 읽지 못해 오늘로 저장")
		}
	}

	conn := c.NewConnection()

	count := 0
	for key, val := range raw {
		if key == "date" {
			continue
		}
		name := normalizeMetricName(key)
		qty, ok := shortcutValue(name, val)
		if !ok || qty == 0 {
			log.Info().Str("key", key).Str("value", truncate(fmt.Sprint(val), 100)).Msg("shortcut ingest: 값 없음/형식 오류로 건너뜀")
			continue
		}
		if err := upsertMetric(conn, date, name, qty, ""); err != nil {
			log.Error().Str("metric", name).Msg(err.Error())
			continue
		}
		count++
	}

	log.Info().Str("date", date).Int("saved", count).Msg("shortcut ingest received")
	c.Set("metrics", count)
	c.Set("date", date)
}

var numberPattern = regexp.MustCompile(`-?\d+(\.\d+)?`)

// toFloat 은 JSON 숫자(float64) 또는 숫자 문자열을 float64 로 변환한다.
// 단축어가 "8,532" / "72.4 kg" / "72.4kg" / "8532 걸음" 처럼 쉼표·단위 붙은 문자열을 보내는 경우도 처리한다.
func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		m := numberPattern.FindString(strings.ReplaceAll(n, ",", ""))
		if m == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(m, 64)
		return f, err == nil
	}
	return 0, false
}

// shortcutValue 는 단축어 값 하나를 하루 값으로 바꾼다.
// 「건강 샘플 찾기」 결과를 그대로 넣으면 배열이나 줄바꿈으로 이어진 여러 값이 온다 —
// 누적 지표(걸음 등)는 더하고, 나머지(체중 등)는 마지막 값을 쓴다.
func shortcutValue(name string, v interface{}) (float64, bool) {
	var parts []interface{}
	switch n := v.(type) {
	case []interface{}:
		parts = n
	case string:
		for _, line := range strings.Split(strings.TrimSpace(n), "\n") {
			if strings.TrimSpace(line) != "" {
				parts = append(parts, line)
			}
		}
	default:
		parts = []interface{}{v}
	}

	total, found := 0.0, false
	for _, p := range parts {
		f, ok := toFloat(p)
		if !ok {
			continue
		}
		if cumulativeMetrics[name] {
			total += f
		} else {
			total = f
		}
		found = true
	}
	return total, found
}

var datePattern = regexp.MustCompile(`(\d{4})\D{1,3}(\d{1,2})\D{1,3}(\d{1,2})`)

// parseShortcutDate 는 단축어가 보내는 날짜를 YYYY-MM-DD 로 바꾼다.
// 한국어 지역 설정의 「2026. 10. 20.」「2026년 10월 20일 오후 9:00」 이나 ISO「2026-10-20T21:00:00+09:00」 모두 허용.
// (예전엔 앞 10글자를 그대로 잘라 「2026. 10. 2」 같은 값이 저장됐다)
func parseShortcutDate(s string) (string, bool) {
	m := datePattern.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local)
	if t.Year() != y || int(t.Month()) != mo || t.Day() != d {
		return "", false // 2026-02-31 같은 잘못된 날짜
	}
	return t.Format("2006-01-02"), true
}

func (c *HealthIngestController) ingestWorkouts(conn *models.Connection, workouts []haeWorkout) int {
	manager := models.NewWorkoutManager(conn)

	count := 0
	for _, workout := range workouts {
		if workout.Id == "" {
			// 구버전 형식엔 id 가 없다 — 예전엔 통째로 버렸다. 시작 시각+종류로 멱등 키를 만든다.
			if workout.Start == "" {
				continue
			}
			workout.Id = "hae-" + workout.Start + "-" + workout.Name
		}

		exists := manager.Count([]interface{}{
			models.Where{Column: "externalid", Value: workout.Id, Compare: "="},
		})
		if exists > 0 {
			continue
		}

		start := haeDateTime(workout.Start)
		date := haeDate(workout.Start)
		if date == "" {
			continue
		}

		duration := int(workout.Duration)
		if duration == 0 && workout.Start != "" && workout.End != "" {
			startTime, err1 := parseHaeTime(workout.Start)
			endTime, err2 := parseHaeTime(workout.End)
			if err1 == nil && err2 == nil {
				duration = int(endTime.Sub(startTime).Seconds())
			}
		}

		energy := workout.ActiveEnergyBurned
		if energy.Qty == 0 {
			energy = workout.ActiveEnergy
		}

		item := &models.Workout{
			Type:        workoutType(workout.Name),
			Title:       workout.Name,
			Workoutdate: date,
			Starttime:   start,
			Duration:    duration,
			Calories:    int(math.Round(toKcal(energy))),
			Distance:    models.Double(toKm(workout.Distance)),
			Source:      "apple",
			Externalid:  workout.Id,
		}
		if err := manager.Insert(item); err != nil {
			log.Error().Str("workout", workout.Id).Msg(err.Error())
			continue
		}
		count++
	}

	return count
}

// haeDate 는 "2026-07-22 00:00:00 +0900" → "2026-07-22"
func haeDate(value string) string {
	if len(value) < 10 {
		return ""
	}
	return value[:10]
}

// haeDateTime 은 "2026-07-22 07:00:00 +0900" → "2026-07-22 07:00:00"
func haeDateTime(value string) string {
	if len(value) < 19 {
		return models.InitDate()
	}
	return value[:19]
}

func parseHaeTime(value string) (time.Time, error) {
	t, err := time.Parse("2006-01-02 15:04:05 -0700", value)
	if err != nil {
		return time.Parse(time.RFC3339, value)
	}
	return t, nil
}

// toKcal 은 kJ 로 온 에너지를 kcal 로 바꾼다 (기기 지역 설정에 따라 kJ 로 내보내는 경우).
func toKcal(q haeQuantity) float64 {
	if strings.EqualFold(q.Units, "kJ") {
		return q.Qty / 4.184
	}
	return q.Qty
}

// toKm 은 거리를 km 로 맞춘다 (workout_tb 거리 단위).
func toKm(q haeQuantity) float64 {
	switch strings.ToLower(q.Units) {
	case "m":
		return q.Qty / 1000
	case "mi":
		return q.Qty * 1.609344
	}
	return q.Qty
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func workoutType(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "strength"), strings.Contains(lower, "functional"):
		return "weight"
	case strings.Contains(lower, "run"):
		return "running"
	case strings.Contains(lower, "cycl"), strings.Contains(lower, "bike"):
		return "cycling"
	case strings.Contains(lower, "swim"):
		return "swimming"
	case strings.Contains(lower, "walk"):
		return "walking"
	case strings.Contains(lower, "hik"):
		return "hiking"
	default:
		return "etc"
	}
}
