package clients

// 식약처 식품영양성분DB 검색 (공공데이터포털 FoodNtrCpntDbInfo03). FOOD_API_KEY 필요.
// 응답 영양 값은 SERVING_SIZE(영양성분 기준량, 보통 "100g") 당이다.
// 항목 번호: AMT_NUM1 에너지(kcal) · AMT_NUM3 단백질 · AMT_NUM4 지방 · AMT_NUM6 탄수화물 (AMT_NUM7 은 당류 — 헷갈리지 말 것)
// Z10500 = 식품중량(1회 제공량 제안용). 같은 검색어는 하루 동안 메모리 캐시.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dashboard/global/config"
)

// 서비스 버전이 바뀌면(02 → 03) 옛 주소는 NO_OPENAPI_SERVICE_ERROR — 활용신청 상세의 「요청주소」와 맞출 것. FOOD_API_URL 로 덮어쓸 수 있다.
func foodApiURL() string {
	if config.FoodApiURL != "" {
		return config.FoodApiURL
	}
	return "https://apis.data.go.kr/1471000/FoodNtrCpntDbInfo03/getFoodNtrCpntDbInq03"
}

var foodClient = &http.Client{Timeout: 10 * time.Second}

type FoodSearchItem struct {
	FoodCode     string  `json:"foodCode"`
	Name         string  `json:"name"`
	Maker        string  `json:"maker"`
	Group        string  `json:"group"` // 음식 / 가공식품 / 농축수산물 등 (DB_GRP_NM)
	Class        string  `json:"-"`     // 품목대표 / 상용제품 / 외식 (DB_CLASS_NM) — 정렬용
	Basis        string  `json:"basis"`
	BasisGrams   float64 `json:"basisGrams"`
	Per          Macros  `json:"per"`
	ServingGrams float64 `json:"servingGrams"`
}

type foodCacheEntry struct {
	items []FoodSearchItem
	at    time.Time
}

var foodCache = map[string]foodCacheEntry{}
var foodCacheMutex sync.Mutex

var leadingNumber = regexp.MustCompile(`[0-9]+(\.[0-9]+)?`)

// parseAmount 는 "100g", "1,000", "12.5", "-" 같은 값을 숫자로 (없으면 0).
func parseAmount(v interface{}) float64 {
	var s string
	switch t := v.(type) {
	case string:
		s = t
	case float64:
		return t
	case nil:
		return 0
	default:
		s = fmt.Sprint(t)
	}
	m := leadingNumber.FindString(strings.ReplaceAll(s, ",", ""))
	f, _ := strconv.ParseFloat(m, 64)
	return f
}

func strOf(v interface{}) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// 흔한 표기 → DB 원재료 표준 명칭. 원재료는 "달걀, 생것"·"닭고기, 가슴, 생것" 처럼 쉼표로 부위·조리법을 붙인다
var foodSynonyms = map[string]string{
	"계란":   "달걀",
	"달걀":   "계란",
	"흰자":   "달걀, 난백",
	"계란흰자": "달걀, 난백",
	"닭가슴살": "닭고기, 가슴",
	"닭안심":  "닭고기, 안심",
	"요거트":  "요구르트",
	"요구르트": "요거트",
	"쉐이크":  "셰이크",
}

// SearchFoods 는 음식 이름으로 검색해 최대 40개를 돌려준다.
// FOOD_NM_KR 은 부분 일치라 "바나나" 가 1,800건이고 앞쪽은 외식·가공품이다. 그래서
// ① 품목대표(DB_CLASS_NM) — 원재료("바나나, 생것")·대표 음식 ② 전체 첫 페이지 — 브랜드 제품 을 병렬로 받아
// 원재료·정확 일치 → 대표 음식 → 제품 순으로 정렬한다. 동의어(계란↔달걀)도 품목대표로 한 번 더 찾는다.
func SearchFoods(q string) ([]FoodSearchItem, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []FoodSearchItem{}, nil
	}
	if config.FoodApiKey == "" {
		return nil, errors.New("식품 검색 API 키 미설정")
	}
	foodCacheMutex.Lock()
	if e, ok := foodCache[q]; ok && time.Since(e.at) < 24*time.Hour {
		foodCacheMutex.Unlock()
		return e.items, nil
	}
	foodCacheMutex.Unlock()

	type job struct {
		name, class string
		rows        int
	}
	jobs := []job{{q, "품목대표", 100}, {q, "", 30}}
	if syn, ok := foodSynonyms[q]; ok {
		jobs = append(jobs, job{syn, "품목대표", 100})
	}
	results := make([][]FoodSearchItem, len(jobs))
	errs := make([]error, len(jobs))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j job) {
			defer wg.Done()
			results[i], errs[i] = foodQuery(j.name, j.class, j.rows)
		}(i, j)
	}
	wg.Wait()
	if errs[0] != nil && errs[1] != nil {
		return nil, errs[0]
	}

	var merged []FoodSearchItem
	seen := map[string]bool{}
	for _, list := range results {
		for _, it := range list {
			key := it.FoodCode + "|" + it.Name
			if !seen[key] {
				seen[key] = true
				merged = append(merged, it)
			}
		}
	}
	terms := []string{q}
	if syn, ok := foodSynonyms[q]; ok {
		terms = append(terms, syn)
	}
	sort.SliceStable(merged, func(a, b int) bool {
		ra, rb := foodRank(merged[a], terms), foodRank(merged[b], terms)
		if ra != rb {
			return ra < rb
		}
		return len([]rune(merged[a].Name)) < len([]rune(merged[b].Name))
	})
	if len(merged) > 40 {
		merged = merged[:40]
	}

	foodCacheMutex.Lock()
	if len(foodCache) > 500 { // 단순 상한 — 넘치면 비운다
		foodCache = map[string]foodCacheEntry{}
	}
	foodCache[q] = foodCacheEntry{merged, time.Now()}
	foodCacheMutex.Unlock()
	return merged, nil
}

