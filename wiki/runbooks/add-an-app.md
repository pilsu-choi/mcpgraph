---
type: Runbook
title: Add a new AI app
description: Steps to support another AI app's MCP config.
tags: [runbook]
timestamp: 2026-10-18T06:45:00Z
---

# Add a new AI app

1. `internal/collector/config/<app>.go`: an empty struct with `App() string` and
   `Parse([]byte) ([]serverConfig, error)`. Usually it decodes the top-level map and calls
   `serversFromMap`. `cursor.go` is the smallest example.
2. `paths.go`: add candidates per OS, marked `TODO(verify)` until checked
   ([runbook](verify-config-paths.md)).
3. `testdata/configs/<app>/...`: a fixture, with fake secrets containing `FAKE`. If you add a new
   fake secret, add it to `testutil.FakeSecrets`.
4. Add a case to `TestParsers`, and extend `FakeHome` and `TestCollectFakeHome`.
5. Optionally teach `process.AppName` the app's process names.
