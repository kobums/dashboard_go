package rest

// 웹푸시 구독 관리 + 시험 발송.
// 수기 작성 파일: buildtool-model 재생성에 덮이지 않는다.

import (
	webpush "github.com/SherClockHolmes/webpush-go"

	"dashboard/clients"
	"dashboard/controllers"
	"dashboard/global/config"
)

type PushController struct {
	controllers.Controller
}

// Status 는 프론트 설정 UI 용 — VAPID 공개키와 채널 구성 여부.
func (c *PushController) Status() {
	c.Set("publicKey", config.VapidPublicKey)
	c.Set("webpushConfigured", config.VapidPublicKey != "" && config.VapidPrivateKey != "")
	c.Set("ntfyConfigured", config.NtfyTopic != "")
	c.Set("subscriptions", clients.CountPushSubscriptions())
}

// Subscribe 는 브라우저 PushSubscription JSON 을 저장한다.
func (c *PushController) Subscribe() {
	var sub webpush.Subscription
	if err := c.Bind(&sub); err != nil || sub.Endpoint == "" {
		c.Set("code", "error")
		c.Set("message", "잘못된 구독 정보입니다.")
		return
	}
	clients.AddPushSubscription(sub)
	c.Set("subscriptions", clients.CountPushSubscriptions())
}

// Unsubscribe 는 endpoint 로 구독을 제거한다.
func (c *PushController) Unsubscribe() {
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := c.Bind(&body); err != nil || body.Endpoint == "" {
		c.Set("code", "error")
		c.Set("message", "endpoint 가 필요합니다.")
		return
	}
	clients.RemovePushSubscription(body.Endpoint)
	c.Set("subscriptions", clients.CountPushSubscriptions())
}

// Test 는 구성된 모든 채널(웹푸시 + ntfy)로 시험 알림을 보낸다.
func (c *PushController) Test() {
	sent := []string{}
	if config.VapidPrivateKey != "" && clients.CountPushSubscriptions() > 0 {
		clients.SendWebPush("대시보드 알림 테스트", "웹푸시가 정상 동작합니다 ✓", "https://dashboard.gowoobro.com")
		sent = append(sent, "webpush")
	}
	if config.NtfyTopic != "" {
		if err := clients.SendNtfy("대시보드 알림 테스트", "ntfy 가 정상 동작합니다 ✓", "https://dashboard.gowoobro.com", 4); err != nil {
			c.Set("code", "error")
			c.Set("message", err.Error())
			return
		}
		sent = append(sent, "ntfy")
	}
	c.Set("sent", sent)
}
