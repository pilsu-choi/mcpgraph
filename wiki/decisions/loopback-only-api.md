---
type: Design Decision
title: Loopback-only, read-only API
description: The API binds only 127.0.0.1, checks the Host header, and exposes GET routes only.
tags: [decision, security, api]
timestamp: 2026-10-18T06:23:01Z
---

> **Archived with the API** on `archive/v1-daemon` ([why](archive-v1-daemon.md)). It applies again when the API returns in milestone 12.

# Loopback-only, read-only API

## Decision
- `api.ValidateAddr` accepts exactly `127.0.0.1:<port>`. It rejects `localhost` (a name can
  resolve elsewhere), `0.0.0.0`, `[::]` and `:port` (empty host means all interfaces).
- After `Listen`, the bound address is checked again.
- Middleware rejects any `Host` header other than `127.0.0.1` or `localhost`, with any port. This
  stops DNS rebinding.
- Go 1.22 routes such as `GET /graph` make other methods return 405 automatically.
- There are no CORS headers, and internal errors are not echoed to clients.

## Verification
`TestRefusesNonLoopbackAddress`, `TestErrorsAndMethods`, and `docs/logs/08-api.log`
(405 and 403 cases), plus `docs/logs/09-api-refuses-non-loopback.log`.
