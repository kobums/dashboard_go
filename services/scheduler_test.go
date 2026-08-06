package services

import (
	"testing"
	"time"
)

func TestNextNotifyRun(t *testing.T) {
	cases := []struct {
		now      string
		wantMode string
		wantAt   string
	}{
		{"2026-08-06 08:30", "morning", "2026-08-06 09:00"},
		{"2026-08-06 09:00", "evening", "2026-08-06 20:00"}, // 정각 발송 직후 재계산 — 같은 슬롯 재발송 금지
		{"2026-08-06 12:00", "evening", "2026-08-06 20:00"},
		{"2026-08-06 20:00", "morning", "2026-08-07 09:00"},
		{"2026-08-06 23:59", "morning", "2026-08-07 09:00"},
	}

	const layout = "2006-01-02 15:04"
	for _, c := range cases {
		now, err := time.ParseInLocation(layout, c.now, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		mode, at := nextNotifyRun(now)
		if mode != c.wantMode || at.Format(layout) != c.wantAt {
			t.Errorf("nextNotifyRun(%v) = (%v, %v), want (%v, %v)",
				c.now, mode, at.Format(layout), c.wantMode, c.wantAt)
		}
	}
}
