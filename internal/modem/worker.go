package modem

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type StateChangeFunc func(modemIndex int, oldState, newState State, status *ModemStatus)

type Worker struct {
	mu            sync.RWMutex
	index         int
	info          ModemInfo
	ctrl          Controller
	apn           string
	state         State
	status        *ModemStatus
	onStateChange StateChangeFunc
	pollInterval  time.Duration
	cancel        context.CancelFunc
	done          chan struct{}
}

func NewWorker(info ModemInfo, ctrl Controller, apn string, pollInterval time.Duration, onStateChange StateChangeFunc) *Worker {
	return &Worker{
		index:         info.Index,
		info:          info,
		ctrl:          ctrl,
		apn:           apn,
		state:         StateDisconnected,
		pollInterval:  pollInterval,
		onStateChange: onStateChange,
		done:          make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) {
	workerCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	go w.run(workerCtx)
}

func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	<-w.done
}

func (w *Worker) State() State {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state
}

func (w *Worker) Status() *ModemStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.status == nil {
		return &ModemStatus{ModemInfo: w.info, State: w.state}
	}
	cp := *w.status
	return &cp
}

func (w *Worker) Index() int {
	return w.index
}

func (w *Worker) run(ctx context.Context) {
	defer close(w.done)

	slog.Info("worker started", "modem", w.index, "model", w.info.Model)

	w.initialConnect(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopping", "modem", w.index)
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) initialConnect(ctx context.Context) {
	status, err := w.ctrl.Status(ctx, w.index)
	if err != nil {
		slog.Warn("initial status check failed", "modem", w.index, "error", err)
		w.setState(StateFailed, nil)
		return
	}

	if status.State == StateConnected && status.IP != "" {
		w.setState(StateConnected, status)
		return
	}

	w.setState(StateConnecting, status)
	slog.Info("connecting modem", "modem", w.index, "apn", w.apn)

	if err := w.ctrl.Connect(ctx, w.index, w.apn); err != nil {
		slog.Error("initial connect failed", "modem", w.index, "error", err)
		w.setState(StateFailed, status)
		return
	}

	time.Sleep(3 * time.Second)

	status, err = w.ctrl.Status(ctx, w.index)
	if err != nil {
		w.setState(StateFailed, nil)
		return
	}

	if status.State == StateConnected && status.IP != "" {
		w.setState(StateConnected, status)
	} else {
		w.setState(StateFailed, status)
	}
}

func (w *Worker) poll(ctx context.Context) {
	status, err := w.ctrl.Status(ctx, w.index)
	if err != nil {
		slog.Debug("poll failed", "modem", w.index, "error", err)
		return
	}

	w.mu.Lock()
	oldState := w.state
	w.status = status
	w.mu.Unlock()

	if status.State == StateConnected && status.IP != "" {
		if oldState != StateConnected {
			w.setState(StateConnected, status)
		}
		return
	}

	if oldState == StateConnected {
		slog.Warn("modem disconnected, attempting reconnect", "modem", w.index)
		w.setState(StateDisconnected, status)
		w.reconnect(ctx)
	}
}

func (w *Worker) reconnect(ctx context.Context) {
	w.setState(StateConnecting, nil)

	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		slog.Info("reconnect attempt", "modem", w.index, "attempt", attempt)

		if err := w.ctrl.Connect(ctx, w.index, w.apn); err != nil {
			slog.Warn("reconnect failed", "modem", w.index, "attempt", attempt, "error", err)
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}

		time.Sleep(3 * time.Second)

		status, err := w.ctrl.Status(ctx, w.index)
		if err == nil && status.State == StateConnected && status.IP != "" {
			w.setState(StateConnected, status)
			slog.Info("reconnect successful", "modem", w.index, "ip", status.IP)
			return
		}

		time.Sleep(time.Duration(attempt*2) * time.Second)
	}

	slog.Error("all reconnect attempts failed", "modem", w.index)
	w.setState(StateFailed, nil)
}

func (w *Worker) setState(newState State, status *ModemStatus) {
	w.mu.Lock()
	oldState := w.state
	w.state = newState
	if status != nil {
		w.status = status
	}
	w.mu.Unlock()

	if oldState != newState {
		slog.Info("modem state changed",
			"modem", w.index,
			"from", oldState.String(),
			"to", newState.String(),
		)
		if w.onStateChange != nil {
			w.onStateChange(w.index, oldState, newState, status)
		}
	}
}
