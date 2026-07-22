package routers

import (
	"dashboard/controllers/api"

	"github.com/gofiber/fiber/v2"
)

// SetupUploadRoutes sets up routes for projects domain
func SetupUploadRoutes(group fiber.Router) {

	group.Post("/upload/index", func(c *fiber.Ctx) error {
		var controller api.UploadController
		controller.Init(c)
		controller.Index()
		controller.Close()
		return c.JSON(controller.Result)
	})

}