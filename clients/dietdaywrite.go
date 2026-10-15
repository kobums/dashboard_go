package clients

// 식단 기록 쓰기 — 대시보드 입력(추가·수정·삭제·복사)을 노션 「식단 기록」 그날 페이지의 본문 표에 직접 쓴다.
// 노션과 대시보드가 같은 DB 를 본다: 대시보드에서 넣은 음식도 노션 표의 한 행이고, 노션에서 고친 행도 다음 동기화에 반영된다.
//   - 추가: 그날 페이지(없으면 생성)의 마지막 표에서 마지막 음식 행 뒤에 「[끼니] 음식 (Ng) | kcal | 단백질 | 탄 | 지」 행
//   - 수정·삭제: 표 행 블록 id(diet_tb id = "r:"+블록id)로 그 행만 고친다 — 다른 행이 바뀌어도 id 가 흔들리지 않는다
//   - 쓴 뒤에는 표의 합계 행(누적)·페이지 하루 합계 속성·바뀐 끼니 텍스트 속성을 음식 행으로 다시 계산한다
// 식품코드·메모 칸은 표에 없어 저장하지 않는다.

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"dashboard/global/config"
)

const dietDayRowPrefix = "r:"

// 합계 행 머리 — 「합계 (점심까지)」「하루 합계」「최종 하루 합계」「야식 포함 최종 합계」
var dietDayTotalRe = regexp.MustCompile(`^\**\s*(합계|하루 합계|최종|야식 포함)`)

// 끼니 텍스트 속성이 있는 끼니 (운동 전은 속성이 없다)
var dietDayMealProps = []string{"아침", "점심", "저녁", "간식"}

func dietDayRowBlockId(id string) (string, bool) {
	if strings.HasPrefix(id, dietDayRowPrefix) {
		return strings.TrimPrefix(id, dietDayRowPrefix), true
	}
	return "", false
}

// fmtDietNum 은 노션 표 표기(천 단위 쉼표, 소수 1자리까지)로 쓴다: 2445 → "2,445", 1.25 → "1.3"
func fmtDietNum(v float64) string {
	v = math.Round(v*10) / 10
	whole, frac := math.Modf(math.Abs(v))
	s := fmt.Sprintf("%d", int64(whole))
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if frac >= 0.05 {
		s += fmt.Sprintf("%.1f", frac)[1:]
	}
	if v < 0 {
		s = "-" + s
	}
	return s
}

func dietDayFoodText(in DietLogInput) string {
	food := strings.TrimSpace(in.Food)
	if in.Grams > 0 {
		food += " (" + fmtDietNum(in.Grams) + "g)"
	}
	return "[" + in.Meal + "] " + food
}

// dietDayCells 는 표 너비·열 위치에 맞춰 음식 행 셀을 만든다.
func dietDayCells(in DietLogInput, width int, cols dietDayCols) []string {
	cells := make([]string, width)
	cells[0] = dietDayFoodText(in)
	set := func(i int, s string) {
		if i >= 0 && i < width {
			cells[i] = s
		}
	}
	set(cols.kcal, fmtDietNum(in.Kcal)+"kcal")
	set(cols.protein, fmtDietNum(in.Protein)+"g")
	set(cols.carbs, fmtDietNum(in.Carbs)+"g")
	set(cols.fat, fmtDietNum(in.Fat)+"g")
	return cells
}

func notionCellsValue(cells []string, bold bool) [][]map[string]interface{} {
	out := make([][]map[string]interface{}, len(cells))
	for i, c := range cells {
		out[i] = []map[string]interface{}{}
		if c != "" {
			out[i] = append(out[i], map[string]interface{}{
				"type": "text", "text": map[string]string{"content": c}, "annotations": map[string]bool{"bold": bold},
			})
		}
	}
	return out
}

func notionTableRow(cells []string, bold bool) map[string]interface{} {
	return map[string]interface{}{"object": "block", "type": "table_row",
		"table_row": map[string]interface{}{"cells": notionCellsValue(cells, bold)}}
}

// ── 페이지 읽기 ───────────────────────────────────────────────────────────

type dietDayRowBlock struct {
	id    string
	cells []string
}

type dietDayTable struct {
	id    string
	width int
	cols  dietDayCols
	rows  []dietDayRowBlock // 0 = 머리행
}

