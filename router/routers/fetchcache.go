package routers

import (

	"strconv"

	"dashboard/global/log"


	"dashboard/controllers/rest"


	"dashboard/models"
	"github.com/gofiber/fiber/v2"
)

// SetupFetchcacheRoutes sets up routes for fetchcache domain
func SetupFetchcacheRoutes(group fiber.Router) {

	group.Get("/fetchcache", func(c *fiber.Ctx) error {
		page_, _ := strconv.Atoi(c.Query("page"))
		pagesize_, _ := strconv.Atoi(c.Query("pagesize"))
		var controller rest.FetchcacheController
		controller.Init(c)
		controller.Index(page_, pagesize_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Get("/fetchcache/:id", func(c *fiber.Ctx) error {
		id_, _ := strconv.ParseInt(c.Params("id"), 10, 64)
		var controller rest.FetchcacheController
		controller.Init(c)
		controller.Read(id_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/fetchcache", func(c *fiber.Ctx) error {
		item_ := &models.Fetchcache{}
		err := c.BodyParser(item_)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.FetchcacheController
		controller.Init(c)
		controller.Insert(item_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/fetchcache/batch", func(c *fiber.Ctx) error {
		var items_ *[]models.Fetchcache
		items__ref := &items_
		err := c.BodyParser(items__ref)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.FetchcacheController
		controller.Init(c)
		controller.Insertbatch(items_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Post("/fetchcache/count", func(c *fiber.Ctx) error {

		var controller rest.FetchcacheController
		controller.Init(c)
		controller.Count()
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Put("/fetchcache", func(c *fiber.Ctx) error {
		item_ := &models.Fetchcache{}
		err := c.BodyParser(item_)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.FetchcacheController
		controller.Init(c)
		controller.Update(item_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Delete("/fetchcache", func(c *fiber.Ctx) error {
		item_ := &models.Fetchcache{}
		err := c.BodyParser(item_)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.FetchcacheController
		controller.Init(c)
		controller.Delete(item_)
		controller.Close()
		return c.JSON(controller.Result)
	})

	group.Delete("/fetchcache/batch", func(c *fiber.Ctx) error {
		item_ := &[]models.Fetchcache{}
		err := c.BodyParser(item_)
		if err != nil {
		    log.Error().Msg(err.Error())
		}
		var controller rest.FetchcacheController
		controller.Init(c)
		controller.Deletebatch(item_)
		controller.Close()
		return c.JSON(controller.Result)
	})

}