package api

import (
	"runtime"
	"time"

	"github.com/gofiber/fiber/v3"
)

var startTime = time.Now()

func (s *Server) handleHealthz(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func (s *Server) handleReadyz(c fiber.Ctx) error {
	modems, err := s.ctrl.Detect(c.Context())
	if err != nil || len(modems) == 0 {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not_ready",
			"reason": "no modems connected",
		})
	}

	return c.JSON(fiber.Map{"status": "ready", "modems": len(modems)})
}

func (s *Server) handleStatus(c fiber.Ctx) error {
	modems, _ := s.ctrl.Detect(c.Context())
	proxies := s.proxyMgr.GetAllProxies()
	rotationCount, _ := s.store.GetRotationCount(c.Context(), "")

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	data := fiber.Map{
		"version":       s.version,
		"uptime":        time.Since(startTime).String(),
		"uptimeSeconds": int(time.Since(startTime).Seconds()),
		"modems":        len(modems),
		"activeProxies": len(proxies),
		"totalRotations": rotationCount,
		"memory": fiber.Map{
			"allocMB": float64(memStats.Alloc) / 1024 / 1024,
			"sysMB":   float64(memStats.Sys) / 1024 / 1024,
		},
		"goVersion": runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
	}

	return c.JSON(successResponse(data))
}
