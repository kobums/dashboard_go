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

}
