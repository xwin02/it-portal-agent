# Architecture

`cmd/itportal-agent` is the executable entry point. `internal/app` selects CLI actions or runs the Windows service. Service initialization creates the YAML configuration, Zap file logger, SQLite schema, and bootstrap service.

The HTTP client is reusable and enforces TLS 1.2+, timeouts, transparent compression, retry attempts, and HMAC SHA-256 request signing. The signing canonical payload is timestamp, nonce, method, URL path, and body hash; its headers are `X-ITPortal-Timestamp`, `X-ITPortal-Nonce`, and `X-ITPortal-Signature`.

`internal/bootstrap` owns registration and configuration synchronization. It uses the existing HTTP and storage packages, keeps Portal configuration in SQLite, and protects the agent token with Windows DPAPI. The Windows service begins bootstrap in the background and retries registration indefinitely with capped exponential backoff.

Future functionality must be implemented through the reserved interfaces in `internal/contracts`. No heartbeat, inventory, command, policy, or update workload is activated.
