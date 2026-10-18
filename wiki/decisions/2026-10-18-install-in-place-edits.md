---
type: Decision
title: install edits configs in place, and uninstall restores or unwraps
description: install replaces only the command and args values using byte spans instead of re-serializing the file; uninstall restores the backup if the file is untouched, otherwise removes just the wrapping.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/install
tags: [decision, install, m8]
timestamp: 2026-10-18T07:55:00Z
---

# install edits configs in place, and uninstall restores or unwraps

**Decided in milestone 8.**

## Edit in place

The simple way to rewrite a config is to parse it, change the map and encode it again. That loses
comments (VS Code uses JSONC), key order and indentation, and it rewrites a multi-megabyte
`~/.claude.json` that holds a lot of state besides MCP servers. It would also break the acceptance
rule that uninstall restores the file byte for byte, if the app ever read the re-encoded file.

Instead `jsonscan.go` parses the file and records the byte offsets of every value. install replaces
only the `command` and `args` values (`args` is inserted after `command` if missing). Every other
byte stays identical. The diff hunks come straight from the edits, so no general diff algorithm is needed.

## Restore or unwrap

Claude Code rewrites `~/.claude.json` all the time. Restoring an old backup over it would silently
lose the app's recent changes. So uninstall checks the file's sha256 against the hash recorded after install:

- **restore**: it matches, so the backup is written back byte for byte.
- **unwrap**: it differs, so only the proxy wrapping is removed in place, using
  `config.UnwrapProxyCommand`.
- **gone**: the file was deleted; nothing to do.

Multiple installs are undone newest first, chained in memory, and the file is written once.

## Consequences

- Safe writes need a sha256 re-check just before the rename ("changed while mcpgraph was working on it").
- The manifest stores hashes and offsets only; backups hold the original bytes.
- The parser must be strict about depth (512) and accept comments and trailing commas.
- Locating entries mirrors the config collector's shapes; a change there must be mirrored in `locate.go`.

Related: [component](../components/2026-10-18-install.md), [milestone](../milestones/2026-10-18-v2-m8-install.md).
