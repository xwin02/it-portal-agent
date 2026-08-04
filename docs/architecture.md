# Architecture

`cmd/itportal-agent` is the executable entry point. `internal/app` selects CLI actions or runs the Windows service. Service initialization creates the YAML configuration, Zap file logger, and SQLite schema.

The HTTP client is reusable and enforces TLS 1.2+, timeouts, transparent compression, retry attempts, and HMAC SHA-256 request signing. The signing canonical payload is timestamp, nonce, method, URL path, and body hash; its headers are `X-ITPortal-Timestamp`, `X-ITPortal-Nonce`, and `X-ITPortal-Signature`.

Future functionality must be implemented through the reserved interfaces in `internal/contracts`. No scheduled or network workload is activated in this foundation.
