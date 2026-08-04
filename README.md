# IT Portal Agent

Windows service foundation for the IT Portal endpoint agent. This project intentionally contains infrastructure only: configuration, logging, SQLite storage, service control, HTTPS request signing, and extension interfaces for future sprints.

It does not perform inventory, software scanning, heartbeats, registration, command processing, policy handling, or automatic updates.

## Build and test

```powershell
go build ./cmd/itportal-agent
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
.\itportal-agent.exe version
.\itportal-agent.exe config
```

The default configuration is created at `C:\ProgramData\ITPortalAgent\config.yaml` on first service startup or installation. Edit the portal URL and credentials before enabling future agent workloads.

See [the installation guide](docs/installation.md), [architecture](docs/architecture.md), and [folder structure](docs/folder-structure.md).
