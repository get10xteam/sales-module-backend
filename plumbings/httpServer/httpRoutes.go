package httpServer

import (
	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/config"

	"github.com/gofiber/fiber/v2"
)

func registerRoutes(config config.RuntimeConfigType) {
	apiRouter := server.Group("api")

	apiRoutes(apiRouter)

	if config.ServeFrontEndDir == "" {
		// universal notFound catcher
		server.Use(func(c *fiber.Ctx) error {
			return c.JSON(errs.ErrRouteNotFound())
		})
	} else {
		apiRouter.Use(func(c *fiber.Ctx) error {
			return c.JSON(errs.ErrRouteNotFound())
		})
		server.Static("", config.ServeFrontEndDir)
		server.Use(func(c *fiber.Ctx) error {
			return c.SendFile(config.ServeFrontEndDir + "/index.html")
		})
	}
}
