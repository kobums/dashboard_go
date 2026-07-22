package routers

// 수기 작성 라우터: buildtool 재생성에 덮이지 않는다.
// - SetupHealthIngestRoutes: api-key 인증 (TokenRequired 등록 전에 연결할 것)
// - SetupHealthRoutes: DASH_TOKEN 인증 구간에 연결

import (
	"dashboard/controllers/rest"
	"dashboard/global/log"
	"dashboard/models"

	"github.com/gofiber/fiber/v2"
)

func SetupHealthIngestRoutes(group fiber.Router) {

	group.Post("/health/ingest", IngestTokenRequired, func(c *fiber.Ctx) error {
		var controller rest.HealthIngestController
		controller.Init(c)
		controller.Ingest(c.Body())
		controller.Close()
		return c.JSON(controller.Result)
	})

	// iOS 기본 단축어(Shortcuts)용 — 평평한 JSON 수신 (같은 api-key 인증)
	group.Post("/health/shortcut", IngestTokenRequired, func(c *fiber.Ctx) error {
		var controller rest.HealthIngestController
		controller.Init(c)
		controller.IngestShortcut(c.Body())
		controller.Close()
		return c.JSON(controller.Result)
	})

}

func SetupHealthRoutes(group fiber.Router) {

	group.Get("/health/metrics", func(c *fiber.Ctx) error {
		from_ := c.Query("from")
		to_ := c.Query("to")
		name_ := c.Query("name")

		var args []interface{}
		if from_ != "" {
			args = append(args, models.Where{Column: "metricdate", Value: from_, Compare: ">="})
		}
		to__ := to_
		if to__ != "" {
			args = append(args, models.Where{Column: "metricdate", Value: to__, Compare: "<="})
		}
		if name_ != "" {
			args = append(args, models.Where{Column: "name", Value: name_, Compare: "="})
		}
		args = append(args, models.Ordering("hm_metricdate asc"))

		conn := models.NewConnection()
		if conn == nil {
			log.Error().Msg("db connection failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"code": "error"})
		}
		defer conn.Close()

		manager := models.NewHealthmetricManager(conn)
		items := manager.Find(args)
		return c.JSON(fiber.Map{"code": "ok", "items": items})
	})

}
