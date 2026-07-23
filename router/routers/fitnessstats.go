package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.
// /api/fitness/* — 생성 라우터의 /workout/:id 와 경로 충돌을 피하기 위해 분리.

import (
	"dashboard/controllers/rest"

	"github.com/gofiber/fiber/v2"
)

func SetupFitnessRoutes(group fiber.Router) {

	group.Get("/fitness/yearly", func(c *fiber.Ctx) error {
		var controller rest.FitnessController
		controller.Init(c)
		controller.Yearly()
		controller.Close()
		return c.JSON(controller.Result)
	})

}
