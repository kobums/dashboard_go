package routers

import (

	"strconv"

	"dashboard/global/log"


	"dashboard/controllers/rest"


	"dashboard/models"
	"github.com/gofiber/fiber/v2"
)

// SetupHealthmetricRoutes sets up routes for healthmetric domain
func SetupHealthmetricRoutes(group fiber.Router) {

	group.Get("/healthmetric", func(c *fiber.Ctx) error {
		page_, _ := strconv.Atoi(c.Query("page"))
		pagesize_, _ := strconv.Atoi(c.Query("pagesize"))
		var controller rest.HealthmetricController
		controller.Init(c)
		controller.Index(page_, pagesize_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Get("/healthmetric/:id", func(c *fiber.Ctx) error {
		id_, _ := strconv.ParseInt(c.Params("id"), 10, 64)
		var controller rest.HealthmetricController
		controller.Init(c)
		controller.Read(id_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/healthmetric", func(c *fiber.Ctx) error {
		item_ := &models.Healthmetric{}
		err := c.BodyParser(item_)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.HealthmetricController
		controller.Init(c)
		controller.Insert(item_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/healthmetric/batch", func(c *fiber.Ctx) error {
		var items_ *[]models.Healthmetric
		items__ref := &items_
		err := c.BodyParser(items__ref)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.HealthmetricController
		controller.Init(c)
		controller.Insertbatch(items_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/healthmetric/count", func(c *fiber.Ctx) error {

		var controller rest.HealthmetricController
		controller.Init(c)
		controller.Count()
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Put("/healthmetric", func(c *fiber.Ctx) error {
		item_ := &models.Healthmetric{}
		err := c.BodyParser(item_)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.HealthmetricController
		controller.Init(c)
		controller.Update(item_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Delete("/healthmetric", func(c *fiber.Ctx) error {
		item_ := &models.Healthmetric{}
		err := c.BodyParser(item_)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.HealthmetricController
		controller.Init(c)
		controller.Delete(item_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Delete("/healthmetric/batch", func(c *fiber.Ctx) error {
		item_ := &[]models.Healthmetric{}
		err := c.BodyParser(item_)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.HealthmetricController
		controller.Init(c)
		controller.Deletebatch(item_)
		controller.Close()
		return c.JSON(controller.Result)
	})

}