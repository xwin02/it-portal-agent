package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/itportal/it-portal-agent/internal/config"
	"github.com/itportal/it-portal-agent/internal/logger"
	"github.com/itportal/it-portal-agent/internal/storage"
	"github.com/itportal/it-portal-agent/internal/windowsservice"
	"go.uber.org/zap"
)

const Version = "0.1.0"

func Run(arguments []string) error {
	if len(arguments) == 0 { return runService() }
	switch strings.ToLower(arguments[0]) {
	case "install": return windowsservice.Install()
	case "uninstall": return windowsservice.Uninstall()
	case "start": return windowsservice.Start()
	case "stop": return windowsservice.Stop()
	case "restart": return windowsservice.Restart()
	case "version": fmt.Println(Version); return nil
	case "config": fmt.Println(config.Path()); return nil
	default: return fmt.Errorf("unknown command %q; valid commands: install, uninstall, start, stop, restart, version, config", arguments[0])
	}
}

func runService() error {
	cfg, err := config.LoadOrCreate()
	if err != nil { return err }
	log, err := logger.New(cfg.LogLevel)
	if err != nil { return err }
	defer log.Sync()
	db, err := storage.Open()
	if err != nil { return err }
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log.Info("agent service initialized", zap.String("version", Version))
	if err := windowsservice.Run(cancel); err != nil { return err }
	<-ctx.Done()
	return nil
}
