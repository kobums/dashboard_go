package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.

import (
	"strconv"

	"dashboard/controllers/rest"

	"github.com/gofiber/fiber/v2"
)

func SetupDevRoutes(group fiber.Router) {

	group.Get("/dev/summary", func(c *fiber.Ctx) error {
		days_, _ := strconv.Atoi(c.Query("days"))
		var controller rest.DevController
		controller.Init(c)
		controller.Summary(days_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Get("/dev/recent", func(c *fiber.Ctx) error {
		var controller rest.DevController
		controller.Init(c)
		controller.Recent()
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Get("/dev/yearly", func(c *fiber.Ctx) error {
		var controller rest.DevController
		controller.Init(c)
		controller.Yearly()
		controller.Close()
		return c.JSON(controller.Result)
	})

}
