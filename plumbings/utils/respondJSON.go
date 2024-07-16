package utils

import "github.com/gofiber/fiber/v2"

func FiberJSONWrap(c *fiber.Ctx, data any, count ...int) (err error) {
	if len(count) == 1 {
		return c.JSON(fiber.Map{"error": false, "data": data, "count": count[0]})
	}
	return c.JSON(fiber.Map{"error": false, "data": data})
}
