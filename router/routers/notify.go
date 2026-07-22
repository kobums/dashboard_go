package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.
// 웹 배너(Bearer)와 iOS 단축어(api-key)가 함께 쓰므로 AnyTokenRequired.

import (
	"dashboard/controllers/rest"

	"github.com/gofiber/fiber/v2"
)

func SetupNotifyRoutes(group fiber.Router) {

	group.Get("/notify/check", AnyTokenRequired, func(c *fiber.Ctx) error {
		var controller rest.NotifyController
		controller.Init(c)
		controller.Check(c.Query("mode"))
		controller.Close()
		return c.JSON(controller.Result)
	})

	// iOS 단축어용 — 알림 불필요면 빈 응답, 필요하면 알림 문장(plain text)만.
	// 단축어가 "URL 콘텐츠 가져오기 → 값이 있으면 → 알림 표시" 3개 액션으로 끝난다.
	group.Get("/notify/text", AnyTokenRequired, func(c *fiber.Ctx) error {
		var controller rest.NotifyController
		controller.Init(c)
		text := controller.CheckText(c.Query("mode"))
		controller.Close()
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(text)
	})

}
