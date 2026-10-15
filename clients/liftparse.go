package clients

// 노션 운동 일지의 웜업/본세트/목표 텍스트 파서.
//
// 문법 (노션 DB 설명에 적힌 것과 같다):
//   무게×횟수            50×15
//   무게×횟수×세트        40×15×5
//   무게×횟수,횟수,…      50×15,11,10       — 숫자만 있는 항목은 직전 무게를 이어받는다
//   그룹 구분 /           50×15,11,10 / 60×4,4
//   맨몸                  BW×10,10 (또는 맨몸, 무게 없이 10,10)
//   빈 기계               빈기계×20 → 0kg
//   횟수 범위(목표용)     55×12~15×4        — Reps=12, RepsMax=15
// 곱셈 기호는 × x X * 모두 허용, kg 표기는 무시한다.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type LiftSet struct {
	Weight     float64 `json:"weight"`
	Reps       int     `json:"reps"`
	RepsMax    int     `json:"-"` // 목표의 횟수 범위 상단 (범위가 아니면 Reps 와 같음)
	Bodyweight bool    `json:"bodyweight"`
}

var setNoteParens = regexp.MustCompile(`[(（][^)）]*[)）]`)

// normalizeSetText 는 기호를 통일하고, 세트 옆에 흔히 붙이는 메모를 떼어낸다:
// 괄호 메모 "25×15×4 (세트 간 횟수 유지)"·"20(빈바)×20", 화살표 뒤 "30×15×4 → 되면 35".
func normalizeSetText(s string) string {
	// 괄호를 먼저 지운다 — 괄호 안에 화살표가 있는 "30×15×4 (한 번에 → 되면 35)" 도 통째로 메모
	s = setNoteParens.ReplaceAllString(s, "")
	if i := strings.IndexAny(s, "(（"); i >= 0 { // 닫히지 않은 괄호는 끝까지 메모
		s = s[:i]
	}
	if i := strings.Index(s, "→"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "->"); i >= 0 {
		s = s[:i]
	}
	r := strings.NewReplacer("×", "x", "X", "x", "*", "x", "✕", "x", "kg", "", "KG", "", "Kg", "", "～", "~", "–", "~")
	return strings.TrimSpace(r.Replace(s))
}

// 세트 앞에 붙는 설명어 — 드롭세트는 그냥 세트로 센다
var setLabelPrefixes = []string{"드롭세트", "드롭", "드랍"}

// 무게 자리 없이 붙여 쓴 맨몸 표기: "맨몸 10", "맨몸 10×3", "BW10"
var bwPrefixes = []string{"맨몸", "BW", "bw", "Bw"}

// ParseSets 는 세트 표기 문자열을 세트 목록으로 편다. 빈 문자열은 (nil, nil).
func ParseSets(text string) ([]LiftSet, error) {
	text = normalizeSetText(text)
	if text == "" || text == "-" || text == "—" {
		return nil, nil
	}

	var sets []LiftSet
	for _, group := range strings.Split(text, "/") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		// 그룹 안에서 직전 무게를 이어받는다 — 그룹이 바뀌면 초기화
		curWeight, curBW, hasWeight := 0.0, false, false
		for _, item := range strings.Split(group, ",") {
			item = strings.ReplaceAll(strings.TrimSpace(item), " ", "")
			if item == "" {
				continue
			}
			for _, label := range setLabelPrefixes {
				if strings.HasPrefix(item, label) {
					item = strings.TrimPrefix(item, label)
					break
				}
			}
			// "맨몸10" / "맨몸10x3" → 맨몸 무게로 풀어 쓴 꼴("BWx10x3")로 바꾼다
			for _, bw := range bwPrefixes {
				if rest := strings.TrimPrefix(item, bw); rest != item && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
					item = "BWx" + rest
					break
				}
			}
			parts := strings.Split(item, "x")
			var repsTok, setsTok string
			switch len(parts) {
			case 1:
				repsTok = parts[0]
			case 2, 3:
				w, bw, err := parseWeight(parts[0])
				if err != nil {
					return nil, fmt.Errorf("%q: %v", item, err)
				}
				curWeight, curBW, hasWeight = w, bw, true
				repsTok = parts[1]
				if len(parts) == 3 {
					setsTok = parts[2]
				}
			default:
				return nil, fmt.Errorf("%q: 형식은 무게×횟수×세트", item)
			}

			reps, repsMax, err := parseReps(repsTok)
			if err != nil {
				return nil, fmt.Errorf("%q: %v", item, err)
			}
			count := 1
			if setsTok != "" {
				count, err = strconv.Atoi(setsTok)
				if err != nil || count <= 0 || count > 50 {
					return nil, fmt.Errorf("%q: 세트 수 %q 를 읽을 수 없음", item, setsTok)
				}
			}
			// 무게 없이 횟수만 있으면 맨몸으로 본다
			bw := curBW || !hasWeight
			for i := 0; i < count; i++ {
				sets = append(sets, LiftSet{Weight: curWeight, Reps: reps, RepsMax: repsMax, Bodyweight: bw})
			}
		}
	}
	return sets, nil
}

func parseWeight(tok string) (float64, bool, error) {
	switch strings.ToUpper(tok) {
	case "BW", "맨몸":
		return 0, true, nil
	case "빈기계", "빈바":
		return 0, false, nil
	}
	w, err := strconv.ParseFloat(tok, 64)
	if err != nil || w < 0 || w > 1000 {
		return 0, false, fmt.Errorf("무게 %q 를 읽을 수 없음", tok)
	}
	return w, false, nil
}

func parseReps(tok string) (int, int, error) {
	lo, hi, isRange := strings.Cut(tok, "~")
	min, err := strconv.Atoi(lo)
	if err != nil || min <= 0 || min > 500 {
		return 0, 0, fmt.Errorf("횟수 %q 를 읽을 수 없음", tok)
	}
	max := min
	if isRange {
		max, err = strconv.Atoi(hi)
		if err != nil || max < min {
			return 0, 0, fmt.Errorf("횟수 범위 %q 를 읽을 수 없음", tok)
		}
	}
	return min, max, nil
}
