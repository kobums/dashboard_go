package rest

// 개발 현황 — github/gitlab 집계 (clients.FetchDevSummary/Recent + 캐시).
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	"encoding/json"
	"fmt"
	"time"

	"dashboard/clients"
	"dashboard/controllers"
)

type DevController struct {
	controllers.Controller
}

// Summary — days 1~400: 해당 일수, days<=0 또는 >400: 전체 기간(백필 포함).
func (c *DevController) Summary(days int) {
	if days < 0 || days > 400 {
		days = 0
	}
	key := fmt.Sprintf("dev_summary_%v", days)
	buf, err := clients.GetCached(key, 60*time.Minute, func() ([]byte, error) {
		return clients.FetchDevSummary(days)
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("item", json.RawMessage(buf))
}

func (c *DevController) Recent() {
	buf, err := clients.GetCached("dev_recent", 60*time.Minute, func() ([]byte, error) {
		return clients.FetchDevRecent()
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("items", json.RawMessage(buf))
}

type devMonthly struct {
	Month int `json:"month"`
	Total int `json:"total"`
}

type devYear struct {
	Year       int          `json:"year"`
	Github     int          `json:"github"`
	Gitlab     int          `json:"gitlab"`
	Total      int          `json:"total"`
	ActiveDays int          `json:"activeDays"`
	Monthly    []devMonthly `json:"monthly"`
}

// Yearly 는 devstat_tb(백필 포함)에서 연도별 컨트리뷰션을 집계한다. 외부 API 안 씀.
func (c *DevController) Yearly() {
	conn := c.NewConnection()

	years := map[int]*devYear{}
	getYear := func(y int) *devYear {
		if _, ok := years[y]; !ok {
			monthly := make([]devMonthly, 12)
			for m := range monthly {
				monthly[m].Month = m + 1
			}
			years[y] = &devYear{Year: y, Monthly: monthly}
		}
		return years[y]
	}

	rows, err := conn.Query(
		"SELECT YEAR(ds_statdate), MONTH(ds_statdate), ds_source, SUM(ds_count), COUNT(*) " +
			"FROM devstat_tb WHERE ds_count > 0 GROUP BY YEAR(ds_statdate), MONTH(ds_statdate), ds_source")
	if err != nil {
		c.Error(err)
		return
	}
	defer rows.Close()

	// (연,월,일) 별 활동일은 소스 합산 시 같은 날 이중 카운트될 수 있어
	// 활동일은 별도 쿼리로 정확히 센다.
	for rows.Next() {
		var y, m, sum, cnt int
		var source string
		if err := rows.Scan(&y, &m, &source, &sum, &cnt); err != nil {
			continue
		}
		yf := getYear(y)
		if source == "github" {
			yf.Github += sum
		} else if source == "gitlab" {
			yf.Gitlab += sum
		}
		yf.Total += sum
		if m >= 1 && m <= 12 {
			yf.Monthly[m-1].Total += sum
		}
	}

	arows, err := conn.Query(
		"SELECT YEAR(ds_statdate), COUNT(DISTINCT ds_statdate) FROM devstat_tb WHERE ds_count > 0 GROUP BY YEAR(ds_statdate)")
	if err == nil {
		defer arows.Close()
		for arows.Next() {
			var y, days int
			if err := arows.Scan(&y, &days); err != nil {
				continue
			}
			getYear(y).ActiveDays = days
		}
	}

	list := make([]*devYear, 0, len(years))
	for y := time.Now().Year(); y >= 2000; y-- {
		if yf, ok := years[y]; ok {
			list = append(list, yf)
		}
	}
	c.Set("years", list)
}
