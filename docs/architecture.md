# Architecture

`cmd/itportal-agent` is the executable entry point. `internal/app` selects CLI actions or runs the Windows service. Service initialization creates the YAML configuration, Zap file logger, SQLite schema, and bootstrap service.

The HTTP client is reusable and enforces TLS 1.2+, timeouts, transparent compression, retry attempts, and HMAC SHA-256 request signing. The signing canonical payload is timestamp, nonce, method, URL path, and body hash; its headers are `X-ITPortal-Timestamp`, `X-ITPortal-Nonce`, and `X-ITPortal-Signature`.

`internal/bootstrap` owns registration and configuration synchronization. It uses the existing HTTP and storage packages, keeps Portal configuration in SQLite, and protects the agent token with Windows DPAPI. The Windows service begins bootstrap in the background and retries registration indefinitely with capped exponential backoff.

`internal/sync` is the shared authenticated transport for heartbeat and future payload types. It performs HMAC signing, retries, response parsing, optional compression, and FIFO offline queue delivery. `internal/heartbeat` collects host metrics and sends only through the sync engine after registration succeeds.

`internal/inventory` is the Windows-only hardware snapshot producer. It uses WMI/COM and registry APIs, then submits the snapshot through `internal/sync`; it never performs direct HTTP communication. Inventory runs after bootstrap and on a 24-hour cadence.

Inventory normalizes volatile fields out of its SHA-256 fingerprint, compares the prior SQLite snapshot, and sends only changed hardware. It attaches change sections, asset-link matching hints, and Health Engine V2 scores to the payload. The Portal can use those fields to update an existing asset, suggest a candidate, and render snapshot history.

Future functionality must be implemented through the reserved interfaces in `internal/contracts`. Software inventory is now a read-only registry snapshot workload: `internal/software` collects and normalizes uninstall entries, then uses `internal/sync` and the SQLite queue for delivery. Command, policy, and update workloads remain inactive.
