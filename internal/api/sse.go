package api

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

type SSEEvent struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

type SSEHub struct {
	mu      sync.RWMutex
	clients map[chan SSEEvent]struct{}
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[chan SSEEvent]struct{}),
	}
}

func (h *SSEHub) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *SSEHub) Unsubscribe(ch chan SSEEvent) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *SSEHub) Broadcast(event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Server) handleSSE(c fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	ch := s.sseHub.Subscribe()
	defer s.sseHub.Unsubscribe(ch)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	notify := c.Context().Done()

	for {
		select {
		case <-notify:
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event.Event, event.Data)
			if _, err := c.Write([]byte(msg)); err != nil {
				return nil
			}
		case <-ticker.C:
			if _, err := c.Write([]byte(": keepalive\n\n")); err != nil {
				return nil
			}
		}
	}
}
