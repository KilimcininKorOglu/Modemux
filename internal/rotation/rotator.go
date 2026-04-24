package rotation

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/KilimcininKorOglu/modemux/internal/modem"
	"github.com/KilimcininKorOglu/modemux/internal/store"
)

type Result struct {
	ModemID    string        `json:"modemId"`
	OldIP      string        `json:"oldIp"`
	NewIP      string        `json:"newIp"`
	Duration   time.Duration `json:"duration"`
	RotationID int64         `json:"rotationId"`
}

type Rotator struct {
	ctrl     modem.Controller
	store    *store.Store
	cooldown *Cooldown
	timeout  time.Duration
}

func NewRotator(ctrl modem.Controller, store *store.Store, cooldown *Cooldown, timeout time.Duration) *Rotator {
	return &Rotator{
		ctrl:     ctrl,
		store:    store,
		cooldown: cooldown,
		timeout:  timeout,
	}
}

func (r *Rotator) Rotate(ctx context.Context, modemIndex int) (*Result, error) {
	modemID := strconv.Itoa(modemIndex)

	if err := r.cooldown.Check(modemID); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	status, err := r.ctrl.Status(ctx, modemIndex)
	if err != nil {
		return nil, fmt.Errorf("getting current status: %w", err)
	}

	oldIP := status.IP
	start := time.Now()

	slog.Info("starting IP rotation",
		"modem", modemIndex,
		"oldIP", oldIP,
	)

	if _, err := r.ctrl.SendAT(ctx, status.SerialPort, "AT+CGATT=0"); err != nil {
		r.store.InsertEvent(ctx, modemID, "rotation_failed", fmt.Sprintf("detach failed: %v", err))
		return nil, fmt.Errorf("detach failed: %w", err)
	}

	time.Sleep(2 * time.Second)

	if _, err := r.ctrl.SendAT(ctx, status.SerialPort, "AT+CGATT=1"); err != nil {
		r.store.InsertEvent(ctx, modemID, "rotation_failed", fmt.Sprintf("attach failed: %v", err))
		return nil, fmt.Errorf("attach failed: %w", err)
	}

	newIP, err := r.waitForNewIP(ctx, modemIndex, oldIP)
	if err != nil {
		r.store.InsertEvent(ctx, modemID, "rotation_failed", fmt.Sprintf("wait for IP failed: %v", err))
		return nil, err
	}

	duration := time.Since(start)
	r.cooldown.Mark(modemID)

	rotationID, _ := r.store.InsertRotation(ctx, modemID, oldIP, newIP, duration.Milliseconds())
	r.store.InsertEvent(ctx, modemID, "rotation", fmt.Sprintf("%s -> %s", oldIP, newIP))

	slog.Info("IP rotation complete",
		"modem", modemIndex,
		"oldIP", oldIP,
		"newIP", newIP,
		"duration", duration.Round(time.Millisecond),
	)

	return &Result{
		ModemID:    modemID,
		OldIP:      oldIP,
		NewIP:      newIP,
		Duration:   duration,
		RotationID: rotationID,
	}, nil
}

func (r *Rotator) waitForNewIP(ctx context.Context, modemIndex int, oldIP string) (string, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for new IP")
		case <-ticker.C:
			status, err := r.ctrl.Status(ctx, modemIndex)
			if err != nil {
				continue
			}
			if status.IP != "" && status.IP != oldIP {
				return status.IP, nil
			}
		}
	}
}

func (r *Rotator) CooldownRemaining(modemIndex int) time.Duration {
	return r.cooldown.Remaining(strconv.Itoa(modemIndex))
}
