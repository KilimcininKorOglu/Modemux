package api

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/KilimcininKorOglu/modemux/internal/config"
	"github.com/KilimcininKorOglu/modemux/internal/modem"
	"github.com/KilimcininKorOglu/modemux/internal/proxy"
	"github.com/KilimcininKorOglu/modemux/internal/rotation"
	"github.com/KilimcininKorOglu/modemux/internal/store"
)

type Server struct {
	app      *fiber.App
	cfg      *config.Config
	ctrl     modem.Controller
	store    *store.Store
	rotator  *rotation.Rotator
	proxyMgr *proxy.Manager
	sseHub   *SSEHub
	version  string
}

func NewServer(cfg *config.Config, ctrl modem.Controller, store *store.Store, rotator *rotation.Rotator, proxyMgr *proxy.Manager, version string) *Server {
	app := fiber.New(fiber.Config{
		AppName:      "Modemux",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	s := &Server{
		app:      app,
		cfg:      cfg,
		ctrl:     ctrl,
		store:    store,
		rotator:  rotator,
		proxyMgr: proxyMgr,
		sseHub:   NewSSEHub(),
		version:  version,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.app.Use(recover.New())
	s.app.Use(logger.New(logger.Config{
		Format:     "${time} ${status} ${method} ${path} ${latency}\n",
		TimeFormat: "15:04:05",
	}))
	s.app.Use(cors.New())

	s.app.Get("/healthz", s.handleHealthz)
	s.app.Get("/readyz", s.handleReadyz)

	api := s.app.Group("/api", basicAuthMiddleware(s.cfg.Auth.Users))

	api.Get("/status", s.handleStatus)
	api.Get("/modems", s.handleListModems)
	api.Get("/modems/:id", s.handleGetModem)
	api.Post("/modems/:id/rotate", s.handleRotate)
	api.Get("/modems/:id/history", s.handleHistory)
	api.Get("/ip-history", s.handleAllHistory)
	api.Get("/speedtest/:id", s.handleSpeedtest)
	api.Get("/events", s.handleSSE)
}

func (s *Server) Listen() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.APIPort)
	return s.app.Listen(addr)
}

func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

func (s *Server) SSEHub() *SSEHub {
	return s.sseHub
}

func (s *Server) App() *fiber.App {
	return s.app
}
