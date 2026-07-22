package clients

// GitHub 컨트리뷰션 집계.
// - 히트맵: GraphQL contributionsCollection.contributionCalendar (1콜에 1년치)
// - 최근 활동: REST /users/:login/events
// GITHUB_TOKEN 이 비어 있으면 빈 결과를 반환한다(에러 아님).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"dashboard/global/config"
)

var githubClient = &http.Client{Timeout: 15 * time.Second}

// DayCount 는 하루치 컨트리뷰션 수.
type DayCount struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Count int    `json:"count"`
}

// Activity 는 최근 활동 한 건.
type Activity struct {
	Source string `json:"source"` // 'github' | 'gitlab'
	Type   string `json:"type"`   // 'push' | 'pr' | 'mr' | 'issue' | ...
	Repo   string `json:"repo"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Date   string `json:"date"` // RFC3339
}

// FetchGithubCalendar 는 from~to 구간의 일별 컨트리뷰션을 가져온다.
func FetchGithubCalendar(from, to time.Time) ([]DayCount, error) {
	if config.GithubToken == "" {
		return nil, nil
	}

	query := `query($login:String!,$from:DateTime!,$to:DateTime!){
      user(login:$login){
        contributionsCollection(from:$from,to:$to){
          contributionCalendar{
            weeks{ contributionDays{ date contributionCount } }
          }
        }
      }
    }`

	body, _ := json.Marshal(map[string]interface{}{
		"query": query,
		"variables": map[string]string{
			"login": config.GithubUsername,
			"from":  from.Format(time.RFC3339),
			"to":    to.Format(time.RFC3339),
		},
	})

	req, err := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.GithubToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := githubClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	buf, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github graphql: status %v", res.StatusCode)
	}

	var parsed struct {
		Data struct {
			User struct {
				ContributionsCollection struct {
					ContributionCalendar struct {
						Weeks []struct {
							ContributionDays []struct {
								Date              string `json:"date"`
								ContributionCount int    `json:"contributionCount"`
							} `json:"contributionDays"`
						} `json:"weeks"`
					} `json:"contributionCalendar"`
				} `json:"contributionsCollection"`
			} `json:"user"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("github graphql: %s", parsed.Errors[0].Message)
	}

	var days []DayCount
	for _, week := range parsed.Data.User.ContributionsCollection.ContributionCalendar.Weeks {
		for _, d := range week.ContributionDays {
			days = append(days, DayCount{Date: d.Date, Count: d.ContributionCount})
		}
	}
	return days, nil
}

// fetchGithubCommitMessage 는 커밋 SHA 로 메시지 첫 줄을 가져온다 (실패 시 빈 문자열).
// 결과는 5분 캐시(dev_recent) 뒤에 있으므로 이벤트당 1콜이어도 부담 없다.
func fetchGithubCommitMessage(repo, sha string) string {
	req, err := http.NewRequest("GET",
		fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", repo, sha), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+config.GithubToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	res, err := githubClient.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	buf, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return ""
	}

	var parsed struct {
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return ""
	}
	// 여러 줄 커밋 메시지는 제목(첫 줄)만
	message := parsed.Commit.Message
	if idx := strings.IndexByte(message, '\n'); idx > 0 {
		message = message[:idx]
	}
	return message
}

// FetchGithubRecent 는 최근 push/PR/issue 활동을 가져온다.
func FetchGithubRecent() ([]Activity, error) {
	if config.GithubToken == "" {
		return nil, nil
	}

	req, err := http.NewRequest("GET",
		fmt.Sprintf("https://api.github.com/users/%s/events?per_page=30", config.GithubUsername), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.GithubToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	res, err := githubClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	buf, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github events: status %v", res.StatusCode)
	}

	var events []struct {
		Type string `json:"type"`
		Repo struct {
			Name string `json:"name"`
		} `json:"repo"`
		Payload struct {
			Commits []struct {
				Message string `json:"message"`
			} `json:"commits"`
			Head        string `json:"head"` // PushEvent 커밋 SHA (commits 배열은 더 이상 안 옴)
			PullRequest struct {
				Title   string `json:"title"`
				HTMLURL string `json:"html_url"`
			} `json:"pull_request"`
			Issue struct {
				Title   string `json:"title"`
				HTMLURL string `json:"html_url"`
			} `json:"issue"`
			Action string `json:"action"`
			Ref    string `json:"ref"`
		} `json:"payload"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(buf, &events); err != nil {
		return nil, err
	}

	var activities []Activity
	for _, e := range events {
		a := Activity{Source: "github", Repo: e.Repo.Name, Date: e.CreatedAt}
		repoURL := "https://github.com/" + e.Repo.Name
		switch e.Type {
		case "PushEvent":
			a.Type = "push"
			if len(e.Payload.Commits) > 0 {
				a.Title = e.Payload.Commits[len(e.Payload.Commits)-1].Message
			} else if e.Payload.Head != "" {
				// events API 가 commits 를 더 이상 포함하지 않음 — head SHA 로 메시지 조회
				a.Title = fetchGithubCommitMessage(e.Repo.Name, e.Payload.Head)
				a.URL = repoURL + "/commit/" + e.Payload.Head
			}
			if a.Title == "" {
				a.Title = "커밋 푸시"
			}
			if a.URL == "" {
				a.URL = repoURL
			}
		case "PullRequestEvent":
			a.Type = "pr"
			a.Title = e.Payload.PullRequest.Title
			a.URL = e.Payload.PullRequest.HTMLURL
		case "IssuesEvent":
			a.Type = "issue"
			a.Title = e.Payload.Issue.Title
			a.URL = e.Payload.Issue.HTMLURL
		case "CreateEvent":
			a.Type = "create"
			a.Title = e.Payload.Ref + " 생성"
			a.URL = repoURL
		default:
			continue // 관심 없는 이벤트는 건너뛴다
		}
		if a.URL == "" {
			a.URL = repoURL
		}
		activities = append(activities, a)
	}
	return activities, nil
}
