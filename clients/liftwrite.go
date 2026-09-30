package clients

// 대시보드 → 노션 운동 기록 쓰기. 노션이 원본이므로 항상 노션에 먼저 쓰고 SyncLift 로 복제본을 갱신한다
// (lift*_tb 에 직접 쓰면 다음 전체 동기화에 지워진다).
//
// 종목 1개 저장 흐름: 형식 검증 → 그날 「운동 캘린더」 페이지 찾기/만들기 → 「운동 일지」 행 생성·수정
// → 그날 페이지의 부위/요약/유산소를 일지 행들로 다시 계산 → 전체 동기화.
// 쓰기는 한 번에 하나씩(liftWriteMutex) — 빠른 연타로 같은 날 캘린더 페이지가 두 개 생기지 않게.

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"dashboard/global/config"
)

var liftWriteMutex sync.Mutex

var liftStrengthParts = []string{"가슴", "등", "어깨·팔", "하체"}

const liftCardioPart = "유산소"

type LiftLogInput struct {
	Id        string  `json:"id"`
	Date      string  `json:"date"`
	Name      string  `json:"name"`
	Part      string  `json:"part"`
	Warmup    string  `json:"warmup"`
	Main      string  `json:"main"`
	Target    string  `json:"target"`
	Minutes   float64 `json:"minutes"`
	Speed     float64 `json:"speed"`
	Incline   float64 `json:"incline"`
	Distance  float64 `json:"distance"`
	Condition string  `json:"condition"`
	Memo      string  `json:"memo"`
}

func validLiftDate(date string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("날짜 형식은 YYYY-MM-DD")
	}
	return nil
}

// SaveLiftLog 는 종목 1개를 노션에 생성(Id 없음) 또는 수정하고 동기화한다.
func SaveLiftLog(in LiftLogInput) (string, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Warmup = strings.TrimSpace(in.Warmup)
	in.Main = strings.TrimSpace(in.Main)
	in.Target = strings.TrimSpace(in.Target)
	if err := validLiftDate(in.Date); err != nil {
		return "", err
	}
	if in.Name == "" {
		return "", fmt.Errorf("종목을 골라 주세요")
	}
	for label, text := range map[string]string{"웜업": in.Warmup, "본세트": in.Main, "목표": in.Target} {
		if _, err := ParseSets(text); err != nil {
			return "", fmt.Errorf("%s %v", label, err)
		}
	}
	if config.NotionToken == "" {
		return "", fmt.Errorf("NOTION_TOKEN 미설정")
	}

	liftWriteMutex.Lock()
	defer liftWriteMutex.Unlock()

	// 수정인데 날짜가 바뀌면 옛 날짜의 캘린더 페이지도 다시 계산해야 한다
	oldDayId := ""
	if in.Id != "" {
		old, err := notionGetPage(in.Id)
		if err != nil {
			return "", err
		}
		if rel := old.Relation("운동일"); len(rel) > 0 {
			oldDayId = rel[0]
		}
	}

	dayId, err := ensureLiftDay(in.Date, in.Part)
	if err != nil {
		return "", err
	}

	props := map[string]interface{}{
		"운동":       notionTitleValue(in.Name),
		"종목":       notionSelectValue(in.Name),
		"부위":       notionSelectValue(in.Part),
		"날짜":       notionDateValue(in.Date),
		"웜업":       notionRichTextValue(in.Warmup),
		"본세트":      notionRichTextValue(in.Main),
		"목표":       notionRichTextValue(in.Target),
		"시간(분)":    notionNumberValue(in.Minutes),
		"속도(km/h)": notionNumberValue(in.Speed),
		"경사":       notionNumberValue(in.Incline),
		"거리(km)":   notionNumberValue(in.Distance),
		"메모":       notionRichTextValue(in.Memo),
		"운동일":      notionRelationValue(dayId),
	}

	// 컨디션은 입력 화면이 날짜 단위(/lift/day)로 다루므로, 값이 올 때만 쓴다 (수정 시 노션에서 적은 값 보존)
	if in.Condition != "" {
		props["컨디션"] = notionSelectValue(in.Condition)
	}

	id := in.Id
	if id == "" {
		if id, err = notionCreatePage(config.NotionLogDS, props); err != nil {
			return "", err
		}
	} else if err := notionUpdatePage(id, props); err != nil {
		return "", err
	}

	if err := refreshLiftDay(dayId); err != nil {
		return id, err
	}
	if oldDayId != "" && oldDayId != dayId {
		if err := refreshLiftDay(oldDayId); err != nil {
			return id, err
		}
	}

	_, err = SyncLift()
	return id, err
}

