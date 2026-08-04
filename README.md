# IT Portal Agent

Windows service agent for IT Portal endpoints. Sprint 6.7B adds bootstrap, one-time enrollment, encrypted local credentials, and Portal-owned configuration synchronization to the existing service foundation.

It does not perform heartbeats, hardware or software inventory, command processing, policy execution, or automatic updates.

## Bootstrap and registration

On service start the agent loads the bootstrap-only YAML file, initializes logging, SQLite, and HTTPS, then checks local registration state. A registered agent loads its local Portal configuration. An unregistered agent posts to `POST /api/agent/register` using the enrollment key and device details.

The Portal response supplies the UUID, token, configuration, policy, registration time, and configuration version. The token is encrypted with Windows DPAPI before being placed in SQLite; it is never written to YAML. If the Portal is unavailable during service bootstrap, the agent retries indefinitely with exponential backoff while the service remains running.

The YAML file contains only `portal_url`, `enrollment_key`, `tls_validation`, `proxy`, and `log_level`. All operational settings are Portal-owned and saved in SQLite.

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
.\itportal-agent.exe reload-config
.\itportal-agent.exe version
.\itportal-agent.exe config
```

The default configuration is created at `C:\ProgramData\ITPortalAgent\config.yaml` on installation or first startup. Set the Portal URL and enrollment key before the first registration. `register`, `status`, and `reload-config` also work through `go run ./cmd/itportal-agent ...` without installing the Windows service.

See [the installation guide](docs/installation.md), [architecture](docs/architecture.md), and [folder structure](docs/folder-structure.md).
