package web

import (
	"crypto/subtle"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v3"

	"github.com/KilimcininKorOglu/modemux/internal/modem"
	"github.com/KilimcininKorOglu/modemux/internal/proxy"
	"github.com/KilimcininKorOglu/modemux/internal/rotation"
	"github.com/KilimcininKorOglu/modemux/internal/store"
	"github.com/KilimcininKorOglu/modemux/internal/web/templates"
)

type Handler struct {
	ctrl     modem.Controller
	store    *store.Store
	rotator  *rotation.Rotator
	proxyMgr *proxy.Manager
	sessions *SessionStore
	users    map[string]string
	limiter  *LoginLimiter
}

func NewHandler(ctrl modem.Controller, st *store.Store, rotator *rotation.Rotator, proxyMgr *proxy.Manager, sessions *SessionStore, users map[string]string, limiter *LoginLimiter) *Handler {
	return &Handler{
		ctrl:     ctrl,
		store:    st,
		rotator:  rotator,
		proxyMgr: proxyMgr,
		sessions: sessions,
		users:    users,
		limiter:  limiter,
	}
}

func (h *Handler) LoginPage(c fiber.Ctx) error {
	return render(c, templates.LoginPage(""))
}

func (h *Handler) LoginPost(c fiber.Ctx) error {
	if !h.limiter.Allow(c.IP()) {
		return render(c, templates.LoginPage("Too many login attempts. Try again later."))
	}

	username := c.FormValue("username")
	password := c.FormValue("password")

	expected, exists := h.users[username]
	if !exists || subtle.ConstantTimeCompare([]byte(expected), []byte(password)) != 1 {
		h.limiter.Record(c.IP())
		return render(c, templates.LoginPage("Invalid username or password"))
	}

	session := h.sessions.Create(username)

	c.Cookie(&fiber.Cookie{
		Name:     "modemux_session",
		Value:    session.Token,
		HTTPOnly: true,
		SameSite: "Lax",
		Path:     "/",
		MaxAge:   int(h.sessions.ttl.Seconds()),
	})

	return c.Redirect().To("/")
}

func (h *Handler) Logout(c fiber.Ctx) error {
	token := c.Cookies("modemux_session")
	if token != "" {
		h.sessions.Delete(token)
	}

	c.Cookie(&fiber.Cookie{
		Name:     "modemux_session",
		Value:    "",
		HTTPOnly: true,
		Path:     "/",
		MaxAge:   -1,
	})

	return c.Redirect().To("/login")
}

func (h *Handler) Dashboard(c fiber.Ctx) error {
	modemCards := h.getModemCards(c)
	historyRows := h.getHistory(c, 20, 0)
	status := h.getStatusBar(c)

	component := templates.Dashboard(modemCards, historyRows, len(historyRows) >= 20, status)
	return render(c, component)
}

func (h *Handler) ModemCards(c fiber.Ctx) error {
	modemCards := h.getModemCards(c)

	c.Set("Content-Type", "text/html")
	for _, m := range modemCards {
		component := templates.ModemCard(m)
		render(c, component)
	}
	return nil
}

func (h *Handler) RotateModem(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(400).SendString("invalid modem id")
	}

	_, err = h.rotator.Rotate(c.Context(), id)
	if err != nil {
		card := h.getModemCard(c, id)
		return render(c, templates.ModemCard(card))
	}

	time.Sleep(100 * time.Millisecond)

	card := h.getModemCard(c, id)
	return render(c, templates.ModemCard(card))
}

func (h *Handler) HistoryRows(c fiber.Ctx) error {
	offsetStr := c.Query("offset", "0")
	offset, _ := strconv.Atoi(offsetStr)

	rows := h.getHistory(c, 20, offset)
	c.Set("Content-Type", "text/html")
	for _, r := range rows {
		render(c, templates.HistoryRowFragment(r))
	}
	return nil
}

func (h *Handler) getModemCards(c fiber.Ctx) []templates.ModemCardData {
	modems, err := h.ctrl.Detect(c.Context())
	if err != nil {
		return nil
	}

	var cards []templates.ModemCardData
	for _, m := range modems {
		cards = append(cards, h.getModemCard(c, m.Index))
	}
	return cards
}

