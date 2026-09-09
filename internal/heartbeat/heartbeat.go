// Package heartbeat produces the agent liveness payload and routes it through sync.Engine.
package heartbeat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/itportal/it-portal-agent/internal/storage"
	syncengine "github.com/itportal/it-portal-agent/internal/sync"
	"go.uber.org/zap"
)

const DefaultInterval = 30 * time.Second

type Health struct {
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"`
	Disk    float64 `json:"disk"`
	Overall float64 `json:"overall"`
	Status  string  `json:"status"`
}

type Payload struct {
	AgentUUID         string  `json:"agentUuid"`
	CurrentTimestamp  string  `json:"timestamp"`
	Hostname          string  `json:"hostname"`
	ComputerName      string  `json:"computerName"`
	AgentVersion      string  `json:"agentVersion"`
	OSName            string  `json:"operatingSystem"`
	OSVersion         string  `json:"osVersion"`
	Architecture      string  `json:"architecture"`
	CurrentLoggedUser string  `json:"loggedUser"`
	IPAddress         string  `json:"ipAddress"`
	MACAddress        string  `json:"macAddress"`
	CPUUsage          float64 `json:"cpuUsage"`
	MemoryUsage       float64 `json:"ramUsage"`
	DiskUsage         float64 `json:"diskUsage"`
	SystemUptime      int64   `json:"systemUptime"`
	PortalLatency     int64   `json:"portalLatency"`
	Health            Health  `json:"health"`
}

type Status struct {
	Started       bool
	LastHeartbeat string
	Latency       string
	Health        string
	QueueSize     int
	Error         string
}

type Engine struct {
	DB      *sql.DB
	Sync    *syncengine.Engine
	Log     *zap.Logger
	Version string
}

func New(db *sql.DB, transport *syncengine.Engine, log *zap.Logger, version string) *Engine {
	return &Engine{DB: db, Sync: transport, Log: log, Version: version}
}

func (e *Engine) Run(ctx context.Context) {
	e.Log.Info("heartbeat started")
	started := false
	for {
		if started {
			interval := e.interval()
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			case <-timer.C:
			}
		}
		started = true
		if _, err := e.send(ctx); err != nil && ctx.Err() != nil {
			return
		}
	}
}

func (e *Engine) send(ctx context.Context) (syncengine.Result, error) {
	previousError, _, _ := storage.Get(e.DB, "heartbeat_last_error")
	flushed, flushErr := e.Sync.Flush(ctx)
	if flushErr != nil {
		e.Log.Warn("heartbeat failed", zap.Error(flushErr))
	}
	payload, err := e.collect()
	if err != nil {
		e.persistFailure(err)
		e.Log.Warn("heartbeat failed", zap.Error(err))
		return syncengine.Result{}, err
	}
	result, sendErr := e.Sync.SendJSON(ctx, "/api/agent/heartbeat", syncengine.HeartbeatType, payload)
	if sendErr != nil {
		e.persistFailure(sendErr)
		if errors.Is(sendErr, syncengine.ErrQueued) {
			e.Log.Warn("queued offline", zap.Error(sendErr))
		} else {
			e.Log.Warn("heartbeat failed", zap.Error(sendErr))
		}
		return result, sendErr
	}
	e.persistSuccess(result, payload.Health)
	e.Log.Info("heartbeat sent", zap.Int64("latency_ms", result.Latency.Milliseconds()))
	if previousError != "" || flushed > 0 {
		e.Log.Info("heartbeat restored", zap.Int("queued_sent", flushed))
	}
	return result, nil
}

func (e *Engine) collect() (Payload, error) {
	agentUUID, found, err := storage.Get(e.DB, "agent_uuid")
	if err != nil {
		return Payload{}, err
	}
	if !found || agentUUID == "" {
		return Payload{}, syncengine.ErrNotRegistered
	}
	hostname, err := os.Hostname()
	if err != nil {
		return Payload{}, fmt.Errorf("read hostname: %w", err)
	}
	metrics := readMetrics()
	latency := int64(0)
	if value, found, readErr := storage.Get(e.DB, "heartbeat_last_latency_ms"); readErr == nil && found {
		latency, _ = strconv.ParseInt(value, 10, 64)
	}
	return Payload{
		AgentUUID: agentUUID, CurrentTimestamp: time.Now().UTC().Format(time.RFC3339), Hostname: hostname, ComputerName: hostname,
		AgentVersion: e.Version, OSName: runtime.GOOS, OSVersion: metrics.OSVersion, Architecture: runtime.GOARCH,
		CurrentLoggedUser: metrics.LoggedUser, IPAddress: metrics.IPAddress, MACAddress: metrics.MACAddress,
		CPUUsage: metrics.CPU, MemoryUsage: metrics.Memory, DiskUsage: metrics.Disk, SystemUptime: metrics.Uptime,
		PortalLatency: latency, Health: CalculateHealth(metrics.CPU, metrics.Memory, metrics.Disk),
	}, nil
}

