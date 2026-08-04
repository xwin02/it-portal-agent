// Package bootstrap registers an agent once and synchronizes Portal-owned configuration.
package bootstrap

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/itportal/it-portal-agent/internal/config"
	"github.com/itportal/it-portal-agent/internal/httpclient"
	"github.com/itportal/it-portal-agent/internal/storage"
	"go.uber.org/zap"
)

const (
	keyAgentUUID            = "agent_uuid"
	keyAgentToken           = "agent_token"
	keyConfiguration        = "portal_configuration"
	keyPolicy               = "portal_policy"
	keyRegistrationTime     = "registration_time"
	keyLastSync             = "last_configuration_sync"
	keyConfigurationVersion = "configuration_version"
)

type Service struct {
	DB      *sql.DB
	Config  config.Config
	Log     *zap.Logger
	Version string
	client  *httpclient.Client
}

type Status struct {
	Registered                                                         bool
	AgentUUID, PortalURL, AgentVersion, ConfigurationVersion, LastSync string
}

type registerRequest struct {
	EnrollmentKey   string `json:"enrollment_key"`
	Hostname        string `json:"hostname"`
	ComputerName    string `json:"computer_name"`
	MachineUUID     string `json:"machine_uuid"`
	OperatingSystem string `json:"operating_system"`
	OSVersion       string `json:"os_version"`
	Architecture    string `json:"architecture"`
	AgentVersion    string `json:"agent_version"`
}

type portalResponse struct {
	AgentUUID            string          `json:"agent_uuid"`
	AgentToken           string          `json:"agent_token"`
	Configuration        json.RawMessage `json:"configuration"`
	Policy               json.RawMessage `json:"policy"`
	RegistrationTime     string          `json:"registration_time"`
	ConfigurationVersion string          `json:"configuration_version"`
}

func New(db *sql.DB, cfg config.Config, log *zap.Logger, version string) *Service {
	client := httpclient.NewWithOptions(httpclient.Options{Timeout: 30 * time.Second, RetryDelay: time.Second, TLSValidation: cfg.TLSValidation, Proxy: cfg.Proxy})
	return &Service{DB: db, Config: cfg, Log: log, Version: version, client: client}
}

func (s *Service) EnsureRegistered(ctx context.Context) error {
	registered, err := s.Registered()
	if err != nil {
		return err
	}
	if registered {
		if _, found, err := storage.Get(s.DB, keyConfiguration); err != nil {
			return err
		} else if !found {
			s.Log.Warn("registered agent has no local configuration")
		}
		s.Log.Info("bootstrap completed", zap.String("state", "already_registered"))
		return nil
	}
	return s.Register(ctx)
}

func (s *Service) Registered() (bool, error) {
	_, found, err := storage.Get(s.DB, keyAgentUUID)
	return found, err
}

func (s *Service) Register(ctx context.Context) error {
	if s.Config.EnrollmentKey == "" {
		return fmt.Errorf("enrollment_key is required for first registration")
	}
	s.Log.Info("registration started")
	payload, err := s.registrationPayload()
	if err != nil {
		return err
	}
	response, err := s.request(ctx, http.MethodPost, "/api/agent/register", "", payload)
	if err != nil {
		s.Log.Warn("registration failed", zap.Error(err))
		return err
	}
	if response.AgentUUID == "" || response.AgentToken == "" {
		return fmt.Errorf("registration response is missing agent_uuid or agent_token")
	}
	if err := s.saveRegistration(response); err != nil {
		return err
	}
	// The enrollment key is a one-time bootstrap credential. It is never used
	// after a successful registration and is removed from local YAML.
	s.Config.EnrollmentKey = ""
	if err := config.Save(s.Config); err != nil {
		s.Log.Warn("could not clear enrollment key after registration", zap.Error(err))
	}
	s.Log.Info("registration successful", zap.String("agent_uuid", response.AgentUUID))
	s.Log.Info("configuration downloaded")
	return nil
}

