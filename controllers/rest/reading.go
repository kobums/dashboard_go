package rest

// 독서 현황 요약 — snippetapi 프록시 (clients.FetchReadingSummary + 캐시).
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"encoding/json"
	"fmt"
	"time"

	"dashboard/controllers"
	"dashboard/clients"
)

type ReadingController struct {
	controllers.Controller
}

func (c *ReadingController) Summary(year int, month int) {
	now := time.Now()
	if year == 0 {
		year = now.Year()
	}
	if month == 0 {
		month = int(now.Month())
	}

	key := fmt.Sprintf("reading_summary_%v_%v", year, month)
	buf, err := clients.GetCached(key, 10*time.Minute, func() ([]byte, error) {
		return clients.FetchReadingSummary(year, month)
	})
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("item", json.RawMessage(buf))
}

// Daily — 전체 기간 일별 독서 집계 (분·페이지·세션 수)
func (c *ReadingController) Daily() {
	buf, err := clients.GetCached("reading_daily", 10*time.Minute, clients.FetchReadingDaily)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("items", json.RawMessage(buf))
}

// Books — 완독한 책 목록 (최근 완독 순, 연도 필터는 프론트)
func (c *ReadingController) Books() {
	buf, err := clients.GetCached("reading_books", 10*time.Minute, clients.FetchReadingBooks)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("items", json.RawMessage(buf))
}
