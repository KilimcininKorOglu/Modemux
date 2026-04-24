package web

import (
	"sync"
	"time"
)

type attemptInfo struct {
	count   int
	firstAt time.Time
}

type LoginLimiter struct {
	mu          sync.RWMutex
	attempts    map[string]*attemptInfo
	maxAttempts int
	window      time.Duration
}

func NewLoginLimiter(maxAttempts int, window time.Duration) *LoginLimiter {
	l := &LoginLimiter{
		attempts:    make(map[string]*attemptInfo),
		maxAttempts: maxAttempts,
		window:      window,
	}
	go l.cleanup()
	return l
}

func (l *LoginLimiter) Allow(ip string) bool {
	l.mu.RLock()
	info, exists := l.attempts[ip]
	l.mu.RUnlock()

	if !exists {
		return true
	}

	if time.Since(info.firstAt) > l.window {
		l.mu.Lock()
		delete(l.attempts, ip)
		l.mu.Unlock()
		return true
	}

	return info.count < l.maxAttempts
}

func (l *LoginLimiter) Record(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	info, exists := l.attempts[ip]
	if !exists || time.Since(info.firstAt) > l.window {
		l.attempts[ip] = &attemptInfo{count: 1, firstAt: time.Now()}
		return
	}

	info.count++
}

func (l *LoginLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for ip, info := range l.attempts {
			if now.Sub(info.firstAt) > l.window*2 {
				delete(l.attempts, ip)
			}
		}
		l.mu.Unlock()
	}
}