// DeleteLiftLog 는 일지 행을 휴지통으로 보내고, 그날 행이 하나도 안 남으면 캘린더 페이지도 휴지통으로 보낸다.
func DeleteLiftLog(id string) error {
	if id == "" {
		return fmt.Errorf("id 필요")
	}
	liftWriteMutex.Lock()
	defer liftWriteMutex.Unlock()

	page, err := notionGetPage(id)
	if err != nil {
		return err
	}
	if err := notionTrashPage(id); err != nil {
		return err
	}
	if rel := page.Relation("운동일"); len(rel) > 0 {
		if err := refreshLiftDay(rel[0]); err != nil {
			return err
		}
	}
	_, err = SyncLift()
	return err
}

// SetLiftDayCondition 은 그날 캘린더 페이지의 컨디션을 바꾼다 (페이지가 없으면 만든다).
func SetLiftDayCondition(date, condition string) error {
	if err := validLiftDate(date); err != nil {
		return err
	}
	liftWriteMutex.Lock()
	defer liftWriteMutex.Unlock()

	dayId, err := ensureLiftDay(date, "")
	if err != nil {
		return err
	}
	if err := notionUpdatePage(dayId, map[string]interface{}{"컨디션": notionSelectValue(condition)}); err != nil {
		return err
	}
	_, err = SyncLift()
	return err
}

// liftDayTitle 은 부위로 만든 기본 제목 — "어깨·팔 + 가슴", 유산소만이면 "유산소".
func liftDayTitle(parts []string) string {
	var strength []string
	for _, p := range parts {
		if p != liftCardioPart {
			strength = append(strength, p)
		}
	}
	if len(strength) == 0 && len(parts) > 0 {
		return liftCardioPart
	}
	return strings.Join(strength, " + ")
}

func findLiftDay(date string) (*NotionPage, error) {
	pages, err := notionQuery(config.NotionDayDS,
		map[string]interface{}{"property": "날짜", "date": map[string]string{"equals": date}},
		[]map[string]string{{"timestamp": "created_time", "direction": "ascending"}})
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, nil
	}
	return &pages[0], nil // 같은 날 페이지가 여럿이면 먼저 만든 것
}

// ensureLiftDay 는 그날 캘린더 페이지 id 를 돌려준다. 없으면 만들고, part 가 부위에 없으면 추가한다.
func ensureLiftDay(date, part string) (string, error) {
	day, err := findLiftDay(date)
	if err != nil {
		return "", err
	}
	if day == nil {
		parts := []string{}
		if part != "" {
			parts = append(parts, part)
		}
		return notionCreatePage(config.NotionDayDS, map[string]interface{}{
			"제목": notionTitleValue(liftDayTitle(parts)),
			"날짜": notionDateValue(date),
			"부위": notionMultiSelectValue(parts),
		})
	}
	return day.Id, nil // 부위·제목은 refreshLiftDay 가 일지 행 기준으로 맞춘다
}

