package services

// 알림 스케줄러 — 저녁 8시/아침 9시에 서버가 직접 푸시를 보낸다.
// 채널: ntfy(NTFY_TOPIC 설정 시) + 웹푸시(VAPID 설정 + 구독 존재 시).
// 기존 조회형 /api/notify/* (홈 배너·iOS 단축어)는 그대로 유지 — 이 스케줄러는 발송만 추가.
// 컨테이너 TZ=Asia/Seoul 전제(docker-compose 에 설정) — time.Local 기준으로 계산한다.

import (
	"fmt"
	"time"

	"dashboard/clients"
	"dashboard/controllers/rest"
	"dashboard/global/config"
	"dashboard/global/log"
)

const dashboardURL = "https://dashboard.gowoobro.com"

var notifySchedule = []struct {
	hour int
	mode string
}{
	{9, "morning"},
	{20, "evening"},
}

// StartNotifyScheduler 는 채널이 하나라도 구성돼 있을 때만 스케줄러 goroutine 을 띄운다.
func StartNotifyScheduler() {
	if config.NtfyTopic == "" && config.VapidPrivateKey == "" {
		log.Info().Str("service", "notify-scheduler").Msg("disabled (NTFY_TOPIC / VAPID 키 미설정)")
		return
	}
	log.Info().Str("service", "notify-scheduler").Msg("Start Service (09:00 morning / 20:00 evening)")
	go runNotifyScheduler()
}

func runNotifyScheduler() {
	for {
		mode, next := nextNotifyRun(time.Now())
		time.Sleep(time.Until(next))
		sendScheduledNotify(mode)
	}
}

// nextNotifyRun 은 now 이후 가장 가까운 발송 시각과 모드를 반환한다.
func nextNotifyRun(now time.Time) (string, time.Time) {
	var best time.Time
	mode := ""
	for _, s := range notifySchedule {
		t := time.Date(now.Year(), now.Month(), now.Day(), s.hour, 0, 0, 0, time.Local)
		if !t.After(now) {
			t = t.AddDate(0, 0, 1)
		}
		if best.IsZero() || t.Before(best) {
			best = t
			mode = s.mode
		}
	}
	return mode, best
}

func sendScheduledNotify(mode string) {
	// DB 순단 등으로 죽어도 스케줄러 루프는 살아야 한다.
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("service", "notify-scheduler").Msg(fmt.Sprintf("panic: %v", r))
		}
	}()

	title, body, notify := rest.RunNotifyCheck(mode)
	if !notify {
		log.Info().Str("service", "notify-scheduler").Str("mode", mode).Msg("skip (알림 불필요)")
		return
	}
	if body == "" {
		body = title
		title = "대시보드"
	}

	priority := 3 // 아침 보고는 기본
	if mode == "evening" {
		priority = 4 // 저녁 경고는 high
	}

	if err := clients.SendNtfy(title, body, dashboardURL, priority); err != nil {
		log.Error().Str("service", "notify-scheduler").Msg(err.Error())
	}
	clients.SendWebPush(title, body, dashboardURL)
	log.Info().Str("service", "notify-scheduler").Str("mode", mode).Str("title", title).Msg("sent")
}
