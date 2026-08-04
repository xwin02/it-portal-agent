package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const AppDirectoryName = "ITPortalAgent"

// Config contains all agent settings. Fields for future sprints are intentionally
// persisted now, but no heartbeat, inventory, queue, or updater workload runs yet.
type Config struct {
	PortalURL         string        `yaml:"portal_url"`
	AgentUUID         string        `yaml:"agent_uuid"`
	AgentToken        string        `yaml:"agent_token"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	InventoryInterval time.Duration `yaml:"inventory_interval"`
	LogLevel          string        `yaml:"log_level"`
	RetryInterval     time.Duration `yaml:"retry_interval"`
	QueueSize         int           `yaml:"queue_size"`
	AutoUpdate        bool          `yaml:"auto_update"`
}

func Default() Config {
	return Config{
		PortalURL: "https://portal.example.com",
		LogLevel: "info", RetryInterval: time.Minute, QueueSize: 100,
		HeartbeatInterval: 5 * time.Minute, InventoryInterval: 24 * time.Hour,
		AutoUpdate: false,
	}
}

func RootDir() string {
	if programData := os.Getenv("ProgramData"); programData != "" {
		return filepath.Join(programData, AppDirectoryName)
	}
	return filepath.Join(`C:\ProgramData`, AppDirectoryName)
}

func Path() string { return filepath.Join(RootDir(), "config.yaml") }

func EnsureDirectories() error {
	for _, name := range []string{"", "logs", "cache", "downloads", "queue"} {
		if err := os.MkdirAll(filepath.Join(RootDir(), name), 0o750); err != nil {
			return fmt.Errorf("create %s directory: %w", name, err)
		}
	}
	return nil
}

func LoadOrCreate() (Config, error) {
	if err := EnsureDirectories(); err != nil { return Config{}, err }
	path := Path()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := Default()
		return cfg, Save(cfg)
	}
	if err != nil { return Config{}, fmt.Errorf("read configuration: %w", err) }
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil { return Config{}, fmt.Errorf("parse configuration: %w", err) }
	if cfg.LogLevel == "" { cfg.LogLevel = Default().LogLevel }
	return cfg, nil
}

func Save(cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil { return fmt.Errorf("serialize configuration: %w", err) }
	if err := os.WriteFile(Path(), data, 0o600); err != nil { return fmt.Errorf("write configuration: %w", err) }
	return nil
}
