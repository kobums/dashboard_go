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
