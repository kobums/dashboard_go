package rest

// 통합 "같은 요일 비교" API — 세 영역(운동·독서·개발) 공용.
// 기준일 + 전날/1주 전/4주 전/1년 전(전부 같은 요일 보존, 전날 제외) 5개 시점의
// 전 지표를 한 번에 반환한다. 기준일에 지표가 전무하면 어제로 폴백.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"sync"
	"time"

	"dashboard/controllers"
	"dashboard/models"
)

type CompareController struct {
	controllers.Controller
}

// unifiedPoint — 포인터 = 데이터 없음(null)과 0 을 구분.
type unifiedPoint struct {
	Label           string   `json:"label"` // base | prev | week | month | year
	Date            string   `json:"date"`
	Steps           *float64 `json:"steps"`
	ActiveEnergy    *float64 `json:"activeEnergy"`
	ExerciseMinutes *float64 `json:"exerciseMinutes"`
	WorkoutMinutes  *float64 `json:"workoutMinutes"`
	WorkoutCount    int      `json:"workoutCount"`
	DevCommits      *float64 `json:"devCommits"`
	ReadingMinutes  *float64 `json:"readingMinutes"`
	ReadingPages    *float64 `json:"readingPages"`
}

var unifiedOffsets = []struct {
	label string
	days  int
}{
	{"base", 0},
	{"prev", -1},
	{"week", -7},
	{"month", -28},
	{"year", -364},
}

// 지표 수집이 원격 DB 왕복 여러 번(직접 쿼리 4회 + 캐시 조회 2회, ~수 초)이라
// 요청 기준일별로 응답을 메모이즈한다. TTL(5분)이 지나면 stale 을 즉시 반환하고
// 백그라운드에서 갱신 (fetchcache 와 같은 stale-while-revalidate, single-flight).
type compareResult struct {
	baseDate     string
	weekdayLabel string
	points       []unifiedPoint
	at           time.Time
}

var (
	compareMemo       sync.Map // 요청 기준일(YYYY-MM-DD) → compareResult
	compareRefreshing sync.Map // 요청 기준일 → true (갱신 goroutine 중복 방지)
)

func (c *CompareController) Compare(dateStr string) {
	base, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		base = time.Now()
	}
	base = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, time.Local)
	memoKey := base.Format("2006-01-02")

	if v, ok := compareMemo.Load(memoKey); ok {
		cached := v.(compareResult)
		if time.Since(cached.at) >= 5*time.Minute {
			go refreshCompare(memoKey, base)
		}
		c.Set("baseDate", cached.baseDate)
		c.Set("weekdayLabel", cached.weekdayLabel)
		c.Set("points", cached.points)
		return
	}

	result := computeCompareResult(base)
	compareMemo.Store(memoKey, result)

	c.Set("baseDate", result.baseDate)
	c.Set("weekdayLabel", result.weekdayLabel)
	c.Set("points", result.points)
}

func refreshCompare(memoKey string, base time.Time) {
	if _, loaded := compareRefreshing.LoadOrStore(memoKey, true); loaded {
		return
	}
	defer compareRefreshing.Delete(memoKey)
	compareMemo.Store(memoKey, computeCompareResult(base))
}

func computeCompareResult(base time.Time) compareResult {
	conn := models.NewConnection()
	defer conn.Close()

	points := buildUnifiedPoints(conn, base)

	// 기준일 폴백: 지표 전무하면 어제 기준
	bp := points[0]
	if bp.Steps == nil && bp.ActiveEnergy == nil && bp.ExerciseMinutes == nil &&
		bp.WorkoutCount == 0 && derefZero(bp.DevCommits) == 0 && derefZero(bp.ReadingMinutes) == 0 {
		base = base.AddDate(0, 0, -1)
		points = buildUnifiedPoints(conn, base)
	}

	return compareResult{
		baseDate:     base.Format("2006-01-02"),
		weekdayLabel: weekdayKo[int(base.Weekday())],
		points:       points,
		at:           time.Now(),
	}
}

func derefZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func ptrIfValid(v float64) *float64 {
	if v < 0 {
		return nil
	}
	value := v
	return &value
}

func buildUnifiedPoints(conn *models.Connection, base time.Time) []unifiedPoint {
	dates := make([]string, len(unifiedOffsets))
	points := make([]unifiedPoint, len(unifiedOffsets))
	for i, off := range unifiedOffsets {
		d := base.AddDate(0, 0, off.days).Format("2006-01-02")
		dates[i] = d
		points[i] = unifiedPoint{Label: off.label, Date: d}
	}

	steps := queryMetricByDates(conn, "steps", dates)
	energy := queryMetricByDates(conn, "active_energy", dates)
	exercise := queryMetricByDates(conn, "exercise_minutes", dates)
	workoutMin, workoutCnt := queryWorkoutMinutes(conn, dates)
	devCommits := queryDevByDates(dates)
	readingMin, readingPages := queryReadingByDates(dates)

	for i := range points {
		points[i].Steps = ptrIfValid(steps[i])
		points[i].ActiveEnergy = ptrIfValid(energy[i])
		points[i].ExerciseMinutes = ptrIfValid(exercise[i])
		points[i].WorkoutMinutes = ptrIfValid(workoutMin[i])
		points[i].WorkoutCount = workoutCnt[i]
		points[i].DevCommits = ptrIfValid(devCommits[i])
		points[i].ReadingMinutes = ptrIfValid(readingMin[i])
		points[i].ReadingPages = ptrIfValid(readingPages[i])
	}
	return points
}