func loadDietDayTables(pageId string) ([]dietDayTable, error) {
	blocks, err := notionChildren(pageId)
	if err != nil {
		return nil, err
	}
	tables := []dietDayTable{}
	for _, b := range blocks {
		if b.Type != "table" {
			continue
		}
		rows, err := notionChildren(b.Id)
		if err != nil {
			return nil, err
		}
		t := dietDayTable{id: b.Id, width: b.Table.TableWidth, cols: defaultDietDayCols}
		for i, r := range rows {
			cells := make([]string, len(r.TableRow.Cells))
			for j, c := range r.TableRow.Cells {
				cells[j] = joinRichText(c)
			}
			if i == 0 {
				t.cols = parseDietDayHeader(cells)
			}
			t.rows = append(t.rows, dietDayRowBlock{id: r.Id, cells: cells})
		}
		tables = append(tables, t)
	}
	return tables, nil
}

func findDietDayPage(date string) (*NotionPage, error) {
	pages, err := notionQuery(config.NotionDietDayDS,
		map[string]interface{}{"property": "날짜", "date": map[string]string{"equals": date}},
		[]map[string]string{{"timestamp": "created_time", "direction": "ascending"}})
	if err != nil || len(pages) == 0 {
		return nil, err
	}
	return &pages[0], nil
}

var weekdayShortKo = []string{"일", "월", "화", "수", "목", "금", "토"}

func ensureDietDayPage(date string) (string, error) {
	p, err := findDietDayPage(date)
	if err != nil {
		return "", err
	}
	if p != nil {
		return p.Id, nil
	}
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return "", err
	}
	title := fmt.Sprintf("%d/%d (%s)", int(t.Month()), t.Day(), weekdayShortKo[t.Weekday()])
	return notionCreatePage(config.NotionDietDayDS, map[string]interface{}{
		"제목": notionTitleValue(title),
		"날짜": notionDateValue(date),
	})
}

// ── 쓰기 ──────────────────────────────────────────────────────────────────

var dietDayHeader = []string{"음식", "칼로리", "단백질", "탄수화물", "지방"}

// appendDietDayItems 는 그날 페이지 마지막 표의 마지막 음식 행 뒤에 음식들을 넣는다 (표가 없으면 새 표).
func appendDietDayItems(pageId string, items []DietLogInput) error {
	tables, err := loadDietDayTables(pageId)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		cols := parseDietDayHeader(dietDayHeader)
		rows := []interface{}{notionTableRow(dietDayHeader, false)}
		for _, in := range items {
			rows = append(rows, notionTableRow(dietDayCells(in, len(dietDayHeader), cols), false))
		}
		rows = append(rows, notionTableRow([]string{"하루 합계", "", "", "", ""}, true)) // 값은 recompute 가 채운다
		return notionDo("PATCH", "/v1/blocks/"+pageId+"/children", map[string]interface{}{
			"children": []interface{}{map[string]interface{}{"object": "block", "type": "table",
				"table": map[string]interface{}{"table_width": len(dietDayHeader), "has_column_header": true, "children": rows}}},
		}, &struct{}{})
	}

	t := tables[len(tables)-1]
	after := t.rows[0].id
	for _, r := range t.rows[1:] {
		if _, ok := parseDietDayRow(r.cells, t.cols); ok {
			after = r.id
		}
	}
	rows := []interface{}{}
	for _, in := range items {
		rows = append(rows, notionTableRow(dietDayCells(in, t.width, t.cols), false))
	}
	return notionDo("PATCH", "/v1/blocks/"+t.id+"/children", map[string]interface{}{"children": rows, "after": after}, &struct{}{})
}

