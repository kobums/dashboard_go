package clients

// 식단 기록 — 노션 「운동 계획 / 식단 기록」 DB(하루 1페이지)를 diet_tb 행으로 풀어 넣는다.
// 실제 식단은 이쪽에 적는다: 속성은 아침·점심·저녁·간식 텍스트 + 하루 칼로리·단백질 합계,
// 본문 표는 「[끼니] 음식 | 약 230kcal | 약 18g」 행 + 「합계 (점심까지)」·「하루 합계」 행.
//   - 표의 음식 행 → 음식 1개 = diet_tb 1행 (끼니 태그 아점→아침, 야식→간식, 그 외 모르는 태그→간식)
//   - 표 합계와 속성 합계(사용자가 정정까지 반영한 최종값)가 다르면 차이를 「합계 보정」 행으로 넣어 하루 합계를 속성에 맞춘다
//   - 표가 없으면 끼니 텍스트를 행으로 넣고 하루 합계는 보정 행에 싣는다
// 탄수화물·지방은 표에 열이 없고 하루 합계 속성만 있다 — 음식별로 지어내지 않고(0) 하루 합계 전체를 「합계 보정」 행에 싣는다. 같은 날짜에 「식단 일지」 기록이 있으면 그날은 식단 일지만 쓴다(이중 집계 방지).
// 행 id 는 「페이지id#순번」 — 대시보드에서 수정·삭제 불가(노션 식단 기록 페이지에서 고친다).

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"dashboard/global/config"
)

const (
	dietDayIdSep      = "#"
	dietDayAdjustFood = "합계 보정" // 복사 대상에서 뺀다 (음수일 수 있고 실제 음식이 아님)
)

// IsDietDayItem 은 「식단 기록」에서 가져온 (대시보드에서 고칠 수 없는) 행인지.
func IsDietDayItem(id string) bool { return strings.Contains(id, dietDayIdSep) }

var dietDayMealAlias = map[string]string{"아점": "아침", "브런치": "아침", "야식": "간식", "운동전": "운동 전"}

var (
	dietDayTagRe  = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*(.+)$`)
	dietDayKcalRe = regexp.MustCompile(`([\d,.]+)\s*kcal`)
	dietDayGramRe = regexp.MustCompile(`([\d,.]+)\s*g`)
)

type dietDayItem struct {
	meal, food    string
	kcal, protein float64
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
func parseDietDayRow(cells []string) (dietDayItem, bool) {
	if len(cells) < 2 {
		return dietDayItem{}, false
	}
	m := dietDayTagRe.FindStringSubmatch(cells[0])
	if m == nil {
		return dietDayItem{}, false // 「음식」 헤더, 「합계 (점심까지)」·「하루 합계」
	}
	it := dietDayItem{meal: dietDayMeal(m[1]), food: strings.TrimSpace(m[2]), kcal: dietDayNumber(dietDayKcalRe, cells[1])}
	if len(cells) >= 3 {
		it.protein = dietDayNumber(dietDayGramRe, cells[2])
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
			m := Macros{Kcal: it.kcal, Protein: it.protein}
			add(it.meal, it.food, "", m)
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
	diff := Macros{Kcal: total.Kcal - sum.Kcal, Protein: total.Protein - sum.Protein, Carbs: total.Carbs, Fat: total.Fat}
	off := func(t, d float64) bool { return t > 0 && math.Abs(d) >= 1 }
	if off(total.Kcal, diff.Kcal) || off(total.Protein, diff.Protein) || total.Carbs > 0 || total.Fat > 0 {
		memo := "노션 식단 기록의 하루 합계에 맞춘 차이 (표에 없는 음식·정정분)"
		if total.Carbs > 0 || total.Fat > 0 {
			memo += " + 탄수화물·지방 하루 합계 (음식별 값은 기록에 없음)"
		}
		add("간식", dietDayAdjustFood, memo, diff.Round())
	}
	return rows
}

type notionBlock struct {
	Id       string `json:"id"`
	Type     string `json:"type"`
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
		for _, r := range rows {
			cells := make([]string, len(r.TableRow.Cells))
			for i, c := range r.TableRow.Cells {
				cells[i] = joinRichText(c)
			}
			if it, ok := parseDietDayRow(cells); ok {
				items = append(items, it)
			}
		}
	}
	dietDayCache.Store(p.Id, dietDayCacheEntry{edited: p.LastEditedTime, items: items})
	return items, nil
}

// fetchDietDayRows 는 「식단 기록」 전체를 diet_tb 행으로 만든다. skipDates 날짜는 건너뛴다.
func fetchDietDayRows(skipDates map[string]bool) ([]dietRow, error) {
	if config.NotionDietDayDS == "" {
		return nil, nil
	}
	pages, err := notionQueryAll(config.NotionDietDayDS)
	if err != nil {
		return nil, err
	}
	rows := []dietRow{}
	for _, p := range pages {
		date := p.Date("날짜")
		if date == "" || skipDates[date] {
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
