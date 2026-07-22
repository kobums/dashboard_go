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