func (s *Service) ReloadConfig(ctx context.Context) error {
	token, found, err := storage.GetSecret(s.DB, keyAgentToken)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("agent is not registered")
	}
	response, err := s.request(ctx, http.MethodGet, "/api/agent/configuration", token, nil)
	if err != nil {
		return err
	}
	if len(response.Configuration) == 0 {
		return fmt.Errorf("configuration response is empty")
	}
	if err := s.saveConfiguration(response); err != nil {
		return err
	}
	s.Log.Info("configuration downloaded")
	return nil
}

func (s *Service) Status() (Status, error) {
	status := Status{PortalURL: s.Config.PortalURL, AgentVersion: s.Version}
	var err error
	status.AgentUUID, status.Registered, err = storage.Get(s.DB, keyAgentUUID)
	if err != nil {
		return Status{}, err
	}
	if status.ConfigurationVersion, _, err = storage.Get(s.DB, keyConfigurationVersion); err != nil {
		return Status{}, err
	}
	if status.LastSync, _, err = storage.Get(s.DB, keyLastSync); err != nil {
		return Status{}, err
	}
	return status, nil
}

func (s *Service) registrationPayload() (registerRequest, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return registerRequest{}, fmt.Errorf("read hostname: %w", err)
	}
	return registerRequest{EnrollmentKey: s.Config.EnrollmentKey, Hostname: hostname, ComputerName: hostname, MachineUUID: machineUUID(), OperatingSystem: runtime.GOOS, OSVersion: osVersion(), Architecture: runtime.GOARCH, AgentVersion: s.Version}, nil
}

func (s *Service) request(ctx context.Context, method, endpoint, token string, payload any) (portalResponse, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return portalResponse{}, fmt.Errorf("marshal portal request: %w", err)
		}
		body = strings.NewReader(string(data))
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(s.Config.PortalURL, "/")+endpoint, body)
	if err != nil {
		return portalResponse{}, fmt.Errorf("create portal request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	s.client.Token = token
	response, err := s.client.Do(request)
	if err != nil {
		return portalResponse{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return portalResponse{}, fmt.Errorf("read portal response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return portalResponse{}, fmt.Errorf("portal returned %s: %s", response.Status, strings.TrimSpace(string(data)))
	}
	var result portalResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return portalResponse{}, fmt.Errorf("parse portal response: %w", err)
	}
	return result, nil
}

func (s *Service) saveRegistration(response portalResponse) error {
	if err := storage.Put(s.DB, keyAgentUUID, response.AgentUUID); err != nil {
		return err
	}
	if err := storage.PutSecret(s.DB, keyAgentToken, response.AgentToken); err != nil {
		return err
	}
	if response.RegistrationTime == "" {
		response.RegistrationTime = time.Now().UTC().Format(time.RFC3339)
	}
	if err := storage.Put(s.DB, keyRegistrationTime, response.RegistrationTime); err != nil {
		return err
	}
	return s.saveConfiguration(response)
}

func (s *Service) saveConfiguration(response portalResponse) error {
	if response.AgentToken != "" {
		if err := storage.PutSecret(s.DB, keyAgentToken, response.AgentToken); err != nil {
			return err
		}
	}
	if len(response.Configuration) > 0 {
		if err := storage.Put(s.DB, keyConfiguration, string(response.Configuration)); err != nil {
			return err
		}
	}
	if len(response.Policy) > 0 {
		if err := storage.Put(s.DB, keyPolicy, string(response.Policy)); err != nil {
			return err
		}
	}
	if response.ConfigurationVersion != "" {
		if err := storage.Put(s.DB, keyConfigurationVersion, response.ConfigurationVersion); err != nil {
			return err
		}
	}
	return storage.Put(s.DB, keyLastSync, time.Now().UTC().Format(time.RFC3339))
}

func machineUUID() string {
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "(Get-CimInstance Win32_ComputerSystemProduct).UUID").Output()
	if err != nil {
		return "unknown"
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "unknown"
	}
	return value
}

func osVersion() string {
	output, err := exec.Command("cmd.exe", "/c", "ver").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}
