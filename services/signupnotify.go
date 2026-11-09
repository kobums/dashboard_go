package services

// 신규 가입 알림 — 5분마다 서비스별 새 가입을 확인해 ntfy + 웹푸시로 알린다.
// 워터마크(서비스별 마지막으로 본 가입 시각)는 fetchcache_tb 의 signup_notify_state 키에 영구 저장 —
// 재시작해도 이미 알린 가입을 다시 보내지 않는다. 첫 실행은 기준선만 잡는다.
// 조용한 시간(0~7시)에는 워터마크를 저장하지 않아 그동안의 가입이 8시 첫 회차에 한 번에 나간다.
// 채널 설정은 알림 스케줄러와 같다(NTFY_TOPIC / VAPID 키, 둘 다 없으면 비활성).

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dashboard/clients"
	"dashboard/controllers/rest"
	"dashboard/global/config"
	"dashboard/global/log"
	"dashboard/models"
)

const (
	signupNotifyInterval = 5 * time.Minute
	signupNotifyStateKey = "signup_notify_state"
	signupQuietUntilHour = 8 // 0시 ~ 이 시각 전에는 보내지 않고 모아 둔다
)

func StartSignupNotify() {
	if config.NtfyTopic == "" && config.VapidPrivateKey == "" {
		log.Info().Str("service", "signup-notify").Msg("disabled (NTFY_TOPIC / VAPID 키 미설정)")
		return
	}
	log.Info().Str("service", "signup-notify").Msg("Start Service (every 5m, quiet 00-08)")
	go func() {
		for {
			runSignupNotify(time.Now())
			time.Sleep(signupNotifyInterval)
		}
	}()
}

func runSignupNotify(now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("service", "signup-notify").Msg(fmt.Sprintf("panic: %v", r))
		}
	}()

	conn := models.NewConnection()
	defer conn.Close()

	state, ok := loadSignupNotifyState(conn)
	if !ok {
		return // 상태를 못 읽으면 기준선을 잘못 잡아 중복 발송할 수 있으니 이번 회차는 건너뜀
	}
	found, next := rest.NewSignupsSince(state)
	if len(found) > 0 && now.Hour() < signupQuietUntilHour {
		return // 조용한 시간 — 워터마크를 두고 8시에 몰아서
	}

	if len(found) > 0 {
		title, body, click := signupNotifyMessage(found)
		if err := clients.SendNtfy(title, body, click, 3); err != nil {
			log.Error().Str("service", "signup-notify").Msg(err.Error())
			return // 발송 실패 — 워터마크를 두고 다음 회차에 재시도
		}
		clients.SendWebPush(title, body, click)
		log.Info().Str("service", "signup-notify").Str("title", title).Msg("sent")
	}
	saveSignupNotifyState(conn, next)
}

// signupNotifyMessage — "신규 가입 +3" / "Fotstat +2 (오늘 3명)\nSnippet +1 (오늘 1명)".
// 서비스가 하나면 그 상세로, 여럿이면 서비스 목록으로 연결.
func signupNotifyMessage(found []rest.NewSignup) (string, string, string) {
	total := 0
	lines := make([]string, 0, len(found))
	for _, f := range found {
		total += f.Count
		lines = append(lines, fmt.Sprintf("%s +%d (오늘 %d명)", f.Name, f.Count, f.Today))
	}
	click := dashboardURL + "/services"
	if len(found) == 1 {
		click += "/" + found[0].Key
	}
	return fmt.Sprintf("신규 가입 +%d", total), strings.Join(lines, "\n"), click
}

func loadSignupNotifyState(conn *models.Connection) (map[string]string, bool) {
	if !conn.IsConnect() {
		return nil, false
	}
	manager := models.NewFetchcacheManager(conn)
	item := manager.GetWhere([]interface{}{models.Where{Column: "cachekey", Value: signupNotifyStateKey, Compare: "="}})
	state := map[string]string{}
	if item == nil || item.Payload == "" {
		return state, true // 첫 실행 — 전부 기준선만 잡힌다
	}
	if err := json.Unmarshal([]byte(item.Payload), &state); err != nil {
		log.Error().Str("service", "signup-notify").Msg(err.Error())
		return nil, false
	}
	return state, true
}

func saveSignupNotifyState(conn *models.Connection, state map[string]string) {
	buf, err := json.Marshal(state)
	if err != nil {
		return
	}
	manager := models.NewFetchcacheManager(conn)
	now := time.Now().Format("2006-01-02 15:04:05")
	item := manager.GetWhere([]interface{}{models.Where{Column: "cachekey", Value: signupNotifyStateKey, Compare: "="}})
	if item == nil {
		manager.Insert(&models.Fetchcache{Cachekey: signupNotifyStateKey, Payload: string(buf), Fetchedat: now})
	} else {
		item.Payload = string(buf)
		item.Fetchedat = now
		manager.Update(item)
	}
}
