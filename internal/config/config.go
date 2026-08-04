package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const AppDirectoryName = "ITPortalAgent"

// Config is the bootstrap-only configuration. Operational configuration is
// supplied by the Portal and persisted locally in SQLite.
type Config struct {
	PortalURL     string `yaml:"portal_url"`
	EnrollmentKey string `yaml:"enrollment_key"`
	TLSValidation bool   `yaml:"tls_validation"`
	Proxy         string `yaml:"proxy"`
	LogLevel      string `yaml:"log_level"`
}

func Default() Config {
	return Config{PortalURL: "https://portal.example.com", TLSValidation: true, LogLevel: "info"}
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
	if err := EnsureDirectories(); err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		cfg := Default()
		return cfg, Save(cfg)
	}
	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse configuration: %w", err)
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = Default().LogLevel
	}
	return cfg, nil
}

func Save(cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialize configuration: %w", err)
	}
	if err := os.WriteFile(Path(), data, 0o600); err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}
	return nil
}
