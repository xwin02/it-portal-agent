package inventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/itportal/it-portal-agent/internal/heartbeat"
	"github.com/itportal/it-portal-agent/internal/storage"
	syncengine "github.com/itportal/it-portal-agent/internal/sync"
	"go.uber.org/zap"
)

const (
	InventoryType     = "inventory"
	InventoryEndpoint = "/api/agent/inventory"
	DefaultInterval   = 24 * time.Hour
)

type Engine struct {
	DB      *sql.DB
	Sync    *syncengine.Engine
	Log     *zap.Logger
	Version string
}

func New(db *sql.DB, transport *syncengine.Engine, log *zap.Logger, version string) *Engine {
	return &Engine{DB: db, Sync: transport, Log: log, Version: version}
}

// Run sends an initial snapshot, then refreshes it at the configured 24-hour
// cadence. A future Portal refresh command can call RunOnce directly.
func (e *Engine) Run(ctx context.Context) {
	for {
		if _, err := e.RunOnce(ctx); err != nil && ctx.Err() != nil {
			return
		}
		timer := time.NewTimer(DefaultInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (e *Engine) RunOnce(ctx context.Context) (Report, error) {
	started := time.Now()
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	if _, err := e.Sync.Flush(ctx); err != nil {
		e.Log.Warn("inventory queue flush failed", zap.Error(err))
	}
	agentUUID, found, err := storage.Get(e.DB, "agent_uuid")
	if err != nil {
		return e.failure(started, err)
	}
	if !found || agentUUID == "" {
		return e.failure(started, syncengine.ErrNotRegistered)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return e.failure(started, fmt.Errorf("read hostname: %w", err))
	}
	collectStarted := time.Now()
	hardware, err := Collect(ctx)
	if err != nil {
		return e.failure(started, err)
	}
	collectDuration := time.Since(collectStarted)
	if hardware.General.ComputerName == "" {
		hardware.General.ComputerName = hostname
	}
	fingerprint, err := Fingerprint(hardware)
	if err != nil {
		return e.failure(started, err)
	}
	hardwareCount := HardwareCount(hardware)
	previousFingerprint, _, _ := storage.Get(e.DB, "inventory_last_fingerprint")
	if previousFingerprint == fingerprint {
		report := Report{CollectDuration: collectDuration, Duration: time.Since(started), HardwareCount: hardwareCount, SkippedItems: hardwareCount, Fingerprint: fingerprint, Result: "SKIPPED"}
		_ = storage.Put(e.DB, "inventory_last_checked", time.Now().UTC().Format(time.RFC3339Nano))
		e.Log.Info("inventory unchanged", zap.String("fingerprint", fingerprint), zap.Int("skipped_items", hardwareCount))
		return report, nil
	}
	var previousHardware *Hardware
	if value, found, readErr := storage.Get(e.DB, "inventory_last_hardware"); readErr == nil && found && value != "" {
		var decoded Hardware
		if json.Unmarshal([]byte(value), &decoded) == nil {
			previousHardware = &decoded
		}
	}
	changes := DiffHardware(previousHardware, hardware)
	assetLink := BuildAssetLinkHints(hardware, agentUUID, hostname)
	assessment := e.health(hardware)
	payload := Payload{Timestamp: time.Now().UTC(), AgentUUID: agentUUID, AgentVersion: e.Version, Hostname: hostname, Fingerprint: fingerprint, ChangedItems: len(changes), Changes: changes, AssetLink: assetLink, Health: assessment, Hardware: hardware}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return e.failure(started, fmt.Errorf("marshal inventory payload: %w", err))
	}
	uploadStarted := time.Now()
	result, sendErr := e.Sync.SendJSON(ctx, InventoryEndpoint, InventoryType, payload)
	report := Report{CollectDuration: collectDuration, UploadDuration: time.Since(uploadStarted), Duration: time.Since(started), PayloadSize: len(encoded), HardwareCount: hardwareCount, ChangedItems: len(changes), Fingerprint: fingerprint}
	if sendErr != nil {
		if errors.Is(sendErr, syncengine.ErrQueued) {
			report.Queued = true
			report.Result = "QUEUED"
			e.persistSnapshot(hardware, fingerprint)
			e.persistFailure(sendErr, report)
			e.Log.Warn("inventory queued offline", zap.Int("payload_bytes", report.PayloadSize), zap.Error(sendErr))
			return report, sendErr
		}
		report.Result = "FAILED"
		e.persistFailure(sendErr, report)
		e.Log.Warn("inventory failed", zap.Error(sendErr))
		return report, sendErr
	}
	report.Result = "SENT"
	report.Duration = time.Since(started)
	e.persistSnapshot(hardware, fingerprint)
	e.persistHistory(hardware, fingerprint, changes)
	e.persistTimeline(changes, fingerprint)
	e.persistSuccess(report)
	e.Log.Info("inventory sent", zap.Int("payload_bytes", report.PayloadSize), zap.Int64("latency_ms", result.Latency.Milliseconds()))
	return report, nil
}

func (e *Engine) failure(started time.Time, err error) (Report, error) {
	report := Report{Duration: time.Since(started), Result: "FAILED"}
	e.persistFailure(err, report)
	e.Log.Warn("inventory failed", zap.Error(err))
	return report, err
}

func (e *Engine) persistSuccess(report Report) {
	_ = storage.Put(e.DB, "inventory_last_success", time.Now().UTC().Format(time.RFC3339Nano))
	_ = storage.Put(e.DB, "inventory_last_duration_ms", fmt.Sprintf("%d", report.Duration.Milliseconds()))
	_ = storage.Put(e.DB, "inventory_last_payload_size", fmt.Sprintf("%d", report.PayloadSize))
	_ = storage.Put(e.DB, "inventory_last_error", "")
}

func (e *Engine) persistSnapshot(hardware Hardware, fingerprint string) {
	if value, err := json.Marshal(hardware); err == nil {
		_ = storage.Put(e.DB, "inventory_last_hardware", string(value))
	}
	_ = storage.Put(e.DB, "inventory_last_fingerprint", fingerprint)
}

func (e *Engine) persistHistory(hardware Hardware, fingerprint string, changes []HardwareChange) {
	payload, payloadErr := json.Marshal(hardware)
	changeData, changeErr := json.Marshal(changes)
	if payloadErr == nil && changeErr == nil {
		_ = storage.RecordInventoryHistory(e.DB, fingerprint, payload, changeData)
	}
}

func (e *Engine) persistTimeline(changes []HardwareChange, fingerprint string) {
	event := InventoryHistoryEvent{Timestamp: time.Now().UTC(), Fingerprint: fingerprint, Changes: changes}
	var timeline []InventoryHistoryEvent
	if value, found, err := storage.Get(e.DB, "inventory_change_timeline"); err == nil && found {
		_ = json.Unmarshal([]byte(value), &timeline)
	}
	timeline = append(timeline, event)
	if len(timeline) > 50 {
		timeline = timeline[len(timeline)-50:]
	}
	if value, err := json.Marshal(timeline); err == nil {
		_ = storage.Put(e.DB, "inventory_change_timeline", string(value))
	}
	if value, err := json.Marshal(changes); err == nil {
		_ = storage.Put(e.DB, "inventory_last_changes", string(value))
	}
}

func (e *Engine) persistFailure(err error, report Report) {
	_ = storage.Put(e.DB, "inventory_last_duration_ms", fmt.Sprintf("%d", report.Duration.Milliseconds()))
	_ = storage.Put(e.DB, "inventory_last_payload_size", fmt.Sprintf("%d", report.PayloadSize))
	_ = storage.Put(e.DB, "inventory_last_error", err.Error())
}

func (e *Engine) health(hardware Hardware) HealthAssessment {
	cpu := storedFloat(e.DB, "heartbeat_last_cpu")
	memory := hardware.RAM.UsagePercent
	if hardware.RAM.TotalBytes == 0 {
		memory = -1
	}
	disk := -1.0
	if hardware.Disk.TotalBytes > 0 && hardware.Disk.FreeBytes <= hardware.Disk.TotalBytes {
		disk = float64(hardware.Disk.TotalBytes-hardware.Disk.FreeBytes) / float64(hardware.Disk.TotalBytes) * 100
	}
	return CalculateHealthV2(cpu, memory, disk, storedHeartbeatStatus(e.DB), storedFloat(e.DB, "heartbeat_last_latency_ms"), hardware.Security)
}

func storedFloat(db *sql.DB, key string) float64 {
	value, found, err := storage.Get(db, key)
	if err != nil || !found {
		return -1
	}
	var parsed float64
	if _, scanErr := fmt.Sscanf(value, "%f", &parsed); scanErr != nil {
		return -1
	}
	return parsed
}

func storedHeartbeatStatus(db *sql.DB) string {
	value, found, err := storage.Get(db, "heartbeat_last_success")
	if err != nil || !found || value == "" {
		return "PENDING"
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || time.Since(parsed) > 2*heartbeat.ConfiguredInterval(db) {
		return "OFFLINE"
	}
	return "ONLINE"
}
