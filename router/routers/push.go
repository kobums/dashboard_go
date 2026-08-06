package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.
// 웹푸시 구독 관리 — 프론트 전용이라 DASH_TOKEN(Bearer) 그룹 아래 등록된다.

import (
	"dashboard/controllers/rest"

	"github.com/gofiber/fiber/v2"
)

func SetupPushRoutes(group fiber.Router) {

	group.Get("/push/status", func(c *fiber.Ctx) error {
		var controller rest.PushController
		controller.Init(c)
		controller.Status()
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/push/subscribe", func(c *fiber.Ctx) error {
		var controller rest.PushController
		controller.Init(c)
		controller.Subscribe()
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/push/unsubscribe", func(c *fiber.Ctx) error {
		var controller rest.PushController
		controller.Init(c)
		controller.Unsubscribe()
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/push/test", func(c *fiber.Ctx) error {
		var controller rest.PushController
		controller.Init(c)
		controller.Test()
		controller.Close()
		return c.JSON(controller.Result)
	})

}
