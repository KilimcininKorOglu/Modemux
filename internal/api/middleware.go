package api

import (
	"crypto/subtle"
	"encoding/base64"
	"time"

	"github.com/gofiber/fiber/v3"
)

type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

func successResponse(data interface{}) *APIResponse {
	return &APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
	}
}

func errorResponse(msg string) *APIResponse {
	return &APIResponse{
		Success:   false,
		Error:     msg,
		Timestamp: time.Now(),
	}
}

func basicAuthMiddleware(users map[string]string) fiber.Handler {
	return func(c fiber.Ctx) error {
		user, pass, ok := parseBasicAuth(c.Get("Authorization"))
		if !ok {
			c.Set("WWW-Authenticate", `Basic realm="Modemux"`)
			return c.Status(fiber.StatusUnauthorized).JSON(errorResponse("authentication required"))
		}

		expected, exists := users[user]
		if !exists || subtle.ConstantTimeCompare([]byte(expected), []byte(pass)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(errorResponse("invalid credentials"))
		}

		c.Locals("username", user)
		return c.Next()
	}
}

func parseBasicAuth(auth string) (string, string, bool) {
	if len(auth) < 7 || auth[:6] != "Basic " {
		return "", "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return "", "", false
	}

	s := string(decoded)
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return s[:i], s[i+1:], true
		}
	}

	return "", "", false
}
