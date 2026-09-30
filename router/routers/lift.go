package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.
// /api/lift/* — 노션 운동 기록(웨이트) 조회 + 수동 동기화 + 입력(노션에 쓰기).

import (
	"dashboard/clients"
	"dashboard/controllers/rest"

	"github.com/gofiber/fiber/v2"
)

func SetupLiftRoutes(group fiber.Router) {

	group.Get("/lift/overview", func(c *fiber.Ctx) error {
		var controller rest.LiftController
		controller.Init(c)
		controller.Overview()
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Get("/lift/exercise", func(c *fiber.Ctx) error {
		var controller rest.LiftController
		controller.Init(c)
		controller.Exercise(c.Query("name"))
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Get("/lift/calendar", func(c *fiber.Ctx) error {
		var controller rest.LiftController
		controller.Init(c)
		controller.Calendar(c.Query("month"))
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/lift/sync", func(c *fiber.Ctx) error {
		var controller rest.LiftController
		controller.Init(c)
		controller.Sync()
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Get("/lift/form", func(c *fiber.Ctx) error {
		var controller rest.LiftController
		controller.Init(c)
		controller.Form(c.Query("date"))
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/lift/log", func(c *fiber.Ctx) error {
		var in clients.LiftLogInput
		if err := c.BodyParser(&in); err != nil {
			return c.JSON(fiber.Map{"code": "error", "message": "요청 형식 오류"})
		}
		var controller rest.LiftController
		controller.Init(c)
		controller.SaveLog(in)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/lift/log/delete", func(c *fiber.Ctx) error {
		var in struct {
			Id string `json:"id"`
		}
		c.BodyParser(&in)
		var controller rest.LiftController
		controller.Init(c)
		controller.DeleteLog(in.Id)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/lift/day", func(c *fiber.Ctx) error {
		var in struct {
			Date      string `json:"date"`
			Condition string `json:"condition"`
		}
		c.BodyParser(&in)
		var controller rest.LiftController
		controller.Init(c)
		controller.SetDay(in.Date, in.Condition)
		controller.Close()
		return c.JSON(controller.Result)
	})

}
