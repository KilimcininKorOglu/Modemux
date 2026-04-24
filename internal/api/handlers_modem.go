package api

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

func (s *Server) handleListModems(c fiber.Ctx) error {
	modems, err := s.ctrl.Detect(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(errorResponse(err.Error()))
	}

	type modemResponse struct {
		Index        int    `json:"index"`
		Model        string `json:"model"`
		Manufacturer string `json:"manufacturer"`
		State        string `json:"state"`
		Operator     string `json:"operator"`
		Signal       int    `json:"signalQuality"`
		IP           string `json:"ip"`
		HTTPPort     int    `json:"httpPort"`
		SOCKS5Port   int    `json:"socks5Port"`
	}

	var result []modemResponse
	for _, m := range modems {
		status, err := s.ctrl.Status(c.Context(), m.Index)
		if err != nil {
			continue
		}

		httpPort, socks5Port := s.proxyMgr.GetPorts(m.Index)

		result = append(result, modemResponse{
			Index:        m.Index,
			Model:        m.Model,
			Manufacturer: m.Manufacturer,
			State:        status.State.String(),
			Operator:     status.Operator,
			Signal:       status.SignalQuality,
			IP:           status.IP,
			HTTPPort:     httpPort,
			SOCKS5Port:   socks5Port,
		})
	}

	return c.JSON(successResponse(result))
}

func (s *Server) handleGetModem(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse("invalid modem id"))
	}

	detail, err := s.ctrl.Detail(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(errorResponse(err.Error()))
	}

	httpPort, socks5Port := s.proxyMgr.GetPorts(id)
	detail.HTTPPort = httpPort
	detail.SOCKS5Port = socks5Port

	cooldown := s.rotator.CooldownRemaining(id)

	data := fiber.Map{
		"modem":             detail,
		"cooldownRemaining": cooldown.String(),
	}

	return c.JSON(successResponse(data))
}
