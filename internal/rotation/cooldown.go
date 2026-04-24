package rotation

import (
	"fmt"
	"sync"
	"time"
)

type Cooldown struct {
	mu       sync.RWMutex
	interval time.Duration
	lastUsed map[string]time.Time
}

func NewCooldown(interval time.Duration) *Cooldown {
	return &Cooldown{
		interval: interval,
		lastUsed: make(map[string]time.Time),
	}
}

func (c *Cooldown) Check(modemID string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	last, ok := c.lastUsed[modemID]
	if !ok {
		return nil
	}

	remaining := c.interval - time.Since(last)
	if remaining > 0 {
		return fmt.Errorf("cooldown active for modem %s: %s remaining", modemID, remaining.Round(time.Second))
	}

	return nil
}

func (c *Cooldown) Mark(modemID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUsed[modemID] = time.Now()
}

func (c *Cooldown) Remaining(modemID string) time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	last, ok := c.lastUsed[modemID]
	if !ok {
		return 0
	}

	remaining := c.interval - time.Since(last)
	if remaining < 0 {
		return 0
	}
	return remaining
}
