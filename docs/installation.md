# Installation Guide

1. Build the agent with `go build ./cmd/itportal-agent` or run `scripts\build-windows-amd64.ps1`.
2. Open an elevated PowerShell in the executable directory.
3. Run `./itportal-agent.exe install` to register the `ITPortalAgent` Windows service.
4. Edit `C:\ProgramData\ITPortalAgent\config.yaml` with the Portal URL and one-time enrollment key. Do not add UUIDs, tokens, or operational settings to this file.
5. Run `./itportal-agent.exe start`. The service registers automatically when no local UUID exists.

The installer creates `C:\ProgramData\ITPortalAgent` and its `logs`, `cache`, `downloads`, and `queue` subdirectories. The SQLite database is initialized when the service first starts.

For developer operation without installing the service, use `go run ./cmd/itportal-agent register`, `status`, or `reload-config`.

To remove the service, stop it and run `./itportal-agent.exe uninstall` from an elevated PowerShell.
