package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/KilimcininKorOglu/modemux/internal/api"
	"github.com/KilimcininKorOglu/modemux/internal/config"
	"github.com/KilimcininKorOglu/modemux/internal/modem"
	"github.com/KilimcininKorOglu/modemux/internal/proxy"
	"github.com/KilimcininKorOglu/modemux/internal/rotation"
	"github.com/KilimcininKorOglu/modemux/internal/store"
	"github.com/KilimcininKorOglu/modemux/internal/web"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "detect":
		cmdDetect()
	case "status":
		cmdStatus()
	case "rotate":
		cmdRotate()
	case "serve":
		cmdServe()
	case "version":
		fmt.Printf("modemux %s\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `modemux %s - Self-hosted mobile proxy management

Usage:
  modemux <command> [flags]

Commands:
  detect     Detect connected USB LTE modems
  status     Show modem status
  rotate     Rotate IP for a modem
  serve      Start the proxy server and API
  version    Print version

Flags:
  --config   Path to config file (default: auto-detect)
  --mock     Use mock modem controller (for development)
`, version)
}

func newController(useMock bool, mockModems int) modem.Controller {
	if useMock {
		return modem.NewMockController(mockModems)
	}
	return modem.NewMMCLIController()
}

func cmdDetect() {
	fs := flag.NewFlagSet("detect", flag.ExitOnError)
	mock := fs.Bool("mock", false, "use mock modem controller")
	mockCount := fs.Int("mock-modems", 2, "number of mock modems")
	jsonOut := fs.Bool("json", false, "output as JSON")
	fs.Parse(os.Args[2:])

	ctrl := newController(*mock, *mockCount)
	ctx := context.Background()

	modems, err := ctrl.Detect(ctx)
	if err != nil {
		slog.Error("failed to detect modems", "error", err)
		os.Exit(1)
	}

	if len(modems) == 0 {
		fmt.Println("No modems found.")
		return
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(modems)
		return
	}

	fmt.Printf("Found %d modem(s):\n\n", len(modems))
	for _, m := range modems {
		fmt.Printf("  [%d] %s %s\n", m.Index, m.Manufacturer, m.Model)
		fmt.Printf("      IMEI: %s\n", m.IMEI)
		fmt.Printf("      Serial: %s  Data: %s\n\n", m.SerialPort, m.DataPort)
	}
}

func cmdStatus() {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	mock := fs.Bool("mock", false, "use mock modem controller")
	mockCount := fs.Int("mock-modems", 2, "number of mock modems")
	jsonOut := fs.Bool("json", false, "output as JSON")
	fs.Parse(os.Args[2:])

	ctrl := newController(*mock, *mockCount)
	ctx := context.Background()

	modemIdx := 0
	if fs.NArg() > 0 {
		var err error
		modemIdx, err = strconv.Atoi(fs.Arg(0))
		if err != nil {
			slog.Error("invalid modem index", "value", fs.Arg(0))
			os.Exit(1)
		}
	}

	status, err := ctrl.Status(ctx, modemIdx)
	if err != nil {
		slog.Error("failed to get modem status", "error", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(status)
		return
	}

	fmt.Printf("Modem %d: %s %s\n", status.Index, status.Manufacturer, status.Model)
	fmt.Printf("  State:    %s\n", status.State)
	fmt.Printf("  Operator: %s\n", status.Operator)
	fmt.Printf("  Signal:   %d%%\n", status.SignalQuality)
	fmt.Printf("  Tech:     %s\n", status.AccessTech)
	fmt.Printf("  IP:       %s\n", status.IP)
	fmt.Printf("  Iface:    %s\n", status.Interface)
}

func cmdRotate() {
	fs := flag.NewFlagSet("rotate", flag.ExitOnError)
	mock := fs.Bool("mock", false, "use mock modem controller")
	mockCount := fs.Int("mock-modems", 2, "number of mock modems")
	fs.Parse(os.Args[2:])

	modemIdx := 0
	if fs.NArg() > 0 {
		var err error
		modemIdx, err = strconv.Atoi(fs.Arg(0))
		if err != nil {
			slog.Error("invalid modem index", "value", fs.Arg(0))
			os.Exit(1)
		}
	}

	ctrl := newController(*mock, *mockCount)
	ctx := context.Background()

	fmt.Printf("Rotating IP for modem %d...\n", modemIdx)

	oldStatus, err := ctrl.Status(ctx, modemIdx)
	if err != nil {
		slog.Error("failed to get current status", "error", err)
		os.Exit(1)
	}
	oldIP := oldStatus.IP

	start := time.Now()

	if _, err := ctrl.SendAT(ctx, oldStatus.SerialPort, "AT+CGATT=0"); err != nil {
		slog.Error("detach failed", "error", err)
		os.Exit(1)
	}

	time.Sleep(2 * time.Second)

	if _, err := ctrl.SendAT(ctx, oldStatus.SerialPort, "AT+CGATT=1"); err != nil {
		slog.Error("attach failed", "error", err)
		os.Exit(1)
	}

	time.Sleep(3 * time.Second)

	newStatus, err := ctrl.Status(ctx, modemIdx)
	if err != nil {
		slog.Error("failed to get new status", "error", err)
		os.Exit(1)
	}

	duration := time.Since(start)
	fmt.Printf("Rotation complete in %s\n", duration.Round(time.Millisecond))
	fmt.Printf("  Old IP: %s\n", oldIP)
	fmt.Printf("  New IP: %s\n", newStatus.IP)
}

func cmdServe() {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "config file path")
	mock := fs.Bool("mock", false, "use mock modem controller")
	mockCount := fs.Int("mock-modems", 2, "number of mock modems")
	fs.Parse(os.Args[2:])

	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.Load(*configPath)
	} else {
		cfg, err = config.FindAndLoad()
	}
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logLevel := slog.LevelInfo
	switch cfg.Server.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	ctrl := newController(*mock, *mockCount)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("starting modemux",
		"version", version,
		"port", cfg.Server.APIPort,
		"mock", *mock,
	)

	db, err := store.New(cfg.Storage.DatabasePath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	cooldown := rotation.NewCooldown(cfg.Rotation.Cooldown.Duration)
	rotator := rotation.NewRotator(ctrl, db, cooldown, cfg.Rotation.Timeout.Duration)

	proxyMgr := proxy.NewManager(
		cfg.Proxy.HTTPPortStart,
		cfg.Proxy.SOCKS5PortStart,
		cfg.Proxy.AuthRequired,
		cfg.Proxy.Username,
		cfg.Proxy.Password,
	)

	overrides := make(map[string]string)
	for _, o := range cfg.Modems.Overrides {
		overrides[o.IMEI] = o.APN
	}

	onStateChange := func(modemIndex int, oldState, newState modem.State, status *modem.ModemStatus) {
		db.InsertEvent(ctx, fmt.Sprintf("%d", modemIndex), "state_change",
			fmt.Sprintf("%s -> %s", oldState, newState))

		switch newState {
		case modem.StateConnected:
			if status != nil && status.IP != "" {
				if err := proxyMgr.StartProxy(modemIndex, status.IP); err != nil {
					slog.Error("failed to start proxy", "modem", modemIndex, "error", err)
				}
			}
		case modem.StateDisconnected, modem.StateFailed:
			proxyMgr.StopProxy(modemIndex)
		}
	}

	supervisor := modem.NewSupervisor(modem.SupervisorConfig{
		Controller:    ctrl,
		DefaultAPN:    cfg.Modems.DefaultAPN,
		ScanInterval:  cfg.Modems.ScanInterval.Duration,
		PollInterval:  5 * time.Second,
		OnStateChange: onStateChange,
		Overrides:     overrides,
	})
	supervisor.Start(ctx)

	time.Sleep(500 * time.Millisecond)

	server := api.NewServer(cfg, ctrl, db, rotator, proxyMgr, version)
	web.BuildVersion = version
	web.RegisterRoutes(server.App(), ctrl, db, rotator, proxyMgr, cfg.Auth.Users)

	go func() {
		if err := server.Listen(); err != nil {
			slog.Error("API server error", "error", err)
		}
	}()

	fmt.Printf("\nmodemux %s running on %s:%d\n", version, cfg.Server.Host, cfg.Server.APIPort)
	fmt.Println("Press Ctrl+C to stop.")

	<-ctx.Done()
	slog.Info("shutting down")

	server.Shutdown()
	supervisor.Stop()
	proxyMgr.StopAll()
	slog.Info("shutdown complete")
}
