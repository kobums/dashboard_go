package clients

import (
	"reflect"
	"testing"
)

func TestParseSets(t *testing.T) {
	s := func(w float64, r int) LiftSet { return LiftSet{Weight: w, Reps: r, RepsMax: r} }
	bw := func(r int) LiftSet { return LiftSet{Reps: r, RepsMax: r, Bodyweight: true} }

	cases := []struct {
		in   string
		want []LiftSet
	}{
		{"", nil},
		{"50×14×5", []LiftSet{s(50, 14), s(50, 14), s(50, 14), s(50, 14), s(50, 14)}},
		{"20×20, 40×15", []LiftSet{s(20, 20), s(40, 15)}},
		{"50×15,11,10 / 60×4,4", []LiftSet{s(50, 15), s(50, 11), s(50, 10), s(60, 4), s(60, 4)}},
		{"67.5×15,15", []LiftSet{s(67.5, 15), s(67.5, 15)}},
		{"30kg × 12, 10", []LiftSet{s(30, 12), s(30, 10)}},
		{"BW×10,10", []LiftSet{bw(10), bw(10)}},
		{"15,15", []LiftSet{bw(15), bw(15)}},
		{"빈기계×20, 30×20", []LiftSet{s(0, 20), s(30, 20)}},
		{"2x20x3", []LiftSet{s(2, 20), s(2, 20), s(2, 20)}},
		{"55×12~15×2", []LiftSet{{Weight: 55, Reps: 12, RepsMax: 15}, {Weight: 55, Reps: 12, RepsMax: 15}}},
		// 세트 옆 메모·표기 허용
		{"30×15×2 → 되면 35", []LiftSet{s(30, 15), s(30, 15)}},
		{"25×15×2 (세트 간 횟수 유지)", []LiftSet{s(25, 15), s(25, 15)}},
		{"20(빈바)×20", []LiftSet{s(20, 20)}},
		{"30×15×2 (한 번에 → 되면 35)", []LiftSet{s(30, 15), s(30, 15)}},
		{"30×15 × 2 / 드롭 20×15, 13", []LiftSet{s(30, 15), s(30, 15), s(20, 15), s(20, 13)}},
		{"맨몸 10×3", []LiftSet{bw(10), bw(10), bw(10)}},
		{"맨몸 10, 8", []LiftSet{bw(10), bw(8)}},
	}
	for _, c := range cases {
		got, err := ParseSets(c.in)
		if err != nil {
			t.Errorf("ParseSets(%q) error: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseSets(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}

	for _, bad := range []string{"50×abc", "무거움×10", "50×10×0", "1×2×3×4"} {
		if _, err := ParseSets(bad); err == nil {
			t.Errorf("ParseSets(%q) expected error", bad)
		}
	}
}