// foodRank 는 낮을수록 위. 0 이름 정확 일치·"검색어, …" 원재료 / 1 원재료성 / 2 대표 음식이 검색어로 시작 / 3 대표 음식 / 4 제품
func foodRank(it FoodSearchItem, terms []string) int {
	for _, t := range terms {
		// "달걀, 생것" (t="달걀") 또는 동의어 자체가 쉼표 명칭인 "닭고기, 가슴, 생것" (t="닭고기, 가슴")
		if it.Name == t || strings.HasPrefix(it.Name, t+",") || (strings.Contains(t, ",") && strings.HasPrefix(it.Name, t)) {
			return 0
		}
	}
	if it.Group == "원재료성" {
		return 1
	}
	if it.Class == "품목대표" {
		for _, t := range terms {
			if strings.HasPrefix(it.Name, t) {
				return 2
			}
		}
		return 3
	}
	return 4
}

// foodQuery 는 식약처 API 1회 호출. class 가 있으면 DB_CLASS_NM 필터(품목대표 등).
func foodQuery(name, class string, rows int) ([]FoodSearchItem, error) {
	params := url.Values{}
	params.Set("serviceKey", foodServiceKey())
	params.Set("type", "json")
	params.Set("pageNo", "1")
	params.Set("numOfRows", strconv.Itoa(rows))
	params.Set("FOOD_NM_KR", name)
	if class != "" {
		params.Set("DB_CLASS_NM", class)
	}
	res, err := foodClient.Get(foodApiURL() + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	buf, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("식품 검색 %v: %s", res.StatusCode, truncateRunes(string(buf), 120))
	}
	return parseFoodResponse(buf)
}

// parseFoodResponse 는 공공데이터포털 JSON 을 관대하게 읽는다 (items 가 배열 / {item:[...]} / 단일 객체인 경우 모두).
func parseFoodResponse(buf []byte) ([]FoodSearchItem, error) {
	var raw struct {
		Header struct {
			ResultCode string `json:"resultCode"`
			ResultMsg  string `json:"resultMsg"`
		} `json:"header"`
		Body struct {
			Items json.RawMessage `json:"items"`
		} `json:"body"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		// 키 오류 등은 XML(OpenAPI_ServiceResponse)로 오는 경우가 있다
		return nil, fmt.Errorf("식품 검색 응답 오류: %s", truncateRunes(string(buf), 120))
	}
	if raw.Header.ResultCode != "" && raw.Header.ResultCode != "00" {
		return nil, fmt.Errorf("식품 검색 오류 %s: %s", raw.Header.ResultCode, raw.Header.ResultMsg)
	}

	var list []map[string]interface{}
	if len(raw.Body.Items) > 0 && string(raw.Body.Items) != "null" && string(raw.Body.Items) != `""` {
		if json.Unmarshal(raw.Body.Items, &list) != nil {
			var wrapped struct {
				Item json.RawMessage `json:"item"`
			}
			if json.Unmarshal(raw.Body.Items, &wrapped) == nil && len(wrapped.Item) > 0 {
				if json.Unmarshal(wrapped.Item, &list) != nil {
					var one map[string]interface{}
					if json.Unmarshal(wrapped.Item, &one) == nil {
						list = []map[string]interface{}{one}
					}
				}
			}
		}
	}

	out := make([]FoodSearchItem, 0, len(list))
	for _, it := range list {
		basis := strOf(it["SERVING_SIZE"])
		basisGrams := parseAmount(basis)
		if basisGrams <= 0 {
			basisGrams = 100
		}
		out = append(out, FoodSearchItem{
			FoodCode:   strOf(it["FOOD_CD"]),
			Name:       strOf(it["FOOD_NM_KR"]),
			Maker:      strOf(it["MAKER_NM"]),
			Group:      strOf(it["DB_GRP_NM"]),
			Class:      strOf(it["DB_CLASS_NM"]),
			Basis:      basis,
			BasisGrams: basisGrams,
			Per: Macros{
				Kcal:    parseAmount(it["AMT_NUM1"]),
				Protein: parseAmount(it["AMT_NUM3"]),
				Carbs:   parseAmount(it["AMT_NUM6"]),
				Fat:     parseAmount(it["AMT_NUM4"]),
			}.Round(),
			ServingGrams: parseAmount(it["Z10500"]),
		})
	}
	return out, nil
}

func FoodSearchEnabled() bool { return config.FoodApiKey != "" }

// foodServiceKey 는 공공데이터포털의 Decoding 키를 돌려준다. Encoding 키(%2B… 형태)를 넣어도
// 여기서 한 번 풀어 두면 params.Encode() 가 올바르게 한 번만 인코딩한다 (이중 인코딩 → SERVICE_KEY_IS_NOT_REGISTERED).
func foodServiceKey() string {
	key := strings.TrimSpace(config.FoodApiKey)
	if strings.Contains(key, "%") {
		if dk, err := url.QueryUnescape(key); err == nil {
			return dk
		}
	}
	return key
}
