package clients

// 식단 일지 — 노션 「식단 일지」 DB(음식 1개 = 1행) ↔ diet_tb 복제본.
// 웨이트(liftsync/liftwrite)와 같은 원칙: 노션이 원본, 쓰기는 노션에 먼저 하고 SyncDiet 로 복제본을 갈아끼운다.
// 목표(운동일/휴식일 매크로 + 기초대사량)는 노션에 속성이 없어 fetchcache_tb 의 diet_targets 키에 JSON 으로 둔다.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"dashboard/global/config"
	"dashboard/global/log"
	"dashboard/models"
)

var DietMeals = []string{"아침", "점심", "운동 전", "저녁", "간식"}

type Macros struct {
	Kcal    float64 `json:"kcal"`
	Protein float64 `json:"protein"`
	Carbs   float64 `json:"carbs"`
	Fat     float64 `json:"fat"`
}

func (m Macros) Add(o Macros) Macros {
	return Macros{m.Kcal + o.Kcal, m.Protein + o.Protein, m.Carbs + o.Carbs, m.Fat + o.Fat}
}

func (m Macros) Round() Macros {
	r := func(v float64) float64 { return math.Round(v*10) / 10 }
	return Macros{r(m.Kcal), r(m.Protein), r(m.Carbs), r(m.Fat)}
}

// ── 동기화 ────────────────────────────────────────────────────────────────

var dietSyncMutex sync.Mutex

type dietRow struct {
	notionId, date, meal, food, foodCode, memo string
	grams                                      float64
	macros                                     Macros
}

func dietRowFromPage(p NotionPage) (dietRow, bool) {
	date := p.Date("날짜")
	food := p.Text("음식")
	if date == "" || food == "" {
		return dietRow{}, false
	}
	return dietRow{
		notionId: p.Id, date: date, meal: p.Select("끼니"), food: food,
		foodCode: p.Text("식품코드"), memo: p.Text("메모"), grams: p.Number("양(g)"),
		macros: Macros{p.Number("칼로리(kcal)"), p.Number("단백질(g)"), p.Number("탄수화물(g)"), p.Number("지방(g)")},
	}, true
}

// SyncDiet 는 노션 식단 일지 전체를 읽어 diet_tb 를 트랜잭션 안에서 갈아끼운다. 노션 실패 시 DB 미변경.
func SyncDiet() error {
	dietSyncMutex.Lock()
	defer dietSyncMutex.Unlock()
	if config.NotionToken == "" {
		return errors.New("NOTION_TOKEN 미설정")
	}

	pages, err := notionQuery(config.NotionDietDS, nil, []map[string]string{{"timestamp": "created_time", "direction": "ascending"}})
	if err != nil {
		return fmt.Errorf("식단 일지: %v", err)
	}
	var rows []dietRow
	for _, p := range pages {
		if r, ok := dietRowFromPage(p); ok {
			rows = append(rows, r)
		}
	}

	conn := models.NewConnection()
	if conn == nil {
		return errors.New("db connection failed")
	}
	defer conn.Close()
	tx, err := conn.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM diet_tb"); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := tx.Exec(
			"INSERT INTO diet_tb (dt_notionid, dt_date, dt_meal, dt_food, dt_grams, dt_kcal, dt_protein, dt_carbs, dt_fat, dt_foodcode, dt_memo) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
			r.notionId, r.date, r.meal, truncateRunes(r.food, 200), r.grams,
			r.macros.Kcal, r.macros.Protein, r.macros.Carbs, r.macros.Fat, truncateRunes(r.foodCode, 40), r.memo,
		); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Info().Str("service", "diet-sync").Int("items", len(rows)).Msg("synced")
	return nil
}

// ── 쓰기 (노션) ───────────────────────────────────────────────────────────

var dietWriteMutex sync.Mutex