func (e *Engine) interval() time.Duration {
	return ConfiguredInterval(e.DB)
}

// ConfiguredInterval returns the Portal heartbeat interval, falling back to the
// documented 30-second default when no valid Portal value is available.
func ConfiguredInterval(db *sql.DB) time.Duration {
	value, found, err := storage.Get(db, "portal_configuration")
	if err == nil && found {
		var raw map[string]any
		if json.Unmarshal([]byte(value), &raw) == nil {
			for _, key := range []string{"heartbeatInterval", "heartbeat_interval"} {
				switch value := raw[key].(type) {
				case float64:
					if value > 0 {
						return time.Duration(value) * time.Second
					}
				case string:
					if duration, parseErr := time.ParseDuration(value); parseErr == nil && duration > 0 {
						return duration
					}
				}
			}
		}
	}
	return DefaultInterval
}

func (e *Engine) persistSuccess(result syncengine.Result, health Health) {
	_ = storage.Put(e.DB, "heartbeat_last_success", time.Now().UTC().Format(time.RFC3339))
	_ = storage.Put(e.DB, "heartbeat_last_latency_ms", strconv.FormatInt(result.Latency.Milliseconds(), 10))
	_ = storage.Put(e.DB, "heartbeat_last_health", health.Status)
	_ = storage.Put(e.DB, "heartbeat_last_cpu", strconv.FormatFloat(health.CPU, 'f', 2, 64))
	_ = storage.Put(e.DB, "heartbeat_last_memory", strconv.FormatFloat(health.Memory, 'f', 2, 64))
	_ = storage.Put(e.DB, "heartbeat_last_disk", strconv.FormatFloat(health.Disk, 'f', 2, 64))
	_ = storage.Put(e.DB, "heartbeat_last_error", "")
}

func (e *Engine) persistFailure(err error) {
	_ = storage.Put(e.DB, "heartbeat_last_error", err.Error())
}

func CalculateHealth(cpu, memory, disk float64) Health {
	values := []float64{cpu, memory, disk}
	var total, maximum float64
	count := 0
	for _, value := range values {
		if value < 0 || value > 100 {
			continue
		}
		total += value
		count++
		if value > maximum {
			maximum = value
		}
	}
	overall := 0.0
	if count > 0 {
		overall = total / float64(count)
	}
	status := "Healthy"
	if maximum >= 90 || overall >= 85 {
		status = "Critical"
	} else if maximum >= 75 || overall >= 65 {
		status = "Warning"
	}
	return Health{CPU: cpu, Memory: memory, Disk: disk, Overall: overall, Status: status}
}

type metrics struct {
	OSVersion         string
	LoggedUser        string
	IPAddress         string
	MACAddress        string
	CPU, Memory, Disk float64
	Uptime            int64
}

func readMetrics() metrics {
	result := metrics{OSVersion: "unknown"}
	command := `$os=Get-CimInstance Win32_OperatingSystem; $cs=Get-CimInstance Win32_ComputerSystem; $cpuSamples=@(); 1..3 | ForEach-Object { $sample=(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average; if ($null -ne $sample) { $cpuSamples += [double]$sample }; if ($_ -lt 3) { Start-Sleep -Milliseconds 1000 } }; $cpu=if ($cpuSamples.Count -gt 0) { ($cpuSamples | Measure-Object -Average).Average } else { 0 }; $disk=Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'"; $ip=(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {$_.IPAddress -notlike '127.*' -and $_.PrefixOrigin -ne 'WellKnown'} | Select-Object -First 1 -ExpandProperty IPAddress); $mac=(Get-NetAdapter -ErrorAction SilentlyContinue | Where-Object Status -eq 'Up' | Select-Object -First 1 -ExpandProperty MacAddress); [pscustomobject]@{OSVersion=$os.Caption+' '+$os.Version; LoggedUser=$cs.UserName; IPAddress=$ip; MACAddress=$mac; CPU=$cpu; Memory=(100-($os.FreePhysicalMemory/$os.TotalVisibleMemorySize*100)); Disk=(100-($disk.FreeSpace/$disk.Size*100)); Uptime=((Get-Date)-$os.LastBootUpTime).TotalSeconds} | ConvertTo-Json -Compress`
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command).Output()
	if err != nil {
		return result
	}
	var value struct {
		OSVersion, LoggedUser, IPAddress, MACAddress string
		CPU, Memory, Disk, Uptime                    float64
	}
	if json.Unmarshal(output, &value) != nil {
		return result
	}
	result.OSVersion, result.LoggedUser, result.IPAddress, result.MACAddress = strings.TrimSpace(value.OSVersion), strings.TrimSpace(value.LoggedUser), strings.TrimSpace(value.IPAddress), strings.TrimSpace(value.MACAddress)
	result.CPU, result.Memory, result.Disk, result.Uptime = value.CPU, value.Memory, value.Disk, int64(value.Uptime)
	return result
}
