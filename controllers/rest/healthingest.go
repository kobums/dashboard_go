package rest

// Health Auto Export(iOS) 수신 엔드포인트.
// 페이로드 형태가 버전에 따라 조금씩 달라서 관대하게 파싱한다 — 모르는 지표는 건너뛴다.
// 멱등성: 지표는 (metricdate, name) upsert, 운동은 externalid(애플 UUID) 로 중복 스킵.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"encoding/json"
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

		for _, point := range metric.Data {
			date := haeDate(point.Date)
			if date == "" {
				continue
			}

			if err := upsertMetric(conn, date, name, point.Qty, metric.Units); err != nil {
				log.Error().Str("metric", name).Msg(err.Error())
				continue
			}
			count++
		}
	}

	return count
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
	if d, ok := raw["date"].(string); ok && len(d) >= 10 {
		date = d[:10]
	}

	conn := c.NewConnection()

	count := 0
	for key, val := range raw {
		if key == "date" {
			continue
		}
		qty, ok := toFloat(val)
		if !ok || qty == 0 {
			continue
		}
		name := normalizeMetricName(key)
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

// toFloat 은 JSON 숫자(float64) 또는 숫자 문자열을 float64 로 변환한다.
// 단축어가 "8,532" / "72.4 kg" 처럼 쉼표·단위 붙은 문자열을 보내는 경우도 처리한다.
func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		s := strings.ReplaceAll(strings.TrimSpace(n), ",", "")
		if i := strings.IndexAny(s, " \t"); i > 0 {
			s = s[:i] // "72.4 kg" → "72.4"
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	}
	return 0, false
}

func (c *HealthIngestController) ingestWorkouts(conn *models.Connection, workouts []haeWorkout) int {
	manager := models.NewWorkoutManager(conn)

	count := 0
	for _, workout := range workouts {
		if workout.Id == "" {
			continue
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

		item := &models.Workout{
			Type:        workoutType(workout.Name),
			Title:       workout.Name,
			Workoutdate: date,
			Starttime:   start,
			Duration:    duration,
			Calories:    int(workout.ActiveEnergyBurned.Qty),
			Distance:    models.Double(workout.Distance.Qty),
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
	return time.Parse("2006-01-02 15:04:05 -0700", value)
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