// refreshLiftDay 는 그날 일지 행들로 캘린더 페이지의 부위/요약/유산소를 다시 쓴다.
// 제목은 사용자가 바꾸지 않았을 때(= 기존 부위로 만든 기본 제목이거나 비었을 때)만 따라 바꾼다.
// 행이 하나도 없으면 페이지를 휴지통으로 보낸다.
func refreshLiftDay(dayId string) error {
	day, err := notionGetPage(dayId)
	if err != nil {
		return err
	}
	rows, err := notionQuery(config.NotionLogDS,
		map[string]interface{}{"property": "운동일", "relation": map[string]string{"contains": dayId}},
		[]map[string]string{{"timestamp": "created_time", "direction": "ascending"}})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return notionTrashPage(dayId)
	}

	var parts, summary, cardio []string
	for _, r := range rows {
		name := r.Select("종목")
		if name == "" {
			name = r.Text("운동")
		}
		part := r.Select("부위")
		if part != "" && !containsString(parts, part) {
			parts = append(parts, part)
		}
		if part == liftCardioPart {
			cardio = append(cardio, cardioText(name, r))
		} else if main := r.Text("본세트"); main != "" {
			summary = append(summary, name+" "+main)
		}
	}
	// 4분할 순서로 정렬, 유산소는 끝
	ordered := []string{}
	for _, p := range append(append([]string{}, liftStrengthParts...), liftCardioPart) {
		if containsString(parts, p) {
			ordered = append(ordered, p)
		}
	}
	for _, p := range parts {
		if !containsString(ordered, p) {
			ordered = append(ordered, p)
		}
	}

	props := map[string]interface{}{
		"부위":  notionMultiSelectValue(ordered),
		"요약":  notionRichTextValue(truncateRunes(strings.Join(summary, " · "), 1900)),
		"유산소": notionRichTextValue(truncateRunes(strings.Join(cardio, " · "), 1900)),
	}
	oldTitle := day.Text("제목")
	if oldTitle == "" || oldTitle == liftDayTitle(day.MultiSelect("부위")) {
		props["제목"] = notionTitleValue(liftDayTitle(ordered))
	}
	return notionUpdatePage(dayId, props)
}

func cardioText(name string, r NotionPage) string {
	parts := []string{name}
	if v := r.Number("시간(분)"); v > 0 {
		parts = append(parts, fmt.Sprintf("%g분", v))
	}
	if v := r.Number("속도(km/h)"); v > 0 {
		parts = append(parts, fmt.Sprintf("%gkm/h", v))
	}
	if v := r.Number("경사"); v > 0 {
		parts = append(parts, fmt.Sprintf("경사 %g", v))
	}
	if v := r.Number("거리(km)"); v > 0 {
		parts = append(parts, fmt.Sprintf("%gkm", v))
	}
	return strings.Join(parts, " · ")
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ── 입력 화면용 종목 목록 (노션 종목 select 옵션, 10분 캐시) ─────────────────

// 노션 종목 옵션 색 → 부위 (DB 에 옵션을 부위 색으로 만들어 둠)
var liftColorPart = map[string]string{"red": "가슴", "blue": "등", "orange": "어깨·팔", "green": "하체", "gray": "유산소"}

type LiftCatalogOption struct {
	Name string
	Part string
}

var liftCatalogCache []LiftCatalogOption
var liftCatalogAt time.Time
var liftCatalogMutex sync.Mutex

// LiftCatalogOptions 는 종목 select 옵션을 부위와 함께 반환한다. 노션 실패 시 마지막 캐시(없으면 nil).
func LiftCatalogOptions() []LiftCatalogOption {
	liftCatalogMutex.Lock()
	defer liftCatalogMutex.Unlock()
	if config.NotionToken == "" || (liftCatalogCache != nil && time.Since(liftCatalogAt) < 10*time.Minute) {
		return liftCatalogCache
	}
	opts, err := notionSelectOptions(config.NotionLogDS, "종목")
	if err != nil {
		return liftCatalogCache
	}
	list := make([]LiftCatalogOption, 0, len(opts))
	for _, o := range opts {
		list = append(list, LiftCatalogOption{Name: o.Name, Part: liftColorPart[o.Color]})
	}
	liftCatalogCache, liftCatalogAt = list, time.Now()
	return list
}
