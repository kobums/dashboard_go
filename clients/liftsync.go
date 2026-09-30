package clients

// 노션 운동 기록 → liftday_tb / liftexercise_tb / liftset_tb 전체 재동기화.
// 노션이 원본이고 이 테이블들은 복제본이다. 매번 전부 다시 가져와 트랜잭션 안에서
// DELETE → INSERT 로 갈아끼운다 — 노션에서의 수정·삭제가 별도 로직 없이 반영된다.
// (하루 10행 안팎이라 몇 년치도 수십 번의 API 호출이면 충분)
// 노션 호출이 하나라도 실패하면 DB 는 건드리지 않는다.

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"dashboard/global/config"
	"dashboard/global/log"
	"dashboard/models"
)

type LiftSyncStatus struct {
	Configured bool   `json:"configured"`
	LastSyncAt string `json:"lastSyncAt"`
	Ok         bool   `json:"ok"`
	Error      string `json:"error"`
	Days       int    `json:"days"`
	Exercises  int    `json:"exercises"`
	Sets       int    `json:"sets"`
}

var liftSyncMutex sync.Mutex
var liftStatus LiftSyncStatus
var liftStatusMutex sync.RWMutex

func GetLiftSyncStatus() LiftSyncStatus {
	liftStatusMutex.RLock()
	defer liftStatusMutex.RUnlock()
	s := liftStatus
	s.Configured = config.NotionToken != ""
	return s
}

func setLiftStatus(s LiftSyncStatus) {
	liftStatusMutex.Lock()
	liftStatus = s
	liftStatusMutex.Unlock()
}

type liftDayRow struct {
	notionId, date, title, parts, summary, cardio, condition string
}

type liftExerciseRow struct {
	notionId, dayNotionId, date, name, part  string
	warmup, mainset, target, condition, memo string
	minutes, speed, incline, distance        float64
	parseError                               string
	sets                                     []LiftSet // 웜업 먼저, 본세트 뒤 (warmupCount 로 구분)
	warmupCount                              int
}

// SyncLift 는 노션에서 전부 읽어 DB 를 갈아끼운다. 동시 실행은 직렬화한다.
func SyncLift() (LiftSyncStatus, error) {
	liftSyncMutex.Lock()
	defer liftSyncMutex.Unlock()

	status := LiftSyncStatus{Configured: config.NotionToken != "", LastSyncAt: time.Now().Format("2006-01-02 15:04:05")}
	if !status.Configured {
		status.Error = "NOTION_TOKEN 미설정"
		setLiftStatus(status)
		return status, errors.New(status.Error)
	}

	days, exercises, err := fetchLift()
	if err == nil {
		err = saveLift(days, exercises)
	}
	if err != nil {
		status.Error = err.Error()
		setLiftStatus(status)
		log.Error().Str("service", "lift-sync").Msg(err.Error())
		return status, err
	}

	status.Ok = true
	status.Days = len(days)
	status.Exercises = len(exercises)
	for _, e := range exercises {
		status.Sets += len(e.sets)
	}
	setLiftStatus(status)
	log.Info().Str("service", "lift-sync").Int("days", status.Days).Int("exercises", status.Exercises).Int("sets", status.Sets).Msg("synced")
	return status, nil
}

func fetchLift() ([]liftDayRow, []liftExerciseRow, error) {
	dayPages, err := notionQueryAll(config.NotionDayDS)
	if err != nil {
		return nil, nil, fmt.Errorf("운동 캘린더: %v", err)
	}
	// 생성 순으로 받아 le_id 순서 = 기록 순서가 되게 한다 (입력 화면의 그날 목록 순서)
	logPages, err := notionQuery(config.NotionLogDS, nil, []map[string]string{{"timestamp": "created_time", "direction": "ascending"}})
	if err != nil {
		return nil, nil, fmt.Errorf("운동 일지: %v", err)
	}

	var days []liftDayRow
	for _, p := range dayPages {
		if d, ok := liftDayFromPage(p); ok {
			days = append(days, d)
		}
	}
	var exercises []liftExerciseRow
	for _, p := range logPages {
		if e, ok := liftExerciseFromPage(p); ok {
			exercises = append(exercises, e)
		}
	}
	return days, exercises, nil
}

