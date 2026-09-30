package clients

import (
	"testing"

	"dashboard/global/config"
)

func TestParseFoodResponse(t *testing.T) {
	// 공공데이터포털 JSON 모양 (값은 문자열, 빈 값 "-" 섞임)
	body := `{"header":{"resultCode":"00","resultMsg":"NORMAL SERVICE."},
	  "body":{"pageNo":1,"totalCount":2,"numOfRows":30,"items":[
	    {"FOOD_CD":"R101-001","FOOD_NM_KR":"닭가슴살","DB_GRP_NM":"농축수산물","MAKER_NM":"","SERVING_SIZE":"100g",
	     "AMT_NUM1":"109","AMT_NUM3":"22.97","AMT_NUM4":"1.21","AMT_NUM6":"0","AMT_NUM7":"-","Z10500":"-"},
	    {"FOOD_CD":"P201","FOOD_NM_KR":"그릭요거트","DB_GRP_NM":"가공식품","MAKER_NM":"어느회사","SERVING_SIZE":"100g",
	     "AMT_NUM1":"97","AMT_NUM3":"9","AMT_NUM4":"5","AMT_NUM6":"4.1","Z10500":"1,000g"}]}}`
	items, err := parseFoodResponse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	a := items[0]
	if a.Name != "닭가슴살" || a.BasisGrams != 100 || a.Per.Kcal != 109 || a.Per.Protein != 23 || a.Per.Carbs != 0 || a.Per.Fat != 1.2 || a.ServingGrams != 0 {
		t.Errorf("item0 = %+v", a)
	}
	if items[1].ServingGrams != 1000 || items[1].Per.Carbs != 4.1 {
		t.Errorf("item1 = %+v", items[1])
	}

	// items 가 {item:{...}} 단일 객체로 오는 경우
	one := `{"header":{"resultCode":"00"},"body":{"items":{"item":{"FOOD_NM_KR":"바나나","SERVING_SIZE":"100g","AMT_NUM1":"93"}}}}`
	items, err = parseFoodResponse([]byte(one))
	if err != nil || len(items) != 1 || items[0].Per.Kcal != 93 {
		t.Errorf("single item: %v %+v", err, items)
	}

	// 결과 없음
	empty := `{"header":{"resultCode":"00"},"body":{"totalCount":0,"items":""}}`
	if items, err := parseFoodResponse([]byte(empty)); err != nil || len(items) != 0 {
		t.Errorf("empty: %v %+v", err, items)
	}

	// 오류 코드
	if _, err := parseFoodResponse([]byte(`{"header":{"resultCode":"30","resultMsg":"SERVICE KEY IS NOT REGISTERED"}}`)); err == nil {
		t.Error("expected error for resultCode 30")
	}
}

func TestFoodServiceKey(t *testing.T) {
	old := config.FoodApiKey
	defer func() { config.FoodApiKey = old }()
	for in, want := range map[string]string{
		"abc%2Bdef%2F%3D%3D": "abc+def/==", // Encoding 키
		"abc+def/==":         "abc+def/==", // Decoding 키는 그대로
		" abc ":              "abc",
	} {
		config.FoodApiKey = in
		if got := foodServiceKey(); got != want {
			t.Errorf("foodServiceKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFoodRank(t *testing.T) {
	terms := []string{"계란", "달걀"}
	cases := []struct {
		it   FoodSearchItem
		want int
	}{
		{FoodSearchItem{Name: "달걀, 생것", Group: "원재료성", Class: "품목대표"}, 0},
		{FoodSearchItem{Name: "메추리알, 생것", Group: "원재료성", Class: "품목대표"}, 1},
		{FoodSearchItem{Name: "계란찜", Group: "음식", Class: "품목대표"}, 2},
		{FoodSearchItem{Name: "볶음밥_계란", Group: "음식", Class: "품목대표"}, 3},
		{FoodSearchItem{Name: "계란과자", Group: "가공식품", Class: "상용제품"}, 4},
	}
	if got := foodRank(FoodSearchItem{Name: "닭고기, 가슴, 생것", Group: "원재료성", Class: "품목대표"}, []string{"닭가슴살", "닭고기, 가슴"}); got != 0 {
		t.Errorf("닭고기, 가슴, 생것 rank = %d, want 0", got)
	}
	for _, c := range cases {
		if got := foodRank(c.it, terms); got != c.want {
			t.Errorf("foodRank(%q) = %d, want %d", c.it.Name, got, c.want)
		}
	}
}
