# Folder Structure

| Directory | Purpose |
| --- | --- |
| `cmd/` | Executable entry point |
| `internal/app/` | CLI and service composition |
| `internal/config/` | YAML configuration and ProgramData paths |
| `internal/logger/` | Zap logger setup |
| `internal/storage/` | SQLite database migration |
| `internal/httpclient/` | Reusable HTTP client |
| `internal/security/` | HMAC request signing |
| `internal/sync/` | Authenticated, retried, compressed, queue-backed payload transport |
| `internal/heartbeat/` | Heartbeat collection, health scoring, and interval scheduling |
| `internal/inventory/` | Windows WMI/registry hardware snapshot collection and delivery |
| `internal/software/` | Windows uninstall-registry software collection, normalization, fingerprinting, and scheduled delivery |
| `internal/windowsservice/` | Windows service hosting and control |
| `internal/contracts/` | Future-sprint extension interfaces |
| `pkg/` | Reserved for public reusable packages |
| `configs/` | Example configuration |
| `service/`, `api/`, `storage/`, `logger/`, `crypto/`, `queue/`, `updater/`, `installer/` | Reserved enterprise package areas |
| `scripts/` | Build automation |
| `docs/` | Project documentation |
