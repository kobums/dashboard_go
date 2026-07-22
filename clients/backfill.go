package clients

// 개발 컨트리뷰션 과거 전체 백필 (일회성 — cmd/backfill 에서 호출).
// - GitHub: 계정 생성 연도부터 올해까지 연도별 contributionCalendar (GraphQL 은 1콜 최대 1년)
// - GitLab: events API 가 보존한 범위 전체 (서버측 보존 한계까지)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dashboard/global/config"
	"dashboard/global/log"
	"dashboard/models"
)

// fetchGithubCreatedAt 은 GitHub 계정 생성일을 조회한다.
func fetchGithubCreatedAt() (time.Time, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"query":     `query($login:String!){ user(login:$login){ createdAt } }`,
		"variables": map[string]string{"login": config.GithubUsername},
	})
	req, err := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewReader(body))
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+config.GithubToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := githubClient.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer res.Body.Close()
	buf, _ := io.ReadAll(res.Body)

	var parsed struct {
		Data struct {
			User struct {
				CreatedAt string `json:"createdAt"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, parsed.Data.User.CreatedAt)
}

// BackfillDev 는 GitHub 전체 히스토리 + GitLab 보존 범위를 devstat_tb 에 채운다.
// 반환: 소스별 저장한 일수.
func BackfillDev() (map[string]int, error) {
	conn := models.NewConnection()
	if conn == nil {
		return nil, fmt.Errorf("db connection failed")
	}
	defer conn.Close()

	result := map[string]int{}

	// --- GitHub: 계정 생성 연도부터 연도별 ---
	if config.GithubToken != "" {
		created, err := fetchGithubCreatedAt()
		if err != nil {
			return nil, fmt.Errorf("github createdAt: %w", err)
		}
		now := time.Now()
		log.Info().Str("createdAt", created.Format("2006-01-02")).Msg("github account")

		for year := created.Year(); year <= now.Year(); year++ {
			from := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC)
			if to.After(now) {
				to = now
			}
			days, err := FetchGithubCalendar(from, to)
			if err != nil {
				log.Error().Int("year", year).Msg(err.Error())
				continue
			}
			// 0인 날은 저장 불필요 (빈 날은 조회 시 0으로 채워짐)
			var nonZero []DayCount
			for _, d := range days {
				if d.Count > 0 {
					nonZero = append(nonZero, d)
				}
			}
			upsertDevstat(conn, "github", nonZero)
			result["github"] += len(nonZero)
			log.Info().Int("year", year).Int("activeDays", len(nonZero)).Msg("github backfill")
		}
	}

	// --- GitLab: 보존된 범위 전체 ---
	if config.GitlabToken != "" && config.GitlabUsername != "" {
		days, err := FetchGitlabCalendarAll(time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC))
		if err != nil {
			log.Error().Msg("gitlab backfill: " + err.Error())
		} else {
			upsertDevstat(conn, "gitlab", days)
			result["gitlab"] = len(days)
			log.Info().Int("activeDays", len(days)).Msg("gitlab backfill")
		}
	}

	return result, nil
}
