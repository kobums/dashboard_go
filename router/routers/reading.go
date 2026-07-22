package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.

import (
	"strconv"

	"dashboard/controllers/rest"

	"github.com/gofiber/fiber/v2"
)

func SetupReadingRoutes(group fiber.Router) {

	group.Get("/reading/summary", func(c *fiber.Ctx) error {
		year_, _ := strconv.Atoi(c.Query("year"))
		month_, _ := strconv.Atoi(c.Query("month"))
		var controller rest.ReadingController
		controller.Init(c)
		controller.Summary(year_, month_)
		controller.Close()
		return c.JSON(controller.Result)
	})

}
