# IT Portal Agent

Windows service agent for IT Portal endpoints. Sprint 6.7B adds bootstrap, one-time enrollment, encrypted local credentials, and Portal-owned configuration synchronization to the existing service foundation.

Sprint 6.7C adds a reusable authenticated sync engine and heartbeat delivery. Sprint 6.7D adds Windows hardware inventory. Sprint 6.8A adds a read-only Windows software inventory foundation; command processing, policy execution, remote scripts, uploads, and automatic updates remain intentionally unimplemented.

## Bootstrap and registration

On service start the agent loads the bootstrap-only YAML file, initializes logging, SQLite, and HTTPS, then checks local registration state. A registered agent loads its local Portal configuration. An unregistered agent posts to `POST /api/agent/register` using the enrollment key and device details.

The Portal response supplies the UUID, token, configuration, policy, registration time, and configuration version. The token is encrypted with Windows DPAPI before being placed in SQLite; it is never written to YAML. If the Portal is unavailable during service bootstrap, the agent retries indefinitely with exponential backoff while the service remains running.

The YAML file contains only `portal_url`, `enrollment_key`, `tls_validation`, `proxy`, and `log_level`. All operational settings are Portal-owned and saved in SQLite.

## Sync engine and heartbeat

`internal/sync` is the shared transport for Portal payloads. It owns token authentication, HMAC request signing, retries, response validation, optional gzip encoding, and FIFO offline delivery through the SQLite `queue` table. Future inventory, software, command-result, policy-sync, and upload payloads should use this engine rather than making HTTP calls directly.

`internal/heartbeat` starts only after successful registration. It sends the agent identity, host and operating-system details, resource health, uptime, and the previous Portal latency to `POST /api/agent/heartbeat`. The default interval is 30 seconds and is replaced by the Portal-provided `heartbeatInterval` configuration when present. Failed deliveries are persisted and retried automatically; queued records are removed only after a successful response.

The `status` command reports registration, heartbeat state (`ONLINE`, `OFFLINE`, or `PENDING`), last heartbeat, latency, health, and queue size.

## Hardware inventory

`internal/inventory` collects a Windows hardware snapshot through WMI/COM, Windows registry values, and native system metadata. It covers computer identity, operating system, CPU, memory modules, storage, network adapters, TPM, Secure Boot, BitLocker, Defender, firewall, displays, GPUs, motherboard, BIOS, and users. PowerShell is not used by the inventory collector.

The service sends one snapshot after successful registration and refreshes it every 24 hours. `inventory` runs an immediate snapshot from the CLI and prints duration, payload size, and delivery result. Inventory uses `POST /api/agent/inventory` through the sync engine; if Portal is unavailable, the raw JSON payload remains in the SQLite queue and is replayed later.

## Hardware intelligence

Each snapshot includes a SHA-256 hardware fingerprint. Identical snapshots are skipped while heartbeat continues normally. Changed snapshots include section-level history entries for memory, storage, BIOS, operating system, NIC, and other hardware changes. Asset-link hints are sent in priority order: serial number, machine UUID, hostname, then existing agent UUID; the Portal remains responsible for applying an update or presenting a candidate without creating duplicates.

The Health Engine V2 combines CPU, memory, disk, heartbeat, Portal latency, TPM, BitLocker, Defender, and firewall scores. It returns `Healthy`, `Warning`, or `Critical` with explanations such as high RAM usage, a nearly full disk, or a disabled firewall.

## Software inventory

`internal/software` reads only the standard Windows uninstall registry views: HKLM 64-bit, HKLM 32-bit, and HKCU. It normalizes and deduplicates records, computes a deterministic SHA-256 fingerprint, and sends snapshots through `internal/sync` to `POST /api/agent/software`. The service waits for the Portal-provided software interval (24 hours by default), so registration and heartbeat remain lightweight. If the Portal is unavailable, the same authenticated payload is retained in the SQLite queue. The `software` CLI command performs one on-demand collection and reports collection duration, payload size, item count, changed/skipped counts, fingerprint, and result.

## Configuration flow

Registration persists Portal configuration locally. `reload-config` makes an authenticated `GET /api/agent/configuration` request without registering again, allowing Portal configuration to remain the source of truth. The storage model supports a newly returned token, which prepares the agent for token rotation.

## Build and test

```powershell
go build ./cmd/itportal-agent
go build ./...
go test ./...
```

To build a Windows x64 executable:

```powershell
.\scripts\build-windows-amd64.ps1
```

## CLI

Run from an elevated PowerShell for Windows service changes:

```powershell
.\itportal-agent.exe install
.\itportal-agent.exe uninstall
.\itportal-agent.exe start
.\itportal-agent.exe stop
.\itportal-agent.exe restart
.\itportal-agent.exe register
.\itportal-agent.exe status
.\itportal-agent.exe inventory
.\itportal-agent.exe reload-config
.\itportal-agent.exe version
.\itportal-agent.exe config
```

The default configuration is created at `C:\ProgramData\ITPortalAgent\config.yaml` on installation or first startup. Set the Portal URL and enrollment key before the first registration. `register`, `status`, `inventory`, and `reload-config` also work through `go run ./cmd/itportal-agent ...` without installing the Windows service.

See [the installation guide](docs/installation.md), [architecture](docs/architecture.md), and [folder structure](docs/folder-structure.md).
