package web

import (
	"embed"
	"io/fs"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/KilimcininKorOglu/modemux/internal/modem"
	"github.com/KilimcininKorOglu/modemux/internal/web/templates"
	"github.com/KilimcininKorOglu/modemux/internal/proxy"
	"github.com/KilimcininKorOglu/modemux/internal/rotation"
	"github.com/KilimcininKorOglu/modemux/internal/store"
)

//go:embed static
var staticFiles embed.FS

var BuildVersion = "dev"

func RegisterRoutes(app *fiber.App, ctrl modem.Controller, st *store.Store, rotator *rotation.Rotator, proxyMgr *proxy.Manager, users map[string]string) {
	templates.AssetVersion = BuildVersion
	sessions := NewSessionStore(24 * time.Hour)
	limiter := NewLoginLimiter(5, time.Minute)
	h := NewHandler(ctrl, st, rotator, proxyMgr, sessions, users, limiter)

	staticFS, _ := fs.Sub(staticFiles, "static")
	app.Use("/static", noCacheHeaders, static.New("", static.Config{
		FS: staticFS,
	}))

	app.Get("/login", h.LoginPage)
	app.Post("/login", h.LoginPost)
	app.Post("/logout", h.Logout)

	app.Use(sessionMiddleware(sessions))

	app.Get("/", h.Dashboard)
	app.Get("/events", h.Events)
	app.Get("/web/modems", h.ModemCards)
	app.Post("/web/modems/:id/rotate", h.RotateModem)
	app.Get("/web/history", h.HistoryRows)
	app.Get("/web/events", h.EventRows)
	app.Get("/web/status", h.StatusBarFragment)
}

func noCacheHeaders(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
	return c.Next()
}

func sessionMiddleware(sessions *SessionStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		if path == "/login" || path == "/healthz" || path == "/readyz" {
			return c.Next()
		}
		if len(path) >= 7 && path[:7] == "/static" {
			return c.Next()
		}
		if len(path) >= 4 && path[:4] == "/api" {
			return c.Next()
		}

		token := c.Cookies("modemux_session")
		if token == "" {
			return c.Redirect().To("/login")
		}

		session, valid := sessions.Validate(token)
		if !valid {
			return c.Redirect().To("/login")
		}

		c.Locals("username", session.Username)
		return c.Next()
	}
}
