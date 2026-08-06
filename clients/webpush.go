package clients

// 웹푸시(PWA) 발송 — iOS 16.4+ 는 홈 화면에 추가한 PWA 에만 웹푸시가 온다.
// 구독 정보는 fetchcache_tb 에 JSON 배열로 보관 — 전용 테이블 없이 재배포에도 유지된다.
// 캐시가 아니라 영구 저장 용도라 GetCached(TTL/stale 갱신) 대신 매니저를 직접 쓴다.
// VAPID_PRIVATE_KEY 가 비어 있으면 조용히 건너뛴다 (opt-in).

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"dashboard/global/config"
	"dashboard/global/log"
	"dashboard/models"
)

const pushSubsKey = "push_subscriptions"

var pushMutex sync.Mutex // 구독 read-modify-write 직렬화 (싱글 유저라 경합은 드물지만 안전하게)

func loadSubscriptions(conn *models.Connection) []webpush.Subscription {
	manager := models.NewFetchcacheManager(conn)
	item := manager.GetWhere([]interface{}{models.Where{Column: "cachekey", Value: pushSubsKey, Compare: "="}})
	if item == nil || item.Payload == "" {
		return []webpush.Subscription{}
	}
	var subs []webpush.Subscription
	if err := json.Unmarshal([]byte(item.Payload), &subs); err != nil {
		log.Error().Str("push", pushSubsKey).Msg(err.Error())
		return []webpush.Subscription{}
	}
	return subs
}

func saveSubscriptions(conn *models.Connection, subs []webpush.Subscription) {
	buf, err := json.Marshal(subs)
	if err != nil {
		log.Error().Str("push", pushSubsKey).Msg(err.Error())
		return
	}
	manager := models.NewFetchcacheManager(conn)
	now := time.Now().Format("2006-01-02 15:04:05")
	item := manager.GetWhere([]interface{}{models.Where{Column: "cachekey", Value: pushSubsKey, Compare: "="}})
	if item == nil {
		manager.Insert(&models.Fetchcache{Cachekey: pushSubsKey, Payload: string(buf), Fetchedat: now})
	} else {
		item.Payload = string(buf)
		item.Fetchedat = now
		manager.Update(item)
	}
}

// AddPushSubscription 은 브라우저 구독을 저장한다 (endpoint 기준 중복 갱신).
func AddPushSubscription(sub webpush.Subscription) {
	pushMutex.Lock()
	defer pushMutex.Unlock()

	conn := models.NewConnection()
	defer conn.Close()

	subs := loadSubscriptions(conn)
	out := make([]webpush.Subscription, 0, len(subs)+1)
	for _, s := range subs {
		if s.Endpoint != sub.Endpoint {
			out = append(out, s)
		}
	}
	out = append(out, sub)
	saveSubscriptions(conn, out)
}

// RemovePushSubscription 은 endpoint 로 구독을 제거한다.
func RemovePushSubscription(endpoint string) {
	pushMutex.Lock()
	defer pushMutex.Unlock()

	conn := models.NewConnection()
	defer conn.Close()

	subs := loadSubscriptions(conn)
	out := make([]webpush.Subscription, 0, len(subs))
	for _, s := range subs {
		if s.Endpoint != endpoint {
			out = append(out, s)
		}
	}
	saveSubscriptions(conn, out)
}

// CountPushSubscriptions 는 저장된 구독 수를 반환한다 (설정 UI 상태 표시용).
func CountPushSubscriptions() int {
	pushMutex.Lock()
	defer pushMutex.Unlock()

	conn := models.NewConnection()
	defer conn.Close()

	return len(loadSubscriptions(conn))
}

// SendWebPush 는 저장된 모든 구독으로 알림을 보낸다.
// 404/410(만료·해지된 구독)은 목록에서 자동 제거한다.
func SendWebPush(title, body, url string) {
	if config.VapidPrivateKey == "" || config.VapidPublicKey == "" {
		return
	}

	pushMutex.Lock()
	defer pushMutex.Unlock()

	conn := models.NewConnection()
	defer conn.Close()

	subs := loadSubscriptions(conn)
	if len(subs) == 0 {
		return
	}

	payload, err := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"url":   url,
	})
	if err != nil {
		log.Error().Str("push", "payload").Msg(err.Error())
		return
	}

	options := &webpush.Options{
		// mailto: 접두어 금지 — webpush-go 가 자동으로 붙인다. 중복되면 sub 클레임이
		// "mailto:mailto:…" 가 되고 Apple(APNs)이 403 BadJwtToken 으로 거절한다.
		Subscriber:      "kobums23@gmail.com",
		VAPIDPublicKey:  config.VapidPublicKey,
		VAPIDPrivateKey: config.VapidPrivateKey,
		TTL:             3600,
		Urgency:         webpush.UrgencyHigh,
	}

	alive := make([]webpush.Subscription, 0, len(subs))
	changed := false
	for i := range subs {
		sub := subs[i]
		resp, err := webpush.SendNotification(payload, &sub, options)
		if err != nil {
			log.Error().Str("push", "send").Msg(err.Error())
			alive = append(alive, sub) // 일시 오류일 수 있으니 유지
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			log.Info().Str("push", "send").Msg("subscription expired — removed")
			changed = true // 만료된 구독 — 버린다
			continue
		}
		if resp.StatusCode >= 300 {
			// APNs/FCM 거절 사유는 응답 본문에 담긴다 (BadJwtToken, VapidPkHashMismatch 등)
			log.Error().Str("push", "send").Msg(fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(body)))
			alive = append(alive, sub)
			continue
		}
		log.Info().Str("push", "send").Msg(fmt.Sprintf("ok status=%d", resp.StatusCode))
		alive = append(alive, sub)
	}
	if changed {
		saveSubscriptions(conn, alive)
	}
}