// liftDayFromPage 는 운동 캘린더 페이지 → 행. 날짜 없는 페이지(작성 중)는 건너뛴다.
func liftDayFromPage(p NotionPage) (liftDayRow, bool) {
	date := p.Date("날짜")
	if date == "" {
		return liftDayRow{}, false
	}
	return liftDayRow{
		notionId:  p.Id,
		date:      date,
		title:     p.Text("제목"),
		parts:     strings.Join(p.MultiSelect("부위"), ","),
		summary:   p.Text("요약"),
		cardio:    p.Text("유산소"),
		condition: p.Select("컨디션"),
	}, true
}

// liftExerciseFromPage 는 운동 일지 페이지 → 행 + 파싱된 세트. 형식 오류는 parseError 에 남기고 행은 살린다.
func liftExerciseFromPage(p NotionPage) (liftExerciseRow, bool) {
	date := p.Date("날짜")
	name := p.Select("종목")
	if name == "" {
		name = p.Text("운동")
	}
	if date == "" || name == "" {
		return liftExerciseRow{}, false
	}
	e := liftExerciseRow{
		notionId:  p.Id,
		date:      date,
		name:      name,
		part:      p.Select("부위"),
		warmup:    p.Text("웜업"),
		mainset:   p.Text("본세트"),
		target:    p.Text("목표"),
		condition: p.Select("컨디션"),
		memo:      p.Text("메모"),
		minutes:   p.Number("시간(분)"),
		speed:     p.Number("속도(km/h)"),
		incline:   p.Number("경사"),
		distance:  p.Number("거리(km)"),
	}
	if rel := p.Relation("운동일"); len(rel) > 0 {
		e.dayNotionId = rel[0]
	}

	var errs []string
	warm, err := ParseSets(e.warmup)
	if err != nil {
		errs = append(errs, "웜업 "+err.Error())
	}
	main, err := ParseSets(e.mainset)
	if err != nil {
		errs = append(errs, "본세트 "+err.Error())
	}
	if _, err := ParseSets(e.target); err != nil {
		errs = append(errs, "목표 "+err.Error())
	}
	// 근력 종목인데 본세트가 비면 0세트로 조용히 집계된다 — 옛 형식(메모·무게/세트/횟수 칸)으로 적은 행을 드러낸다
	if e.part != liftCardioPart && e.mainset == "" {
		errs = append(errs, "본세트 비어 있음 (세트를 메모나 무게/세트/횟수 칸에만 적었는지 확인)")
	}
	e.parseError = truncateRunes(strings.Join(errs, " · "), 290)
	e.sets = append(warm, main...)
	e.warmupCount = len(warm)
	return e, true
}

func saveLift(days []liftDayRow, exercises []liftExerciseRow) error {
	conn := models.NewConnection()
	if conn == nil {
		return fmt.Errorf("db connection failed")
	}
	defer conn.Close()

	tx, err := conn.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // Commit 후엔 no-op

	for _, q := range []string{"DELETE FROM liftset_tb", "DELETE FROM liftexercise_tb", "DELETE FROM liftday_tb"} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}

	for _, d := range days {
		if _, err := tx.Exec(
			"INSERT INTO liftday_tb (ld_notionid, ld_date, ld_title, ld_parts, ld_summary, ld_cardio, ld_condition) VALUES (?,?,?,?,?,?,?)",
			d.notionId, d.date, truncateRunes(d.title, 200), truncateRunes(d.parts, 200), d.summary, truncateRunes(d.cardio, 500), d.condition,
		); err != nil {
			return err
		}
	}

	for _, e := range exercises {
		if _, err := tx.Exec(
			"INSERT INTO liftexercise_tb (le_notionid, le_daynotionid, le_date, le_name, le_part, le_warmup, le_mainset, le_target, "+
				"le_minutes, le_speed, le_incline, le_distance, le_condition, le_memo, le_parseerror) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			e.notionId, e.dayNotionId, e.date, truncateRunes(e.name, 100), e.part,
			truncateRunes(e.warmup, 500), truncateRunes(e.mainset, 500), truncateRunes(e.target, 200),
			e.minutes, e.speed, e.incline, e.distance, e.condition, e.memo, e.parseError,
		); err != nil {
			return err
		}
		for i, s := range e.sets {
			if _, err := tx.Exec(
				"INSERT INTO liftset_tb (ls_exercisenotionid, ls_date, ls_order, ls_warmup, ls_bodyweight, ls_weight, ls_reps) VALUES (?,?,?,?,?,?,?)",
				e.notionId, e.date, i, i < e.warmupCount, s.Bodyweight, s.Weight, s.Reps,
			); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
