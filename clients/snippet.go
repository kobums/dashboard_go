package clients

// snippetapi.gowoobro.com 프록시 클라이언트.
// SNIPPET_EMAIL/PASSWORD 로 로그인해 JWT 를 메모리에 캐시하고, 401 이면 재로그인한다.
// (refresh 토큰은 쓰지 않는다 — 싱글 유저라 재로그인이 더 단순하다)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"dashboard/global/config"
	"dashboard/global/log"
)

var snippetClient = &http.Client{Timeout: 15 * time.Second}

var snippetToken string
var snippetMutex sync.Mutex

type snippetAuthResponse struct {
	Token string `json:"token"`
}

func snippetLogin() error {
	snippetMutex.Lock()
	defer snippetMutex.Unlock()

	body, _ := json.Marshal(map[string]string{
		"email":    config.SnippetEmail,
		"password": config.SnippetPassword,
	})

	res, err := snippetClient.Post(config.SnippetApiUrl+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("snippet login failed: status %v", res.StatusCode)
	}

	var auth snippetAuthResponse
	if err := json.NewDecoder(res.Body).Decode(&auth); err != nil {
		return err
	}
	if auth.Token == "" {
		return fmt.Errorf("snippet login: empty token")
	}

	snippetToken = auth.Token
	return nil
}

// snippetGet 은 인증 GET 요청을 보내고, 401 이면 한 번 재로그인 후 재시도한다.
func snippetGet(path string) ([]byte, error) {
	if snippetToken == "" {
		if err := snippetLogin(); err != nil {
			return nil, err
		}
	}

	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequest("GET", config.SnippetApiUrl+path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+snippetToken)

		res, err := snippetClient.Do(req)
		if err != nil {
			return nil, err
		}

		buf, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return nil, err
		}

		if res.StatusCode == http.StatusUnauthorized && attempt == 0 {
			if err := snippetLogin(); err != nil {
				return nil, err
			}
			continue
		}
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("snippet GET %v: status %v", path, res.StatusCode)
		}

		return buf, nil
	}

	return nil, fmt.Errorf("snippet GET %v: unauthorized", path)
}

// FetchReadingDaily 는 전체 독서 세션을 일별로 집계한다: [{date, minutes, pages, sessions}]
// pages 는 세션 페이지와 읽은 페이지 로그 중 큰 값 — 로그만 있는 날은 minutes·sessions 가 0 이다.
func FetchReadingDaily() ([]byte, error) {
	buf, err := snippetGet("/readingsessions")
	if err != nil {
		return nil, err
	}

	var sessions []struct {
		DurationSeconds int    `json:"durationSeconds"`
		PagesRead       int    `json:"pagesRead"`
		SessionDate     string `json:"sessionDate"`
	}
	if err := json.Unmarshal(buf, &sessions); err != nil {
		return nil, err
	}

	type dailyAgg struct {
		Minutes  int `json:"minutes"`
		Pages    int `json:"pages"`
		Sessions int `json:"sessions"`
	}
	byDay := map[string]*dailyAgg{}
	for _, s := range sessions {
		if len(s.SessionDate) < 10 {
			continue
		}
		day := s.SessionDate[:10]
		if _, ok := byDay[day]; !ok {
			byDay[day] = &dailyAgg{}
		}
		byDay[day].Minutes += s.DurationSeconds / 60
		byDay[day].Pages += s.PagesRead
		byDay[day].Sessions++
	}

	// 타이머 없이 진행률만 올린 날도 반영 — 같은 날 세션·로그가 겹치면(세션 저장도 로그를 남김) 큰 쪽을 쓴다
	logPages, err := fetchReadingLogPages()
	if err != nil {
		log.Error().Str("path", "/readinglogs/daily").Msg(err.Error())
	}
	for day, pages := range logPages {
		if _, ok := byDay[day]; !ok {
			byDay[day] = &dailyAgg{}
		}
		if pages > byDay[day].Pages {
			byDay[day].Pages = pages
		}
	}

	type dailyRow struct {
		Date     string `json:"date"`
		Minutes  int    `json:"minutes"`
		Pages    int    `json:"pages"`
		Sessions int    `json:"sessions"`
	}
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)
	rows := make([]dailyRow, 0, len(days))
	for _, d := range days {
		agg := byDay[d]
		rows = append(rows, dailyRow{Date: d, Minutes: agg.Minutes, Pages: agg.Pages, Sessions: agg.Sessions})
	}
	return json.Marshal(rows)
}

// snippet 읽은 페이지 로그(readinglog_tb)가 쌓이기 시작한 달 — 이전 달은 조회해도 비어 있다.
const readingLogSinceYear, readingLogSinceMonth = 2026, 10

type readingLogDay struct {
	Date  string `json:"date"`
	Pages int    `json:"pages"`
}

