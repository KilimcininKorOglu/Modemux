package api

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

func queryInt(c fiber.Ctx, key string, defaultVal int) int {
	s := c.Query(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func (s *Server) handleRotate(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse("invalid modem id"))
	}

	result, err := s.rotator.Rotate(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(errorResponse(err.Error()))
	}

	data := fiber.Map{
		"modemId":    result.ModemID,
		"oldIp":      result.OldIP,
		"newIp":      result.NewIP,
		"durationMs": result.Duration.Milliseconds(),
		"rotationId": result.RotationID,
	}

	return c.JSON(successResponse(data))
}

func (s *Server) handleHistory(c fiber.Ctx) error {
	idStr := c.Params("id")
	modemID := ""
	if idStr != "" {
		if _, err := strconv.Atoi(idStr); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(errorResponse("invalid modem id"))
		}
		modemID = idStr
	}

	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	if limit > 200 {
		limit = 200
	}

	rotations, err := s.store.GetRotations(c.Context(), modemID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(errorResponse(err.Error()))
	}

	total, _ := s.store.GetRotationCount(c.Context(), modemID)

	data := fiber.Map{
		"rotations": rotations,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	}

	return c.JSON(successResponse(data))
}

func (s *Server) handleAllHistory(c fiber.Ctx) error {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	if limit > 200 {
		limit = 200
	}

	rotations, err := s.store.GetRotations(c.Context(), "", limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(errorResponse(err.Error()))
	}

	total, _ := s.store.GetRotationCount(c.Context(), "")
	uniqueIPs, _ := s.store.GetUniqueIPCount(c.Context(), "")

	data := fiber.Map{
		"rotations": rotations,
		"total":     total,
		"uniqueIPs": uniqueIPs,
		"limit":     limit,
		"offset":    offset,
	}

	return c.JSON(successResponse(data))
}
