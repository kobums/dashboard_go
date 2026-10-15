package clients

import "testing"

func TestProgressPages(t *testing.T) {
	snaps := readingSnapshots{
		"2026-10-12": {"1": 100, "2": 0},
		"2026-10-14": {"1": 141, "2": 0, "3": 50}, // 13일은 서버 다운 — 직전(12일)과 비교
		"2026-10-15": {"1": 120, "2": 10, "3": 80},
	}
	cases := []struct {
		date string
		want int
	}{
		{"2026-10-12", 0},  // 이전 스냅샷 없음
		{"2026-10-13", 0},  // 그날 스냅샷 없음
		{"2026-10-14", 41}, // 새로 등록한 책(3)은 제외
		{"2026-10-15", 40}, // 감소(1: 141→120)는 무시, 2: +10, 3: +30
	}
	for _, c := range cases {
		if got := progressPages(snaps, c.date); got != c.want {
			t.Errorf("%s: got %d, want %d", c.date, got, c.want)
		}
	}
}

func TestFinishedOn(t *testing.T) {
	books := []snippetUserBook{
		{Title: "구의 증명", Status: "completed", EndDate: "2026-10-14T17:47:48"},
		{Title: "설득의 심리학", Status: "reading", EndDate: "2026-10-14T11:18:44"}, // 읽는 중도 endDate 가 찍혀 있음
		{Title: "쇼펜하우어", Status: "completed", EndDate: "2026-10-13T13:56:05"},
	}
	got := finishedOn(books, "2026-10-14")
	if len(got) != 1 || got[0] != "구의 증명" {
		t.Errorf("got %v", got)
	}
}
