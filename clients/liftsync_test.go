package clients

import (
	"encoding/json"
	"testing"
)

// 노션 API(2025-09-03) data source query 결과 페이지 모양 그대로.
const notionLogPageJSON = `{
  "id": "3ea3ae0b-75f5-81eb-99d2-ebfba0dcaea7",
  "url": "https://www.notion.so/3ea3ae0b75f581eb99d2ebfba0dcaea7",
  "properties": {
    "운동": {"id":"title","type":"title","title":[{"type":"text","plain_text":"벤치프레스"}]},
    "종목": {"id":"a","type":"select","select":{"id":"x","name":"벤치프레스","color":"red"}},
    "부위": {"id":"b","type":"select","select":{"id":"y","name":"가슴","color":"red"}},
    "날짜": {"id":"c","type":"date","date":{"start":"2026-09-28","end":null,"time_zone":null}},
    "웜업": {"id":"d","type":"rich_text","rich_text":[{"type":"text","plain_text":"20×20, 40×20"}]},
    "본세트": {"id":"e","type":"rich_text","rich_text":[{"type":"text","plain_text":"50×15,11,10 / "},{"type":"text","plain_text":"60×4,4"}]},
    "목표": {"id":"f","type":"rich_text","rich_text":[{"type":"text","plain_text":"50×12~15×4"}]},
    "시간(분)": {"id":"g","type":"number","number":null},
    "컨디션": {"id":"h","type":"select","select":null},
    "메모": {"id":"i","type":"rich_text","rich_text":[]},
    "운동일": {"id":"j","type":"relation","relation":[{"id":"3ea3ae0b-75f5-81ea-9757-e12675d1f576"}],"has_more":false}
  }
}`

func TestLiftExerciseFromPage(t *testing.T) {
	var p NotionPage
	if err := json.Unmarshal([]byte(notionLogPageJSON), &p); err != nil {
		t.Fatal(err)
	}
	e, ok := liftExerciseFromPage(p)
	if !ok {
		t.Fatal("row skipped")
	}
	if e.name != "벤치프레스" || e.part != "가슴" || e.date != "2026-09-28" {
		t.Errorf("basic fields: %+v", e)
	}
	if e.mainset != "50×15,11,10 / 60×4,4" || e.target != "50×12~15×4" {
		t.Errorf("rich text join: main=%q target=%q", e.mainset, e.target)
	}
	if e.dayNotionId != "3ea3ae0b-75f5-81ea-9757-e12675d1f576" {
		t.Errorf("relation: %q", e.dayNotionId)
	}
	if e.warmupCount != 2 || len(e.sets) != 7 || e.parseError != "" {
		t.Errorf("sets: warmup=%d total=%d err=%q", e.warmupCount, len(e.sets), e.parseError)
	}
	if e.minutes != 0 || e.condition != "" {
		t.Errorf("null values should be zero: %+v", e)
	}
}

func TestLiftExerciseFromPageParseError(t *testing.T) {
	var p NotionPage
	json.Unmarshal([]byte(notionLogPageJSON), &p)
	p.Properties["본세트"] = json.RawMessage(`{"type":"rich_text","rich_text":[{"plain_text":"50kg 열다섯개"}]}`)
	e, ok := liftExerciseFromPage(p)
	if !ok || e.parseError == "" {
		t.Errorf("parse error should be kept on the row: ok=%v err=%q", ok, e.parseError)
	}
	if e.warmupCount != 2 || len(e.sets) != 2 {
		t.Errorf("warmup sets should survive a main-set error: %d/%d", e.warmupCount, len(e.sets))
	}
}

func TestLiftExerciseFromPageEmptyMainSet(t *testing.T) {
	var p NotionPage
	json.Unmarshal([]byte(notionLogPageJSON), &p)
	p.Properties["본세트"] = json.RawMessage(`{"type":"rich_text","rich_text":[]}`)
	if e, _ := liftExerciseFromPage(p); e.parseError == "" {
		t.Error("strength row without 본세트 should be flagged")
	}
	// 유산소는 본세트가 없는 게 정상
	p.Properties["부위"] = json.RawMessage(`{"type":"select","select":{"name":"유산소"}}`)
	p.Properties["웜업"] = json.RawMessage(`{"type":"rich_text","rich_text":[]}`)
	p.Properties["목표"] = json.RawMessage(`{"type":"rich_text","rich_text":[]}`)
	if e, _ := liftExerciseFromPage(p); e.parseError != "" {
		t.Errorf("cardio row should not be flagged: %q", e.parseError)
	}
}
