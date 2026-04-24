package api

import (
	"github.com/gofiber/fiber/v3"
)

func (s *Server) handleSpeedtest(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(errorResponse("speedtest not yet implemented"))
}
