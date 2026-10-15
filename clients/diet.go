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

// SyncDiet 는 노션 「식단 기록」 전체를 읽어 diet_tb 를 트랜잭션 안에서 갈아끼운다. 노션 실패 시 DB 미변경.
func SyncDiet() error {
	dietSyncMutex.Lock()
	defer dietSyncMutex.Unlock()
	if config.NotionToken == "" {
		return errors.New("NOTION_TOKEN 미설정")
	}

	rows, err := fetchDietDayRows()
	if err != nil {
		return fmt.Errorf("식단 기록: %v", err)
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

// SaveDietLog 는 음식 1개를 노션 「식단 기록」 그날 표에 추가(Id 없음)·수정하고 동기화한다.
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
		err = addDietDayItems(in.Date, []DietLogInput{in})
	} else if rowId, ok := dietDayRowBlockId(id); ok {
		err = updateDietDayItem(rowId, in)
	} else {
		return "", errors.New(dietReadonlyMsg)
	}
	if err != nil {
		SyncDiet() // 일부만 쓰였어도 화면엔 반영
		return "", err
	}
	return id, SyncDiet()
}

const dietReadonlyMsg = "이 항목(합계 보정·끼니 메모)은 노션 「식단 기록」 페이지에서 고쳐 주세요"

func DeleteDietLog(id string) error {
	if id == "" {
		return errors.New("id 필요")
	}
	rowId, ok := dietDayRowBlockId(id)
	if !ok {
		return errors.New(dietReadonlyMsg)
	}
	dietWriteMutex.Lock()
	defer dietWriteMutex.Unlock()
	if err := deleteDietDayItem(rowId); err != nil {
		SyncDiet()
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
	q := "SELECT dt_meal, dt_food, dt_grams, dt_kcal, dt_protein, dt_carbs, dt_fat, dt_foodcode, dt_memo FROM diet_tb WHERE dt_date = ? AND dt_food <> ?"
	args := []interface{}{from, dietDayAdjustFood}
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
	for i := range items {
		if !containsString(DietMeals, items[i].Meal) {
			items[i].Meal = "간식"
		}
	}
	if err := addDietDayItems(to, items); err != nil {
		SyncDiet() // 일부만 복사됐어도 화면엔 반영
		return 0, err
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
