---
type: Data Model
title: Graph model
description: Node kinds, edge kinds and how stable node IDs are derived.
tags: [architecture, model]
timestamp: 2026-10-18T06:50:00Z
---

# Graph model

Defined in [`internal/model`](../components/model.md).

| Node | Key fields | Stable ID key |
| --- | --- | --- |
| Host | hostname, os, arch, scanned_at | hostname |
| AIApp | name, config_path | name + config path |
| MCPServer | name, app, command, args (redacted), env keys, transport, url (redacted), source_config | config path + JSON path |
| Process | pid, ppid, exe, args/cmdline (redacted), started_at, ai_app, mcp_like | pid + create time |
| Credential | kind (env/file/inline), key_name, location, reason. **No value** | location |
| DataSource | kind (filesystem/database/saas), identifier | kind + identifier |

| Edge | From → To |
| --- | --- |
| CONFIGURES | AIApp → MCPServer |
| RUNS_AS | MCPServer → Process |
| USES_SECRET | MCPServer → Credential |
| ACCESSES | MCPServer → DataSource |
| CHILD_OF | Process → Process (child → parent) |

`ID = kind + ":" + sha256(key)[:12]`. Why: [stable node IDs](../decisions/stable-node-ids.md).

A `Node` is a struct with one optional pointer per kind (`Host`, `App`, `Server`, ...). This acts as a
tagged union without interfaces or reflection, and nil pointers are omitted from JSON.
