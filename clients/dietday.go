package clients

// 식단 기록 — 노션 「운동 계획 / 식단 기록」 DB(하루 1페이지)를 diet_tb 행으로 풀어 넣는다.
// 실제 식단은 이쪽에 적는다: 속성은 아침·점심·저녁·간식 텍스트 + 하루 칼로리·단백질 합계,
// 본문 표는 「[끼니] 음식 | 약 230kcal | 약 18g」 행 + 「합계 (점심까지)」·「하루 합계」 행.
//   - 표의 음식 행 → 음식 1개 = diet_tb 1행 (끼니 태그 아점→아침, 야식→간식, 그 외 모르는 태그→간식)
//   - 표 합계와 속성 합계(사용자가 정정까지 반영한 최종값)가 다르면 차이를 「합계 보정」 행으로 넣어 하루 합계를 속성에 맞춘다
//   - 표가 없으면 끼니 텍스트를 행으로 넣고 하루 합계는 보정 행에 싣는다
// 표 열은 머리행 이름으로 찾는다(음식 | 칼로리 | 단백질 | 탄수화물 | 지방 — 탄·지 열은 선택). 탄·지 열이 없는 표는 음식별 0 이고 하루 합계가 「합계 보정」 행으로 간다. 같은 날짜 페이지가 여러 개면 최근 수정한 하나만 읽는다(이중 집계 방지).
// 음식 행 id = "r:"+표 행 블록 id (대시보드에서 수정·삭제 — dietdaywrite.go). 보정 행·끼니 텍스트 폴백 행은 「페이지id#순번」 = 읽기 전용.
// 「식단 일지」(음식 1행 DB)는 더 이상 쓰지 않는다 — 노션과 대시보드가 「식단 기록」 하나를 본다.

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"dashboard/global/config"
	"dashboard/global/log"
)

const (
	dietDayIdSep      = "#"
	dietDayAdjustFood = "합계 보정" // 복사 대상에서 뺀다 (음수일 수 있고 실제 음식이 아님)
)

// isDietDayReadonly 는 대시보드에서 고칠 수 없는 행(보정·끼니 텍스트 폴백)인지.
func isDietDayReadonly(id string) bool { return strings.Contains(id, dietDayIdSep) }

var dietDayMealAlias = map[string]string{"아점": "아침", "브런치": "아침", "야식": "간식", "운동전": "운동 전"}

