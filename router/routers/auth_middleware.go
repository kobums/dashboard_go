package routers

// 싱글 유저 대시보드 인증.
// - TokenRequired: Authorization: Bearer <DASH_TOKEN> 정적 토큰 검증 (프론트 전용).
// - IngestTokenRequired: api-key 헤더로 Health Auto Export 수신 전용 토큰 검증.
//   폰에는 ingest 토큰만 저장되므로 유출돼도 대시보드 조회는 불가능하다.

import (
	"crypto/subtle"
	stdtime "time"

	"dashboard/global/config"

	"github.com/gofiber/fiber/v2"
)

// TokenRequired 는 인증이 필요한 라우트 앞에 붙는 미들웨어다.
// 토큰이 없거나 유효하지 않으면 401 을 반환한다.
func TokenRequired(c *fiber.Ctx) error {
	token := c.Get("Authorization")
	const prefix = "Bearer "
	if len(token) > len(prefix) && token[:len(prefix)] == prefix {
		token = token[len(prefix):]
	}

	if config.DashToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(config.DashToken)) != 1 {
		return errorJSON(c, fiber.StatusUnauthorized, "인증이 필요합니다.")
	}
	return c.Next()
}

// IngestTokenRequired 는 /api/health/ingest 전용 미들웨어다.
func IngestTokenRequired(c *fiber.Ctx) error {
	token := c.Get("api-key")
	if config.HealthIngestToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(config.HealthIngestToken)) != 1 {
		return errorJSON(c, fiber.StatusUnauthorized, "인증이 필요합니다.")
	}
	return c.Next()
}

// AnyTokenRequired 는 DASH_TOKEN(Bearer) 또는 HEALTH_INGEST_TOKEN(api-key)
// 둘 중 하나면 통과 — 웹(배너)과 iOS 단축어가 같은 엔드포인트를 쓰는 알림 API 용.
func AnyTokenRequired(c *fiber.Ctx) error {
	token := c.Get("Authorization")
	const prefix = "Bearer "
	if len(token) > len(prefix) && token[:len(prefix)] == prefix {
		token = token[len(prefix):]
	}
	if config.DashToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(config.DashToken)) == 1 {
		return c.Next()
	}

	apiKey := c.Get("api-key")
	if config.HealthIngestToken != "" && subtle.ConstantTimeCompare([]byte(apiKey), []byte(config.HealthIngestToken)) == 1 {
		return c.Next()
	}

	return errorJSON(c, fiber.StatusUnauthorized, "인증이 필요합니다.")
}

// errorJSON 은 공통 에러 응답 JSON 을 반환한다.
func errorJSON(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"status":      status,
		"message":     message,
		"fieldErrors": fiber.Map{},
		"timestamp":   stdtime.Now().Format("2006-01-02T15:04:05"),
	})
}
