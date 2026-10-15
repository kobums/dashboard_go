package clients

import "testing"

func TestParseDietDayRow(t *testing.T) {
	cases := []struct {
		cells []string
		ok    bool
		want  dietDayItem
	}{
		{[]string{"[아침] 계란 3개", "약 230kcal", "약 18g"}, true, dietDayItem{"아침", "계란 3개", 230, 18}},
		{[]string{"[야식] 치킨 (반 마리 기준)", "약 1,100kcal", "약 55g"}, true, dietDayItem{"간식", "치킨 (반 마리 기준)", 1100, 55}},
		{[]string{"[아점] 밥 1공기", "약 310kcal", "약 6g"}, true, dietDayItem{"아침", "밥 1공기", 310, 6}},
		{[]string{"[저녁] 사이다 1캔", "약 110kcal", "0g"}, true, dietDayItem{"저녁", "사이다 1캔", 110, 0}},
		{[]string{"음식", "칼로리", "단백질"}, false, dietDayItem{}},
		{[]string{"하루 합계", "약 2,970kcal", "약 145g"}, false, dietDayItem{}},
		{[]string{"**합계 (점심까지)**", "약 1,530kcal", "약 75g"}, false, dietDayItem{}},
	}
	for _, c := range cases {
		got, ok := parseDietDayRow(c.cells)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%v: got %+v %v, want %+v %v", c.cells, got, ok, c.want, c.ok)
		}
	}
}

func TestBuildDietDayRows(t *testing.T) {
	items := []dietDayItem{{"점심", "결혼식 뷔페", 900, 60}, {"저녁", "빅맥 1개", 580, 26}}
	// 10/11: 표에 프로틴 쉐이크가 빠져 속성 합계(1590/106)와 110kcal·20g 차이
	rows := buildDietDayRows("p", "2026-10-11", items, nil, Macros{Kcal: 1590, Protein: 106})
	if len(rows) != 3 || rows[2].food != dietDayAdjustFood || rows[2].macros.Kcal != 110 || rows[2].macros.Protein != 20 {
		t.Fatalf("adjust row: %+v", rows)
	}
	if rows[0].notionId != "p#0" || !IsDietDayItem(rows[0].notionId) {
		t.Errorf("id: %v", rows[0].notionId)
	}
	// 탄·지는 음식별 0, 하루 합계는 보정 행에 (칼로리·단백질이 맞아도 보정 행 생김)
	rows = buildDietDayRows("p", "d", items, nil, Macros{Kcal: 1480, Protein: 86, Carbs: 118, Fat: 77})
	if len(rows) != 3 || rows[0].macros.Carbs != 0 || rows[2].macros != (Macros{Carbs: 118, Fat: 77}) {
		t.Errorf("carbs/fat: %+v", rows)
	}
	// 합계가 맞으면 보정 행 없음
	if rows := buildDietDayRows("p", "d", items, nil, Macros{Kcal: 1480, Protein: 86}); len(rows) != 2 {
		t.Errorf("no adjust expected: %+v", rows)
	}
	// 표 없음 → 끼니 텍스트 행 + 하루 합계 보정
	rows = buildDietDayRows("p", "d", nil, map[string]string{"아침": "계란", "저녁": "닭가슴살"}, Macros{Kcal: 1000, Protein: 80})
	if len(rows) != 3 || rows[0].meal != "아침" || rows[1].meal != "저녁" || rows[2].macros.Kcal != 1000 {
		t.Errorf("fallback: %+v", rows)
	}
}