// recomputeDietDayPage 는 음식 행으로 합계 행(누적)·하루 합계 속성·끼니 텍스트(touched 끼니만)를 다시 쓴다.
func recomputeDietDayPage(pageId string, touched map[string]bool) error {
	tables, err := loadDietDayTables(pageId)
	if err != nil {
		return err
	}
	cum := Macros{}
	mealFoods := map[string][]string{}
	for _, t := range tables {
		for _, r := range t.rows[1:] {
			if it, ok := parseDietDayRow(r.cells, t.cols); ok {
				cum = cum.Add(Macros{Kcal: it.kcal, Protein: it.protein, Carbs: it.carbs, Fat: it.fat})
				mealFoods[it.meal] = append(mealFoods[it.meal], it.food)
				continue
			}
			if !dietDayTotalRe.MatchString(r.cells[0]) {
				continue
			}
			want := append([]string{}, r.cells...)
			set := func(i int, s string) {
				if i > 0 && i < len(want) {
					want[i] = s
				}
			}
			set(t.cols.kcal, "약 "+fmtDietNum(cum.Kcal)+"kcal")
			set(t.cols.protein, "약 "+fmtDietNum(cum.Protein)+"g")
			set(t.cols.carbs, "약 "+fmtDietNum(cum.Carbs)+"g")
			set(t.cols.fat, "약 "+fmtDietNum(cum.Fat)+"g")
			if strings.Join(want, "|") != strings.Join(r.cells, "|") {
				if err := notionDo("PATCH", "/v1/blocks/"+r.id, map[string]interface{}{
					"table_row": map[string]interface{}{"cells": notionCellsValue(want, true)},
				}, &struct{}{}); err != nil {
					return err
				}
			}
		}
	}

	props := map[string]interface{}{
		"칼로리(kcal)": notionNumberValue(cum.Round().Kcal),
		"단백질(g)":    notionNumberValue(cum.Round().Protein),
		"탄수화물(g)":   notionNumberValue(cum.Round().Carbs),
		"지방(g)":     notionNumberValue(cum.Round().Fat),
	}
	for _, meal := range dietDayMealProps {
		if touched[meal] {
			props[meal] = notionRichTextValue(strings.Join(mealFoods[meal], ", "))
		}
	}
	dietDayCache.Delete(pageId)
	return notionUpdatePage(pageId, props)
}

type notionBlockParent struct {
	Parent struct {
		Type    string `json:"type"`
		PageId  string `json:"page_id"`
		BlockId string `json:"block_id"`
	} `json:"parent"`
	TableRow struct {
		Cells [][]notionRichText `json:"cells"`
	} `json:"table_row"`
}

// dietDayRowContext 는 표 행 블록의 (페이지 id, 페이지 날짜, 기존 끼니)를 찾는다.
func dietDayRowContext(rowId string) (pageId, date, meal string, err error) {
	var row notionBlockParent
	if err = notionDo("GET", "/v1/blocks/"+rowId, nil, &row); err != nil {
		return
	}
	var table notionBlockParent
	if err = notionDo("GET", "/v1/blocks/"+row.Parent.BlockId, nil, &table); err != nil {
		return
	}
	pageId = table.Parent.PageId
	if pageId == "" {
		err = errors.New("식단 기록 표가 페이지 바로 아래에 있지 않습니다")
		return
	}
	p, err := notionGetPage(pageId)
	if err != nil {
		return
	}
	date = p.Date("날짜")
	if len(row.TableRow.Cells) > 0 {
		if m := dietDayTagRe.FindStringSubmatch(joinRichText(row.TableRow.Cells[0])); m != nil {
			meal = dietDayMeal(m[1])
		}
	}
	return
}

func addDietDayItems(date string, items []DietLogInput) error {
	pageId, err := ensureDietDayPage(date)
	if err != nil {
		return err
	}
	if err := appendDietDayItems(pageId, items); err != nil {
		return err
	}
	touched := map[string]bool{}
	for _, in := range items {
		touched[in.Meal] = true
	}
	return recomputeDietDayPage(pageId, touched)
}

func updateDietDayItem(rowId string, in DietLogInput) error {
	pageId, date, oldMeal, err := dietDayRowContext(rowId)
	if err != nil {
		return err
	}
	if date != in.Date { // 날짜를 옮기면 그날 페이지로 이동
		if err := notionDo("DELETE", "/v1/blocks/"+rowId, nil, &struct{}{}); err != nil {
			return err
		}
		if err := recomputeDietDayPage(pageId, map[string]bool{oldMeal: true}); err != nil {
			return err
		}
		return addDietDayItems(in.Date, []DietLogInput{in})
	}
	tables, err := loadDietDayTables(pageId)
	if err != nil {
		return err
	}
	for _, t := range tables {
		for _, r := range t.rows {
			if r.id != rowId {
				continue
			}
			if err := notionDo("PATCH", "/v1/blocks/"+rowId, map[string]interface{}{
				"table_row": map[string]interface{}{"cells": notionCellsValue(dietDayCells(in, t.width, t.cols), false)},
			}, &struct{}{}); err != nil {
				return err
			}
			return recomputeDietDayPage(pageId, map[string]bool{oldMeal: true, in.Meal: true})
		}
	}
	return errors.New("노션 표에서 해당 행을 찾지 못했습니다")
}

func deleteDietDayItem(rowId string) error {
	pageId, _, oldMeal, err := dietDayRowContext(rowId)
	if err != nil {
		return err
	}
	if err := notionDo("DELETE", "/v1/blocks/"+rowId, nil, &struct{}{}); err != nil {
		return err
	}
	return recomputeDietDayPage(pageId, map[string]bool{oldMeal: true})
}