func (h *Handler) getModemCard(c fiber.Ctx, index int) templates.ModemCardData {
	status, err := h.ctrl.Status(c.Context(), index)
	if err != nil {
		return templates.ModemCardData{Index: index, State: "error"}
	}

	httpPort, socks5Port := h.proxyMgr.GetPorts(index)

	cooldown := h.rotator.CooldownRemaining(index)
	cooldownActive := cooldown > 0
	cooldownStr := ""
	if cooldownActive {
		cooldownStr = fmt.Sprintf("%ds", int(cooldown.Seconds()))
	}

	uptime := ""
	if !status.ConnectedAt.IsZero() {
		d := time.Since(status.ConnectedAt)
		switch {
		case d < time.Minute:
			uptime = fmt.Sprintf("%ds", int(d.Seconds()))
		case d < time.Hour:
			uptime = fmt.Sprintf("%dm", int(d.Minutes()))
		default:
			uptime = fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
		}
	}

	return templates.ModemCardData{
		Index:             index,
		Model:             status.Model,
		Manufacturer:      status.Manufacturer,
		State:             status.State.String(),
		Operator:          status.Operator,
		Signal:            status.SignalQuality,
		IP:                status.IP,
		HTTPPort:          httpPort,
		SOCKS5Port:        socks5Port,
		IMEI:              status.IMEI,
		Uptime:            uptime,
		RotationCount:     status.RotationCount,
		CooldownActive:    cooldownActive,
		CooldownRemaining: cooldownStr,
	}
}

func (h *Handler) getHistory(c fiber.Ctx, limit, offset int) []templates.HistoryRow {
	rotations, err := h.store.GetRotations(c.Context(), "", limit, offset)
	if err != nil {
		return nil
	}

	var rows []templates.HistoryRow
	for _, r := range rotations {
		rows = append(rows, templates.HistoryRow{
			Time:       r.RotatedAt.Format("15:04:05"),
			ModemID:    fmt.Sprintf("Modem %s", r.ModemID),
			OldIP:      r.OldIP,
			NewIP:      r.NewIP,
			DurationMs: r.DurationMs,
		})
	}
	return rows
}

func (h *Handler) Events(c fiber.Ctx) error {
	modemFilter := c.Query("modem", "")
	events := h.getEvents(c, modemFilter, 50, 0)
	component := templates.EventsPage(events, modemFilter, len(events) >= 50, len(events))
	return render(c, component)
}

func (h *Handler) EventRows(c fiber.Ctx) error {
	modemFilter := c.Query("modem", "")
	offsetStr := c.Query("offset", "0")
	offset, _ := strconv.Atoi(offsetStr)

	events := h.getEvents(c, modemFilter, 50, offset)
	c.Set("Content-Type", "text/html")
	for _, e := range events {
		render(c, templates.EventRowFragment(e))
	}
	return nil
}

func (h *Handler) StatusBarFragment(c fiber.Ctx) error {
	status := h.getStatusBar(c)
	return render(c, templates.StatusBar(status))
}

func (h *Handler) getEvents(c fiber.Ctx, modemID string, limit, offset int) []templates.EventRow {
	events, err := h.store.GetEvents(c.Context(), modemID, limit)
	if err != nil {
		return nil
	}

	var rows []templates.EventRow
	for _, e := range events {
		modemLabel := fmt.Sprintf("Modem %s", e.ModemID)
		rows = append(rows, templates.EventRow{
			Time:    e.Timestamp.Format("15:04:05"),
			ModemID: modemLabel,
			Event:   e.Event,
			Detail:  e.Detail,
		})
	}
	return rows
}

func (h *Handler) getStatusBar(c fiber.Ctx) templates.StatusBarData {
	modems, _ := h.ctrl.Detect(c.Context())
	connectedCount := 0
	for _, m := range modems {
		st, err := h.ctrl.Status(c.Context(), m.Index)
		if err == nil && st.IP != "" {
			connectedCount++
		}
	}

	totalRotations, _ := h.store.GetRotationCount(c.Context(), "")
	uniqueIPs, _ := h.store.GetUniqueIPCount(c.Context(), "")

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Since(serverStartTime)
	uptimeStr := fmt.Sprintf("%dh %dm", int(uptime.Hours()), int(uptime.Minutes())%60)
	if uptime < time.Hour {
		uptimeStr = fmt.Sprintf("%dm", int(uptime.Minutes()))
	}

	return templates.StatusBarData{
		Uptime:          uptimeStr,
		TotalModems:     len(modems),
		ConnectedModems: connectedCount,
		TotalRotations:  totalRotations,
		UniqueIPs:       uniqueIPs,
		MemoryMB:        fmt.Sprintf("%.1f MB", float64(memStats.Alloc)/1024/1024),
	}
}

var serverStartTime = time.Now()

func render(c fiber.Ctx, component templ.Component) error {
	c.Set("Content-Type", "text/html")
	return component.Render(c.Context(), c.Response().BodyWriter())
}
