package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.

import (
	"dashboard/controllers/rest"

	"github.com/gofiber/fiber/v2"
)

func SetupCompareRoutes(group fiber.Router) {

	group.Get("/compare", func(c *fiber.Ctx) error {
		var controller rest.CompareController
		controller.Init(c)
		controller.Compare(c.Query("date"))
		controller.Close()
		return c.JSON(controller.Result)
	})

}
