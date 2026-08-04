package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/itportal/it-portal-agent/internal/bootstrap"
	"github.com/itportal/it-portal-agent/internal/config"
	"github.com/itportal/it-portal-agent/internal/logger"
	"github.com/itportal/it-portal-agent/internal/storage"
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
	default:
		return fmt.Errorf("unknown command %q; valid commands: install, uninstall, start, stop, restart, register, status, reload-config, version, config", arguments[0])
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
	start := func() { bootstrapWithRetry(ctx, service, log) }
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

func bootstrapWithRetry(ctx context.Context, service *bootstrap.Service, log *zap.Logger) {
	log.Info("bootstrap started", zap.String("version", Version))
	delay := time.Second
	for {
		if err := service.EnsureRegistered(ctx); err == nil {
			log.Info("bootstrap completed")
			return
		} else {
			log.Warn("bootstrap retry scheduled", zap.Duration("delay", delay), zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
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
	fmt.Printf("Registration Status: %s\nAgent UUID: %s\nPortal URL: %s\nAgent Version: %s\nConfiguration Version: %s\nLast Sync: %s\n", state, status.AgentUUID, status.PortalURL, status.AgentVersion, status.ConfigurationVersion, status.LastSync)
	return nil
}
