package clients

// GitLab(gitlab.com) 컨트리뷰션 집계.
// 계획: 이벤트 API(/api/v4/users/:id/events)를 기본 구현으로, 일별 버킷팅.
//   (undocumented calendar.json 은 쓰지 않는다 — 세션 의존적이라 불안정)
// GITLAB_TOKEN 또는 GITLAB_USERNAME 이 비어 있으면 빈 결과.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dashboard/global/config"
	"dashboard/global/log"
)

var gitlabClient = &http.Client{Timeout: 15 * time.Second}

const gitlabBase = "https://gitlab.com/api/v4"

func gitlabConfigured() bool {
	return config.GitlabToken != "" && config.GitlabUsername != ""
}

func gitlabGet(path string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", gitlabBase+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("PRIVATE-TOKEN", config.GitlabToken)

	res, err := gitlabClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	buf, _ := io.ReadAll(res.Body)
	return buf, res.StatusCode, nil
}

var gitlabUserID int

// resolveGitlabUserID 는 username → numeric id 를 한 번 조회해 캐시한다.
func resolveGitlabUserID() (int, error) {
	if gitlabUserID != 0 {
		return gitlabUserID, nil
	}
	buf, status, err := gitlabGet("/users?username=" + config.GitlabUsername)
	if err != nil {
		return 0, err
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("gitlab users: status %v", status)
	}
	var users []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(buf, &users); err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, fmt.Errorf("gitlab user %q not found", config.GitlabUsername)
	}
	gitlabUserID = users[0].ID
	return gitlabUserID, nil
}

type gitlabEvent struct {
	ActionName  string `json:"action_name"`
	TargetType  string `json:"target_type"`
	TargetTitle string `json:"target_title"`
	CreatedAt   string `json:"created_at"`
	ProjectID   int    `json:"project_id"`
	PushData    struct {
		CommitTitle string `json:"commit_title"`
		Ref         string `json:"ref"`
	} `json:"push_data"`
}

// fetchGitlabEvents 는 after 이후의 이벤트를 페이지네이션으로 모두 가져온다.
// maxPages/timeout 은 호출 목적별 상한 (일반 요청 20p/20s, 백필은 크게).
func fetchGitlabEvents(after time.Time, maxPages int, timeout time.Duration) ([]gitlabEvent, error) {
	uid, err := resolveGitlabUserID()
	if err != nil {
		return nil, err
	}

	var all []gitlabEvent
	afterStr := after.Format("2006-01-02")
	deadline := time.Now().Add(timeout) // 전체 페이지네이션 상한
	for page := 1; page <= maxPages; page++ {
		if time.Now().After(deadline) {
			log.Info().Int("pages", page-1).Msg("gitlab pagination deadline reached")
			break
		}
		path := fmt.Sprintf("/users/%d/events?after=%s&per_page=100&page=%d", uid, afterStr, page)
		buf, status, err := gitlabGet(path)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("gitlab events: status %v", status)
		}
		var events []gitlabEvent
		if err := json.Unmarshal(buf, &events); err != nil {
			return nil, err
		}
		if len(events) == 0 {
			break
		}
		all = append(all, events...)
		if len(events) < 100 {
			break
		}
	}
	return all, nil
}

// FetchGitlabCalendar 는 from~to 구간 이벤트를 일별 카운트로 버킷팅한다.
func FetchGitlabCalendar(from, to time.Time) ([]DayCount, error) {
	return fetchGitlabCalendarLimits(from, 20, 20*time.Second)
}

// FetchGitlabCalendarAll 은 백필용 — GitLab 이 보존한 전체 이벤트를 가져온다.
func FetchGitlabCalendarAll(from time.Time) ([]DayCount, error) {
	return fetchGitlabCalendarLimits(from, 200, 3*time.Minute)
}

func fetchGitlabCalendarLimits(from time.Time, maxPages int, timeout time.Duration) ([]DayCount, error) {
	if !gitlabConfigured() {
		return nil, nil
	}

	events, err := fetchGitlabEvents(from, maxPages, timeout)
	if err != nil {
		return nil, err
	}

	buckets := map[string]int{}
	for _, e := range events {
		if len(e.CreatedAt) < 10 {
			continue
		}
		day := e.CreatedAt[:10]
		buckets[day]++
	}

	var days []DayCount
	for day, count := range buckets {
		days = append(days, DayCount{Date: day, Count: count})
	}
	return days, nil
}

// FetchGitlabRecent 는 최근 활동을 가져온다(첫 페이지).
func FetchGitlabRecent() ([]Activity, error) {
	if !gitlabConfigured() {
		return nil, nil
	}

	uid, err := resolveGitlabUserID()
	if err != nil {
		return nil, err
	}
	buf, status, err := gitlabGet(fmt.Sprintf("/users/%d/events?per_page=30", uid))
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("gitlab events: status %v", status)
	}
	var events []gitlabEvent
	if err := json.Unmarshal(buf, &events); err != nil {
		return nil, err
	}

	var activities []Activity
	for _, e := range events {
		a := Activity{Source: "gitlab", Date: e.CreatedAt}
		switch {
		case e.PushData.CommitTitle != "":
			a.Type = "push"
			a.Title = e.PushData.CommitTitle
		case e.TargetType == "MergeRequest":
			a.Type = "mr"
			a.Title = e.TargetTitle
		case e.TargetType == "Issue":
			a.Type = "issue"
			a.Title = e.TargetTitle
		default:
			a.Type = "activity"
			a.Title = e.ActionName
			if e.TargetTitle != "" {
				a.Title = e.TargetTitle
			}
		}
		activities = append(activities, a)
	}
	return activities, nil
}