// fetchReadingLogMonth 는 /readinglogs/daily 한 달치: 날짜(YYYY-MM-DD) → 그날 읽은 페이지.
// snippet 이 (책, 날짜)별 순증가를 0 하한으로 합산해 주므로 오타 수정·재독으로 음수가 되지 않는다.
func fetchReadingLogMonth(year int, month int) (map[string]int, error) {
	buf, err := snippetGet(fmt.Sprintf("/readinglogs/daily?year=%v&month=%v", year, month))
	if err != nil {
		return nil, err
	}
	var days []readingLogDay
	if err := json.Unmarshal(buf, &days); err != nil {
		return nil, err
	}
	pages := map[string]int{}
	for _, d := range days {
		if len(d.Date) >= 10 {
			pages[d.Date[:10]] += d.Pages
		}
	}
	return pages, nil
}

// fetchReadingLogPages 는 readingLogSince ~ 이번 달 일별 읽은 페이지를 모은다.
// 한 달이라도 실패하면 그때까지 모은 값과 에러를 함께 돌려준다 (세션 집계는 막지 않는다).
func fetchReadingLogPages() (map[string]int, error) {
	pages := map[string]int{}
	now := time.Now()
	for m := time.Date(readingLogSinceYear, time.Month(readingLogSinceMonth), 1, 0, 0, 0, 0, time.Local); !m.After(now); m = m.AddDate(0, 1, 0) {
		month, err := fetchReadingLogMonth(m.Year(), int(m.Month()))
		if err != nil {
			return pages, err
		}
		for d, p := range month {
			pages[d] += p
		}
	}
	return pages, nil
}

// FetchReadingBooks 는 완독한 책 목록을 만든다 (연도 필터는 프론트에서).
// 완독 판정은 status(completed, 옛 값 done)로만 한다 — snippet 은 읽을 예정·읽는 중·중단 책에도
// endDate 를 채워 두므로 endDate 유무로는 구분되지 않는다.
func FetchReadingBooks() ([]byte, error) {
	buf, err := snippetGet("/userbooks")
	if err != nil {
		return nil, err
	}

	var books []struct {
		Title    string `json:"title"`
		Author   string `json:"author"`
		CoverUrl string `json:"coverUrl"`
		Status   string `json:"status"`
		Rating   *int   `json:"rating"`
		EndDate  string `json:"endDate"`
	}
	if err := json.Unmarshal(buf, &books); err != nil {
		return nil, err
	}

	type doneBook struct {
		Title    string `json:"title"`
		Author   string `json:"author"`
		CoverUrl string `json:"coverUrl"`
		Rating   int    `json:"rating"`
		EndDate  string `json:"endDate"`
	}
	done := []doneBook{}
	for _, b := range books {
		if b.Status != "done" && b.Status != "completed" {
			continue
		}
		rating := 0
		if b.Rating != nil {
			rating = *b.Rating
		}
		endDate := b.EndDate
		if len(endDate) > 10 {
			endDate = endDate[:10]
		}
		done = append(done, doneBook{Title: b.Title, Author: b.Author, CoverUrl: b.CoverUrl, Rating: rating, EndDate: endDate})
	}
	// 최근 완독 순
	sort.Slice(done, func(i, j int) bool { return done[i].EndDate > done[j].EndDate })
	return json.Marshal(done)
}

// FetchReadingSummary 는 snippet 통계 API 들을 모아 대시보드용 요약 JSON 을 만든다.
func FetchReadingSummary(year int, month int) ([]byte, error) {
	paths := map[string]string{
		"monthlyStats":  fmt.Sprintf("/userbooks/stats/monthly?year=%v", year),
		"yearlyStats":   "/userbooks/stats/yearly",
		"categoryStats": fmt.Sprintf("/userbooks/stats/category?year=%v", year),
		"insights":      fmt.Sprintf("/userbooks/stats/insights?year=%v", year),
		"streak":        "/readingsessions/streak",
		"goals":         "/readinggoals",
		"progress":      fmt.Sprintf("/userbooks/progress?year=%v&month=%v", year, month),
	}

	summary := make(map[string]json.RawMessage)
	success := 0
	for key, path := range paths {
		buf, err := snippetGet(path)
		if err != nil {
			// 일부 실패는 전체를 막지 않는다 — 해당 키만 null 로 둔다.
			log.Error().Str("path", path).Msg(err.Error())
			summary[key] = json.RawMessage("null")
			continue
		}
		summary[key] = json.RawMessage(buf)
		success++
	}

	// 전부 실패했으면 (대개 로그인 설정 문제) 빈 요약을 캐시하지 않고 에러를 낸다.
	if success == 0 {
		return nil, fmt.Errorf("snippet fetch failed for all endpoints — check snippetEmail/snippetPassword")
	}

	summary["year"], _ = json.Marshal(year)
	summary["month"], _ = json.Marshal(month)

	return json.Marshal(summary)
}
