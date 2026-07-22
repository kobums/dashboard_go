package router

import (
	"dashboard/router/routers"

	"github.com/gofiber/fiber/v2"
)

func SetRouter(r *fiber.App) {
	apiGroup := r.Group("/api")

	// 인증 불필요 구간 — TokenRequired 등록 전에 연결해야 한다.
	apiGroup.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"code": "ok"})
	})
	routers.SetupHealthIngestRoutes(apiGroup) // api-key 자체 인증

	// 이후 라우트는 전부 DASH_TOKEN Bearer 인증.
	apiGroup.Use(routers.TokenRequired)

	routers.SetupReadingRoutes(apiGroup)
	routers.SetupHealthRoutes(apiGroup)
	routers.SetupWorkoutRoutes(apiGroup)
	routers.SetupFitnessRoutes(apiGroup)
	routers.SetupDevRoutes(apiGroup)
	routers.SetupUploadRoutes(apiGroup)
}
