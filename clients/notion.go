package clients

// 노션 공식 API 클라이언트 — data source 조회 + 페이지 생성/수정/휴지통 (웨이트 기록 입력용).
// NOTION_TOKEN 은 내부 integration 시크릿(쓰기에는 「콘텐츠 업데이트·입력」 기능 필요). 「운동 계획」 페이지에 integration 을 연결해야
// 하위 DB(운동 캘린더/운동 일지)를 읽을 수 있다.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dashboard/global/config"
)

const notionVersion = "2025-09-03" // data source API (/v1/data_sources/{id}/query)

var notionClient = &http.Client{Timeout: 20 * time.Second}

// NotionPage 는 query 결과의 페이지 1개. Properties 는 이름 → 원본 JSON.
type NotionPage struct {
	Id         string                     `json:"id"`
	Url        string                     `json:"url"`
	Properties map[string]json.RawMessage `json:"properties"`
}

// notionQueryAll 은 data source 의 모든 페이지를 페이지네이션하며 가져온다 (휴지통 제외).
func notionQueryAll(dataSourceId string) ([]NotionPage, error) {
	return notionQuery(dataSourceId, nil, nil)
}

// notionQuery 는 filter/sorts 를 붙여 조회한다 (nil 이면 생략).
func notionQuery(dataSourceId string, filter interface{}, sorts interface{}) ([]NotionPage, error) {
	var all []NotionPage
	cursor := ""
	for page := 0; page < 100; page++ { // 100×100 = 1만 행 안전장치
		body := map[string]interface{}{"page_size": 100}
		if filter != nil {
			body["filter"] = filter
		}
		if sorts != nil {
			body["sorts"] = sorts
		}
		if cursor != "" {
			body["start_cursor"] = cursor
		}
		var res struct {
			Results    []NotionPage `json:"results"`
			HasMore    bool         `json:"has_more"`
			NextCursor string       `json:"next_cursor"`
		}
		if err := notionDo("POST", "/v1/data_sources/"+dataSourceId+"/query", body, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Results...)
		if !res.HasMore || res.NextCursor == "" {
			return all, nil
		}
		cursor = res.NextCursor
	}
	return all, nil
}

// notionCreatePage 는 data source 에 새 페이지를 만들고 page id 를 반환한다.
func notionCreatePage(dataSourceId string, props map[string]interface{}) (string, error) {
	var res NotionPage
	err := notionDo("POST", "/v1/pages", map[string]interface{}{
		"parent":     map[string]string{"type": "data_source_id", "data_source_id": dataSourceId},
		"properties": props,
	}, &res)
	return res.Id, err
}

func notionUpdatePage(pageId string, props map[string]interface{}) error {
	return notionDo("PATCH", "/v1/pages/"+pageId, map[string]interface{}{"properties": props}, &NotionPage{})
}

// notionTrashPage 는 페이지를 휴지통으로 보낸다 (노션에서 30일 안에 복원 가능).
func notionTrashPage(pageId string) error {
	return notionDo("PATCH", "/v1/pages/"+pageId, map[string]interface{}{"in_trash": true}, &NotionPage{})
}

func notionGetPage(pageId string) (NotionPage, error) {
	var p NotionPage
	err := notionDo("GET", "/v1/pages/"+pageId, nil, &p)
	return p, err
}

type NotionSelectOption struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// notionSelectOptions 는 data source 스키마에서 select 속성의 옵션들을 읽는다.
func notionSelectOptions(dataSourceId, property string) ([]NotionSelectOption, error) {
	var ds struct {
		Properties map[string]struct {
			Select struct {
				Options []NotionSelectOption `json:"options"`
			} `json:"select"`
		} `json:"properties"`
	}
	if err := notionDo("GET", "/v1/data_sources/"+dataSourceId, nil, &ds); err != nil {
		return nil, err
	}
	return ds.Properties[property].Select.Options, nil
}

// notionDo 는 429(rate limit, 평균 3req/s)면 Retry-After 만큼 쉬고 최대 3번 재시도한다.
func notionDo(method, path string, body interface{}, out interface{}) error {
	var buf []byte
	if body != nil {
		buf, _ = json.Marshal(body)
	}
	for attempt := 0; attempt < 3; attempt++ {
		var reader io.Reader
		if buf != nil {
			reader = bytes.NewReader(buf)
		}
		req, err := http.NewRequest(method, "https://api.notion.com"+path, reader)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+config.NotionToken)
		req.Header.Set("Notion-Version", notionVersion)
		req.Header.Set("Content-Type", "application/json")

		res, err := notionClient.Do(req)
		if err != nil {
			return err
		}
		data, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode == http.StatusTooManyRequests {
			wait, _ := strconv.Atoi(res.Header.Get("Retry-After"))
			if wait <= 0 {
				wait = 1
			}
			time.Sleep(time.Duration(wait) * time.Second)
			continue
		}
		if res.StatusCode != http.StatusOK {
			var e struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			json.Unmarshal(data, &e)
			return fmt.Errorf("notion %v: %v %v", res.StatusCode, e.Code, e.Message)
		}
		return json.Unmarshal(data, out)
	}
	return fmt.Errorf("notion: rate limited")
}

