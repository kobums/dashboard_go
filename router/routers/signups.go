package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.

import (
	"strconv"

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

	// /summary 보다 뒤에 등록해야 "summary" 가 :key 로 잡히지 않는다.
	group.Get("/signups/:key", func(c *fiber.Ctx) error {
		service := rest.FindSignupService(c.Params("key"))
		if service == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"code": "error", "message": "unknown service"})
		}
		days, _ := strconv.Atoi(c.Query("days"))
		var controller rest.SignupsController
		controller.Init(c)
		controller.Detail(service, days)
		controller.Close()
		return c.JSON(controller.Result)
	})

}
