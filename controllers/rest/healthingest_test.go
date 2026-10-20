package rest

import "testing"

func TestDailyMetricValues(t *testing.T) {
	// 시간 단위로 온 걸음 — 하루 합계여야 한다 (예전엔 마지막 시간 값으로 덮였다)
	steps := haeMetric{Name: "step_count", Units: "count", Data: []haeMetricPoint{
		{Date: "2026-10-20 08:00:00 +0900", Qty: 1200},
		{Date: "2026-10-20 09:00:00 +0900", Qty: 800},
		{Date: "2026-10-21 08:00:00 +0900", Qty: 50},
	}}
	daily, _ := dailyMetricValues("steps", steps)
	if daily["2026-10-20"] != 2000 || daily["2026-10-21"] != 50 {
		t.Errorf("steps: %v", daily)
	}

	// 체중은 그날 마지막 값
	weight := haeMetric{Name: "weight_body_mass", Units: "kg", Data: []haeMetricPoint{
		{Date: "2026-10-20 07:00:00 +0900", Qty: 72.4},
		{Date: "2026-10-20 22:00:00 +0900", Qty: 73.1},
	}}
	daily, _ = dailyMetricValues("weight", weight)
	if daily["2026-10-20"] != 73.1 {
		t.Errorf("weight: %v", daily)
	}

	// kJ → kcal
	energy := haeMetric{Name: "active_energy", Units: "kJ", Data: []haeMetricPoint{{Date: "2026-10-20 00:00:00 +0900", Qty: 4184}}}
	daily, unit := dailyMetricValues("active_energy", energy)
	if unit != "kcal" || daily["2026-10-20"] < 999.9 || daily["2026-10-20"] > 1000.1 {
		t.Errorf("energy: %v %v", daily, unit)
	}
}

func TestParseShortcutDate(t *testing.T) {
	cases := map[string]string{
		"2026-10-20":                "2026-10-20",
		"2026. 10. 20.":             "2026-10-20",
		"2026. 1. 5. 오후 9:00":       "2026-01-05",
		"2026년 10월 20일 오후 9:00":     "2026-10-20",
		"2026-10-20T21:00:00+09:00": "2026-10-20",
		"2026/10/20":                "2026-10-20",
	}
	for in, want := range cases {
		if got, ok := parseShortcutDate(in); !ok || got != want {
			t.Errorf("%q: got %q %v, want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"어제", "2026-02-31", ""} {
		if got, ok := parseShortcutDate(bad); ok {
			t.Errorf("%q: should fail, got %q", bad, got)
		}
	}
}

func TestShortcutValue(t *testing.T) {
	cases := []struct {
		name string
		val  interface{}
		want float64
	}{
		{"steps", float64(8532), 8532},
		{"steps", "8,532", 8532},
		{"steps", "8532 걸음", 8532},
		{"steps", "1200\n800\n35", 2035},                   // 건강 샘플 여러 개 — 합계
		{"steps", []interface{}{float64(100), "200"}, 300}, // 배열
		{"weight", "72.4kg", 72.4},                         // 단위 붙음(공백 없음)
		{"weight", "72.4 kg\n73.1 kg", 73.1},               // 여러 개면 마지막
	}
	for _, c := range cases {
		if got, ok := shortcutValue(c.name, c.val); !ok || got != c.want {
			t.Errorf("%s %q: got %v %v, want %v", c.name, c.val, got, ok, c.want)
		}
	}
	if _, ok := shortcutValue("steps", "없음"); ok {
		t.Error("non-number should fail")
	}
}