// ── 속성 값 만들기 (쓰기용) ────────────────────────────────────────────────

func notionRichTextValue(s string) map[string]interface{} {
	items := []interface{}{}
	if s != "" {
		items = append(items, map[string]interface{}{"text": map[string]string{"content": s}})
	}
	return map[string]interface{}{"rich_text": items}
}

func notionTitleValue(s string) map[string]interface{} {
	return map[string]interface{}{"title": []interface{}{map[string]interface{}{"text": map[string]string{"content": s}}}}
}

// notionSelectValue 는 빈 값이면 선택 해제. 없는 옵션 이름이면 노션이 옵션을 새로 만든다.
func notionSelectValue(s string) map[string]interface{} {
	if s == "" {
		return map[string]interface{}{"select": nil}
	}
	return map[string]interface{}{"select": map[string]string{"name": s}}
}

func notionMultiSelectValue(names []string) map[string]interface{} {
	items := []interface{}{}
	for _, n := range names {
		items = append(items, map[string]string{"name": n})
	}
	return map[string]interface{}{"multi_select": items}
}

// notionNumberValue 는 0 을 빈 칸으로 쓴다 (유산소 칸이 근력 행에 0 으로 찍히지 않게).
func notionNumberValue(v float64) map[string]interface{} {
	if v == 0 {
		return map[string]interface{}{"number": nil}
	}
	return map[string]interface{}{"number": v}
}

func notionDateValue(date string) map[string]interface{} {
	return map[string]interface{}{"date": map[string]string{"start": date}}
}

func notionRelationValue(ids ...string) map[string]interface{} {
	items := []interface{}{}
	for _, id := range ids {
		items = append(items, map[string]string{"id": id})
	}
	return map[string]interface{}{"relation": items}
}

// ── 속성 값 꺼내기 (없거나 타입이 다르면 zero value) ────────────────────────

type notionRichText struct {
	PlainText string `json:"plain_text"`
}

func joinRichText(items []notionRichText) string {
	var b strings.Builder
	for _, t := range items {
		b.WriteString(t.PlainText)
	}
	return strings.TrimSpace(b.String())
}

func (p NotionPage) Text(name string) string {
	var v struct {
		Type     string           `json:"type"`
		Title    []notionRichText `json:"title"`
		RichText []notionRichText `json:"rich_text"`
	}
	if json.Unmarshal(p.Properties[name], &v) != nil {
		return ""
	}
	if v.Type == "title" {
		return joinRichText(v.Title)
	}
	return joinRichText(v.RichText)
}

func (p NotionPage) Select(name string) string {
	var v struct {
		Select *struct {
			Name string `json:"name"`
		} `json:"select"`
	}
	if json.Unmarshal(p.Properties[name], &v) != nil || v.Select == nil {
		return ""
	}
	return v.Select.Name
}

func (p NotionPage) MultiSelect(name string) []string {
	var v struct {
		MultiSelect []struct {
			Name string `json:"name"`
		} `json:"multi_select"`
	}
	json.Unmarshal(p.Properties[name], &v)
	out := make([]string, 0, len(v.MultiSelect))
	for _, o := range v.MultiSelect {
		out = append(out, o.Name)
	}
	return out
}

func (p NotionPage) Number(name string) float64 {
	var v struct {
		Number *float64 `json:"number"`
	}
	if json.Unmarshal(p.Properties[name], &v) != nil || v.Number == nil {
		return 0
	}
	return *v.Number
}

// Date 는 date 속성의 시작일을 YYYY-MM-DD 로 반환한다.
func (p NotionPage) Date(name string) string {
	var v struct {
		Date *struct {
			Start string `json:"start"`
		} `json:"date"`
	}
	if json.Unmarshal(p.Properties[name], &v) != nil || v.Date == nil || len(v.Date.Start) < 10 {
		return ""
	}
	return v.Date.Start[:10]
}

func (p NotionPage) Relation(name string) []string {
	var v struct {
		Relation []struct {
			Id string `json:"id"`
		} `json:"relation"`
	}
	json.Unmarshal(p.Properties[name], &v)
	out := make([]string, 0, len(v.Relation))
	for _, r := range v.Relation {
		out = append(out, r.Id)
	}
	return out
}