var (
	dietDayTagRe  = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*(.+)$`)
	dietDayKcalRe = regexp.MustCompile(`([\d,.]+)\s*kcal`)
	dietDayGramRe = regexp.MustCompile(`([\d,.]+)\s*g`)
	// 음식 이름 끝의 양 「햇반 작은 공기 (130g)」 — 대시보드 입력도 이 꼴로 쓴다
	dietDayFoodGramsRe = regexp.MustCompile(`^(.*\S)\s*\((\d+(?:\.\d+)?)g\)$`)
)

type dietDayItem struct {
	rowId, meal, food                string
	grams, kcal, protein, carbs, fat float64
}

// dietDayCols 는 표 열 위치 (머리행 이름으로 찾는다, 없으면 -1). 기본 = 음식 | 칼로리 | 단백질
type dietDayCols struct{ kcal, protein, carbs, fat int }

var defaultDietDayCols = dietDayCols{kcal: 1, protein: 2, carbs: -1, fat: -1}

// parseDietDayHeader 는 머리행(「음식 | 칼로리 | 단백질 | 탄수화물 | 지방」)에서 열 위치를 찾는다.
func parseDietDayHeader(cells []string) dietDayCols {
	cols := dietDayCols{-1, -1, -1, -1}
	for i, c := range cells {
		switch {
		case strings.Contains(c, "칼로리"):
			cols.kcal = i
		case strings.Contains(c, "단백질"):
			cols.protein = i
		case strings.Contains(c, "탄수"):
			cols.carbs = i
		case strings.Contains(c, "지방"):
			cols.fat = i
		}
	}
	if cols.kcal < 0 {
		return defaultDietDayCols // 머리행이 아니면 옛 3열 형식으로 본다
	}
	return cols
}

func dietDayNumber(re *regexp.Regexp, s string) float64 {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	return v
}

func dietDayMeal(tag string) string {
	tag = strings.TrimSpace(tag)
	if a, ok := dietDayMealAlias[tag]; ok {
		return a
	}
	if containsString(DietMeals, tag) {
		return tag
	}
	return "간식"
}

// parseDietDayRow 는 표 1행(음식 | 칼로리 | 단백질)을 음식으로 푼다. 헤더·합계 행은 false.
func parseDietDayRow(cells []string, cols dietDayCols) (dietDayItem, bool) {
	if len(cells) < 2 {
		return dietDayItem{}, false
	}
	m := dietDayTagRe.FindStringSubmatch(cells[0])
	if m == nil {
		return dietDayItem{}, false // 「음식」 헤더, 「합계 (점심까지)」·「하루 합계」
	}
	cell := func(i int) string {
		if i < 0 || i >= len(cells) {
			return ""
		}
		return cells[i]
	}
	it := dietDayItem{
		meal: dietDayMeal(m[1]), food: strings.TrimSpace(m[2]),
		kcal:    dietDayNumber(dietDayKcalRe, cell(cols.kcal)),
		protein: dietDayNumber(dietDayGramRe, cell(cols.protein)),
		carbs:   dietDayNumber(dietDayGramRe, cell(cols.carbs)),
		fat:     dietDayNumber(dietDayGramRe, cell(cols.fat)),
	}
	if m := dietDayFoodGramsRe.FindStringSubmatch(it.food); m != nil {
		it.food = m[1]
		it.grams, _ = strconv.ParseFloat(m[2], 64)
	}
	return it, it.food != ""
}

// buildDietDayRows 는 하루 페이지 1개를 diet_tb 행으로 만든다.
// mealTexts: 끼니 → 속성 텍스트, total: 속성 하루 합계(칼로리·단백질)
func buildDietDayRows(pageId, date string, items []dietDayItem, mealTexts map[string]string, total Macros) []dietRow {
	rows := []dietRow{}
	add := func(meal, food, memo string, m Macros) {
		rows = append(rows, dietRow{
			notionId: fmt.Sprintf("%s%s%d", pageId, dietDayIdSep, len(rows)),
			date:     date, meal: meal, food: food, memo: memo, macros: m,
		})
	}

	sum := Macros{}
	if len(items) > 0 {
		for _, it := range items {
			m := Macros{Kcal: it.kcal, Protein: it.protein, Carbs: it.carbs, Fat: it.fat}
			add(it.meal, it.food, "", m)
			if it.rowId != "" {
				rows[len(rows)-1].notionId = dietDayRowPrefix + it.rowId
			}
			rows[len(rows)-1].grams = it.grams
			sum = sum.Add(m)
		}
	} else {
		for _, meal := range DietMeals {
			if t := strings.TrimSpace(mealTexts[meal]); t != "" {
				add(meal, t, "", Macros{})
			}
		}
	}

	// 속성 합계가 최종값 — 표에서 빠졌거나 본문에서 정정한 만큼 보정
	// 표에 탄·지 열이 없으면 음식별 탄·지는 0 이라 하루 합계 전체가 보정 행으로 간다
	diff := Macros{Kcal: total.Kcal - sum.Kcal, Protein: total.Protein - sum.Protein, Carbs: total.Carbs - sum.Carbs, Fat: total.Fat - sum.Fat}
	off := func(t, d float64) bool { return t > 0 && math.Abs(d) >= 1 }
	if off(total.Kcal, diff.Kcal) || off(total.Protein, diff.Protein) || off(total.Carbs, diff.Carbs) || off(total.Fat, diff.Fat) {
		add("간식", dietDayAdjustFood, "노션 식단 기록의 하루 합계에 맞춘 차이 (표에 없는 음식·정정분, 표에 없는 탄수화물·지방)", diff.Round())
	}
	return rows
}

type notionBlock struct {
	Id    string `json:"id"`
	Type  string `json:"type"`
	Table struct {
		TableWidth int `json:"table_width"`
	} `json:"table"`
	TableRow struct {
		Cells [][]notionRichText `json:"cells"`
	} `json:"table_row"`
}

func notionChildren(blockId string) ([]notionBlock, error) {
	var all []notionBlock
	cursor := ""
	for page := 0; page < 20; page++ {
		path := "/v1/blocks/" + blockId + "/children?page_size=100"
		if cursor != "" {
			path += "&start_cursor=" + cursor
		}
		var res struct {
			Results    []notionBlock `json:"results"`
			HasMore    bool          `json:"has_more"`
			NextCursor string        `json:"next_cursor"`
		}
		if err := notionDo("GET", path, nil, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Results...)
		if !res.HasMore || res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return all, nil
}

// 본문 표는 페이지마다 블록 API 를 여러 번 불러야 해서, 수정 시각이 같으면 지난번 파싱 결과를 재사용한다.
type dietDayCacheEntry struct {
	edited string
	items  []dietDayItem
}

var dietDayCache sync.Map // pageId → dietDayCacheEntry

func dietDayItems(p NotionPage) ([]dietDayItem, error) {
	if v, ok := dietDayCache.Load(p.Id); ok && p.LastEditedTime != "" && v.(dietDayCacheEntry).edited == p.LastEditedTime {
		return v.(dietDayCacheEntry).items, nil
	}
	blocks, err := notionChildren(p.Id)
	if err != nil {
		return nil, err
	}
	items := []dietDayItem{}
	for _, b := range blocks {
		if b.Type != "table" {
			continue
		}
		rows, err := notionChildren(b.Id)
		if err != nil {
			return nil, err
		}
		cols := defaultDietDayCols
		for i, r := range rows {
			cells := make([]string, len(r.TableRow.Cells))
			for i, c := range r.TableRow.Cells {
				cells[i] = joinRichText(c)
			}
			if i == 0 {
				cols = parseDietDayHeader(cells)
			}
			if it, ok := parseDietDayRow(cells, cols); ok {
				it.rowId = r.Id
				items = append(items, it)
			}
		}
	}
	// last_edited_time 은 분 단위라, 방금(2분 안) 고친 페이지는 같은 분 안의 다음 수정을 놓칠 수 있어 캐시하지 않는다
	if edited, err := time.Parse(time.RFC3339, p.LastEditedTime); err == nil && time.Since(edited) > 2*time.Minute {
		dietDayCache.Store(p.Id, dietDayCacheEntry{edited: p.LastEditedTime, items: items})
	}
	return items, nil
}

// fetchDietDayRows 는 「식단 기록」 전체를 diet_tb 행으로 만든다.
func fetchDietDayRows() ([]dietRow, error) {
	if config.NotionDietDayDS == "" {
		return nil, nil
	}
	pages, err := notionQueryAll(config.NotionDietDayDS)
	if err != nil {
		return nil, err
	}
	// 같은 날짜 페이지가 여러 개면(노션에서 새로 만든 경우 등) 가장 최근 수정한 하나만 — 이중 집계 방지, 쓰기(findDietDayPage)도 같은 페이지
	latest := map[string]NotionPage{}
	for _, p := range pages {
		date := p.Date("날짜")
		if date == "" {
			continue
		}
		if q, ok := latest[date]; ok {
			log.Warn().Str("service", "diet-sync").Str("date", date).Msg("같은 날짜 페이지 중복 — 최근 수정 페이지만 사용")
			if q.LastEditedTime >= p.LastEditedTime {
				continue
			}
		}
		latest[date] = p
	}
	rows := []dietRow{}
	for _, p := range pages {
		date := p.Date("날짜")
		if date == "" || latest[date].Id != p.Id {
			continue
		}
		items, err := dietDayItems(p)
		if err != nil {
			return nil, err
		}
		mealTexts := map[string]string{}
		for _, meal := range DietMeals {
			mealTexts[meal] = p.Text(meal)
		}
		total := Macros{Kcal: p.Number("칼로리(kcal)"), Protein: p.Number("단백질(g)"), Carbs: p.Number("탄수화물(g)"), Fat: p.Number("지방(g)")}
		rows = append(rows, buildDietDayRows(p.Id, date, items, mealTexts, total)...)
	}
	return rows, nil
}
