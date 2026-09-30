package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.
// /api/diet/* — 식단 일지 조회·검색·입력(노션에 쓰기)·목표.

import (
	"dashboard/clients"
	"dashboard/controllers/rest"

	"github.com/gofiber/fiber/v2"
)

func SetupDietRoutes(group fiber.Router) {

	run := func(c *fiber.Ctx, fn func(ctl *rest.DietController)) error {
		var controller rest.DietController
		controller.Init(c)
		fn(&controller)
		controller.Close()
		return c.JSON(controller.Result)
	}

	group.Get("/diet/day", func(c *fiber.Ctx) error {
		return run(c, func(ctl *rest.DietController) { ctl.Day(c.Query("date")) })
	})

	group.Get("/diet/summary", func(c *fiber.Ctx) error {
		return run(c, func(ctl *rest.DietController) { ctl.Summary(c.QueryInt("days", 28)) })
	})

	group.Get("/diet/search", func(c *fiber.Ctx) error {
		return run(c, func(ctl *rest.DietController) { ctl.Search(c.Query("q")) })
	})

	group.Post("/diet/log", func(c *fiber.Ctx) error {
		var in clients.DietLogInput
		if err := c.BodyParser(&in); err != nil {
			return c.JSON(fiber.Map{"code": "error", "message": "요청 형식 오류"})
		}
		return run(c, func(ctl *rest.DietController) { ctl.SaveLog(in) })
	})

	group.Post("/diet/log/delete", func(c *fiber.Ctx) error {
		var in struct {
			Id string `json:"id"`
		}
		c.BodyParser(&in)
		return run(c, func(ctl *rest.DietController) { ctl.DeleteLog(in.Id) })
	})

	group.Post("/diet/copy", func(c *fiber.Ctx) error {
		var in struct {
			From string `json:"from"`
			To   string `json:"to"`
			Meal string `json:"meal"`
		}
		c.BodyParser(&in)
		return run(c, func(ctl *rest.DietController) { ctl.Copy(in.From, in.To, in.Meal) })
	})

	group.Get("/diet/targets", func(c *fiber.Ctx) error {
		return run(c, func(ctl *rest.DietController) { ctl.Targets() })
	})

	group.Post("/diet/targets", func(c *fiber.Ctx) error {
		var in clients.DietTargets
		if err := c.BodyParser(&in); err != nil {
			return c.JSON(fiber.Map{"code": "error", "message": "요청 형식 오류"})
		}
		return run(c, func(ctl *rest.DietController) { ctl.SaveTargets(in) })
	})

}
