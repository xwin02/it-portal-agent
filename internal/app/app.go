package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/itportal/it-portal-agent/internal/bootstrap"
	"github.com/itportal/it-portal-agent/internal/config"
	"github.com/itportal/it-portal-agent/internal/heartbeat"
	"github.com/itportal/it-portal-agent/internal/inventory"
	"github.com/itportal/it-portal-agent/internal/logger"
	"github.com/itportal/it-portal-agent/internal/software"
	"github.com/itportal/it-portal-agent/internal/storage"
	syncengine "github.com/itportal/it-portal-agent/internal/sync"
	"github.com/itportal/it-portal-agent/internal/windowsservice"
	"go.uber.org/zap"
)

const Version = "0.2.0"

func Run(arguments []string) error {
	if len(arguments) == 0 {
		return runService()
	}
	switch strings.ToLower(arguments[0]) {
	case "install":
		return windowsservice.Install()
	case "uninstall":
		return windowsservice.Uninstall()
	case "start":
		return windowsservice.Start()
	case "stop":
		return windowsservice.Stop()
	case "restart":
		return windowsservice.Restart()
	case "version":
		fmt.Println(Version)
		return nil
	case "config":
		fmt.Println(config.Path())
		return nil
	case "register":
		return withBootstrap(func(ctx context.Context, service *bootstrap.Service) error { return service.EnsureRegistered(ctx) })
	case "reload-config":
		return withBootstrap(func(ctx context.Context, service *bootstrap.Service) error { return service.ReloadConfig(ctx) })
	case "status":
		return withBootstrap(func(_ context.Context, service *bootstrap.Service) error { return printStatus(service) })
	case "inventory":
		return withBootstrap(func(ctx context.Context, service *bootstrap.Service) error {
			transport := syncengine.New(service.DB, syncengine.Options{
				PortalURL: service.Config.PortalURL, Proxy: service.Config.Proxy, TLSValidate: service.Config.TLSValidation,
				Timeout: 30 * time.Second, RetryDelay: time.Second,
			}, service.Log)
			report, err := inventory.New(service.DB, transport, service.Log, Version).RunOnce(ctx)
			fmt.Printf("Collect Duration: %s\nUpload Duration: %s\nPayload Size: %d bytes\nHardware Count: %d\nChanged Items: %d\nSkipped Items: %d\nFingerprint: %s\nResult: %s\n", report.CollectDuration.Round(time.Millisecond), report.UploadDuration.Round(time.Millisecond), report.PayloadSize, report.HardwareCount, report.ChangedItems, report.SkippedItems, report.Fingerprint, report.Result)
			if errors.Is(err, syncengine.ErrQueued) {
				return nil
			}
			return err
		})
	case "software":
		return withBootstrap(func(ctx context.Context, service *bootstrap.Service) error {
			transport := syncengine.New(service.DB, syncengine.Options{
				PortalURL: service.Config.PortalURL, Proxy: service.Config.Proxy, TLSValidate: service.Config.TLSValidation,
				Timeout: 60 * time.Second, RetryDelay: time.Second,
			}, service.Log)
			report, err := software.New(service.DB, transport, service.Log, Version).RunOnce(ctx)
			fmt.Printf("Software Collection Duration: %s\nSoftware Count: %d\nChanged Items: %d\nSkipped Items: %d\nPayload Size: %d bytes\nFingerprint: %s\nResult: %s\n", report.CollectDuration.Round(time.Millisecond), report.SoftwareCount, report.ChangedItems, report.SkippedItems, report.PayloadSize, report.Fingerprint, report.Result)
			if errors.Is(err, syncengine.ErrQueued) {
				return nil
			}
			return err
		})
	default:
		return fmt.Errorf("unknown command %q; valid commands: install, uninstall, start, stop, restart, register, status, inventory, software, reload-config, version, config", arguments[0])
	}
}

func runService() error {
	cfg, log, db, err := initialize()
	if err != nil {
		return err
	}
	defer log.Sync()
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service := bootstrap.New(db, cfg, log, Version)
	start := func() {
		if err := bootstrapWithRetry(ctx, service, log); err != nil {
			return
		}
		transport := syncengine.New(db, syncengine.Options{
			PortalURL: cfg.PortalURL, Proxy: cfg.Proxy, TLSValidate: cfg.TLSValidation,
			Timeout: 30 * time.Second, RetryDelay: time.Second,
		}, log)
		go inventory.New(db, transport, log, Version).Run(ctx)
		go software.New(db, transport, log, Version).Run(ctx)
		heartbeat.New(db, transport, log, Version).Run(ctx)
	}
	return windowsservice.Run(cancel, start)
}

func withBootstrap(action func(context.Context, *bootstrap.Service) error) error {
	cfg, log, db, err := initialize()
	if err != nil {
		return err
	}
	defer log.Sync()
	defer db.Close()
	return action(context.Background(), bootstrap.New(db, cfg, log, Version))
}

func initialize() (config.Config, *zap.Logger, *sql.DB, error) {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	db, err := storage.Open()
	if err != nil {
		log.Sync()
		return config.Config{}, nil, nil, err
	}
	return cfg, log, db, nil
}

func bootstrapWithRetry(ctx context.Context, service *bootstrap.Service, log *zap.Logger) error {
	log.Info("bootstrap started", zap.String("version", Version))
	delay := time.Second
	for {
		if err := service.EnsureRegistered(ctx); err == nil {
			log.Info("bootstrap completed")
			return nil
		} else {
			log.Warn("bootstrap retry scheduled", zap.Duration("delay", delay), zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		if delay < 5*time.Minute {
			delay *= 2
			if delay > 5*time.Minute {
				delay = 5 * time.Minute
			}
		}
	}
}

func printStatus(service *bootstrap.Service) error {
	status, err := service.Status()
	if err != nil {
		return err
	}
	state := "Not registered"
	if status.Registered {
		state = "Registered"
	}
	lastHeartbeat, _, err := storage.Get(service.DB, "heartbeat_last_success")
	if err != nil {
		return err
	}
	latency, _, err := storage.Get(service.DB, "heartbeat_last_latency_ms")
	if err != nil {
		return err
	}
	health, _, err := storage.Get(service.DB, "heartbeat_last_health")
	if err != nil {
		return err
	}
	lastError, _, err := storage.Get(service.DB, "heartbeat_last_error")
	if err != nil {
		return err
	}
	queueSize, err := storage.QueueSize(service.DB)
	if err != nil {
		return err
	}
	heartbeatState := "PENDING"
	if status.Registered && lastHeartbeat != "" {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, lastHeartbeat); parseErr == nil {
			heartbeatState = "ONLINE"
			if time.Since(parsed) > 2*heartbeat.ConfiguredInterval(service.DB) {
				heartbeatState = "OFFLINE"
			}
		}
	}
	fmt.Printf("Registration Status: %s\nAgent UUID: %s\nPortal URL: %s\nAgent Version: %s\nConfiguration Version: %s\nLast Sync: %s\nHeartbeat Status: %s\nLast Heartbeat: %s\nPortal Latency: %sms\nHealth: %s\nQueue Size: %d\n", state, status.AgentUUID, status.PortalURL, status.AgentVersion, status.ConfigurationVersion, status.LastSync, heartbeatState, lastHeartbeat, latency, health, queueSize)
	if lastError != "" {
		fmt.Printf("Heartbeat Error: %s\n", lastError)
	}
	return nil
}
