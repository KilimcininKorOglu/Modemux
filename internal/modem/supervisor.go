package modem

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type SupervisorConfig struct {
	Controller    Controller
	DefaultAPN    string
	ScanInterval  time.Duration
	PollInterval  time.Duration
	OnStateChange StateChangeFunc
	Overrides     map[string]string // IMEI -> APN override
}

type Supervisor struct {
	mu            sync.RWMutex
	cfg           SupervisorConfig
	workers       map[int]*Worker
	cancel        context.CancelFunc
	done          chan struct{}
}

func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.ScanInterval == 0 {
		cfg.ScanInterval = 30 * time.Second
	}
	if cfg.Overrides == nil {
		cfg.Overrides = make(map[string]string)
	}

	return &Supervisor{
		cfg:     cfg,
		workers: make(map[int]*Worker),
		done:    make(chan struct{}),
	}
}

func (s *Supervisor) Start(ctx context.Context) {
	supervisorCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go s.run(supervisorCtx)
}

func (s *Supervisor) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	<-s.done
}

func (s *Supervisor) AllStates() map[int]*ModemStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[int]*ModemStatus, len(s.workers))
	for idx, w := range s.workers {
		result[idx] = w.Status()
	}
	return result
}

func (s *Supervisor) WorkerStatus(modemIndex int) (*ModemStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w, ok := s.workers[modemIndex]
	if !ok {
		return nil, fmt.Errorf("no worker for modem %d", modemIndex)
	}
	return w.Status(), nil
}

func (s *Supervisor) WorkerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.workers)
}

func (s *Supervisor) ConnectedModems() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var connected []int
	for idx, w := range s.workers {
		if w.State() == StateConnected {
			connected = append(connected, idx)
		}
	}
	return connected
}

func (s *Supervisor) run(ctx context.Context) {
	defer close(s.done)

	slog.Info("supervisor started",
		"scanInterval", s.cfg.ScanInterval,
		"pollInterval", s.cfg.PollInterval,
	)

	s.scan(ctx)

	ticker := time.NewTicker(s.cfg.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.stopAllWorkers()
			slog.Info("supervisor stopped")
			return
		case <-ticker.C:
			s.scan(ctx)
		}
	}
}

func (s *Supervisor) scan(ctx context.Context) {
	modems, err := s.cfg.Controller.Detect(ctx)
	if err != nil {
		slog.Error("modem scan failed", "error", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	activeIndices := make(map[int]bool)
	for _, m := range modems {
		activeIndices[m.Index] = true

		if _, exists := s.workers[m.Index]; exists {
			continue
		}

		apn := s.cfg.DefaultAPN
		if override, ok := s.cfg.Overrides[m.IMEI]; ok {
			apn = override
		}

		slog.Info("new modem detected, starting worker",
			"modem", m.Index,
			"model", m.Model,
			"imei", m.IMEI,
		)

		worker := NewWorker(m, s.cfg.Controller, apn, s.cfg.PollInterval, s.cfg.OnStateChange)
		worker.Start(ctx)
		s.workers[m.Index] = worker
	}

	for idx, w := range s.workers {
		if !activeIndices[idx] {
			slog.Warn("modem removed, stopping worker", "modem", idx)
			go w.Stop()
			delete(s.workers, idx)

			if s.cfg.OnStateChange != nil {
				s.cfg.OnStateChange(idx, StateConnected, StateDisconnected, nil)
			}
		}
	}
}

func (s *Supervisor) stopAllWorkers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var wg sync.WaitGroup
	for idx, w := range s.workers {
		wg.Add(1)
		go func(index int, worker *Worker) {
			defer wg.Done()
			worker.Stop()
			slog.Info("worker stopped", "modem", index)
		}(idx, w)
	}
	wg.Wait()

	s.workers = make(map[int]*Worker)
}
