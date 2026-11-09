package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.

import (
	"dashboard/controllers/rest"

	"github.com/gofiber/fiber/v2"
)

func SetupSignupsRoutes(group fiber.Router) {

	group.Get("/signups/summary", func(c *fiber.Ctx) error {
		var controller rest.SignupsController
		controller.Init(c)
		controller.Summary()
		controller.Close()
		return c.JSON(controller.Result)
	})

}