type DietLogInput struct {
	Id       string  `json:"id"`
	Date     string  `json:"date"`
	Meal     string  `json:"meal"`
	Food     string  `json:"food"`
	Grams    float64 `json:"grams"`
	Kcal     float64 `json:"kcal"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
	FoodCode string  `json:"foodCode"`
	Memo     string  `json:"memo"`
}

func (in DietLogInput) validate() error {
	if err := validLiftDate(in.Date); err != nil {
		return err
	}
	if !containsString(DietMeals, in.Meal) {
		return fmt.Errorf("끼니는 %s 중 하나", strings.Join(DietMeals, "/"))
	}
	if strings.TrimSpace(in.Food) == "" {
		return errors.New("음식 이름을 입력해 주세요")
	}
	for _, v := range []float64{in.Grams, in.Kcal, in.Protein, in.Carbs, in.Fat} {
		if v < 0 || v > 20000 || math.IsNaN(v) {
			return errors.New("숫자는 0 이상으로 입력해 주세요")
		}
	}
	return nil
}

func dietProps(in DietLogInput) map[string]interface{} {
	r1 := func(v float64) float64 { return math.Round(v*10) / 10 }
	return map[string]interface{}{
		"음식":        notionTitleValue(strings.TrimSpace(in.Food)),
		"날짜":        notionDateValue(in.Date),
		"끼니":        notionSelectValue(in.Meal),
		"양(g)":      notionNumberValue(r1(in.Grams)),
		"칼로리(kcal)": notionNumberValue(r1(in.Kcal)),
		"단백질(g)":    notionNumberValue(r1(in.Protein)),
		"탄수화물(g)":   notionNumberValue(r1(in.Carbs)),
		"지방(g)":     notionNumberValue(r1(in.Fat)),
		"식품코드":      notionRichTextValue(in.FoodCode),
		"메모":        notionRichTextValue(in.Memo),
	}
}

// SaveDietLog 는 음식 1개를 노션에 생성(Id 없음)·수정하고 동기화한다.
func SaveDietLog(in DietLogInput) (string, error) {
	if err := in.validate(); err != nil {
		return "", err
	}
	if config.NotionToken == "" {
		return "", errors.New("NOTION_TOKEN 미설정")
	}
	dietWriteMutex.Lock()
	defer dietWriteMutex.Unlock()

	id := in.Id
	var err error
	if id == "" {
		id, err = notionCreatePage(config.NotionDietDS, dietProps(in))
	} else {
		err = notionUpdatePage(id, dietProps(in))
	}
	if err != nil {
		return "", err
	}
	return id, SyncDiet()
}

func DeleteDietLog(id string) error {
	if id == "" {
		return errors.New("id 필요")
	}
	dietWriteMutex.Lock()
	defer dietWriteMutex.Unlock()
	if err := notionTrashPage(id); err != nil {
		return err
	}
	return SyncDiet()
}

// CopyDiet 는 from 날짜의 기록(meal 이 있으면 그 끼니만)을 to 날짜로 복사한다. 복사한 개수를 반환.
func CopyDiet(from, to, meal string) (int, error) {
	if err := validLiftDate(from); err != nil {
		return 0, err
	}
	if err := validLiftDate(to); err != nil {
		return 0, err
	}
	conn := models.NewConnection()
	if conn == nil {
		return 0, errors.New("db connection failed")
	}
	q := "SELECT dt_meal, dt_food, dt_grams, dt_kcal, dt_protein, dt_carbs, dt_fat, dt_foodcode, dt_memo FROM diet_tb WHERE dt_date = ?"
	args := []interface{}{from}
	if meal != "" {
		q += " AND dt_meal = ?"
		args = append(args, meal)
	}
	rows, err := conn.Query(q+" ORDER BY dt_id", args...)
	if err != nil {
		conn.Close()
		return 0, err
	}
	var items []DietLogInput
	for rows.Next() {
		it := DietLogInput{Date: to}
		if rows.Scan(&it.Meal, &it.Food, &it.Grams, &it.Kcal, &it.Protein, &it.Carbs, &it.Fat, &it.FoodCode, &it.Memo) == nil {
			items = append(items, it)
		}
	}
	rows.Close()
	conn.Close()
	if len(items) == 0 {
		return 0, errors.New("복사할 기록이 없습니다")
	}

	dietWriteMutex.Lock()
	defer dietWriteMutex.Unlock()
	for i, it := range items {
		if !containsString(DietMeals, it.Meal) {
			it.Meal = "간식"
		}
		if _, err := notionCreatePage(config.NotionDietDS, dietProps(it)); err != nil {
			SyncDiet() // 일부만 복사됐어도 화면엔 반영
			return i, err
		}
	}
	return len(items), SyncDiet()
}

// ── 목표 (fetchcache_tb diet_targets) ────────────────────────────────────

const dietTargetsKey = "diet_targets"

type DietTargets struct {
	Training Macros  `json:"training"`
	Rest     Macros  `json:"rest"`
	Bmr      float64 `json:"bmr"`
}

// 노션 「운동 계획 — 4분할 (감량기)」 식단 목표 수치
var defaultDietTargets = DietTargets{
	Training: Macros{Kcal: 1950, Protein: 160, Carbs: 200, Fat: 55},
	Rest:     Macros{Kcal: 1750, Protein: 160, Carbs: 150, Fat: 55},
	Bmr:      1693,
}

func GetDietTargets(conn *models.Connection) DietTargets {
	item := models.NewFetchcacheManager(conn).GetWhere([]interface{}{models.Where{Column: "cachekey", Value: dietTargetsKey, Compare: "="}})
	if item == nil || item.Payload == "" {
		return defaultDietTargets
	}
	t := defaultDietTargets
	if err := json.Unmarshal([]byte(item.Payload), &t); err != nil {
		return defaultDietTargets
	}
	return t
}

func SaveDietTargets(t DietTargets) error {
	for _, m := range []Macros{t.Training, t.Rest} {
		if m.Kcal <= 0 || m.Kcal > 10000 || m.Protein < 0 || m.Carbs < 0 || m.Fat < 0 {
			return errors.New("목표 값을 확인해 주세요")
		}
	}
	if t.Bmr <= 0 || t.Bmr > 5000 {
		return errors.New("기초대사량을 확인해 주세요")
	}
	buf, _ := json.Marshal(t)
	conn := models.NewConnection()
	if conn == nil {
		return errors.New("db connection failed")
	}
	defer conn.Close()
	manager := models.NewFetchcacheManager(conn)
	now := time.Now().Format("2006-01-02 15:04:05")
	item := manager.GetWhere([]interface{}{models.Where{Column: "cachekey", Value: dietTargetsKey, Compare: "="}})
	if item == nil {
		return manager.Insert(&models.Fetchcache{Cachekey: dietTargetsKey, Payload: string(buf), Fetchedat: now})
	}
	item.Payload = string(buf)
	item.Fetchedat = now
	return manager.Update(item)
}
