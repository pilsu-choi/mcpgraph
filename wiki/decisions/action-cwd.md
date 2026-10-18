---
type: Decision
title: Action carries the working directory
description: policy.Action has a Cwd field that the AGENTS.md sketch lacks, because ask-recursive-delete must tell inside-the-project from outside.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/policy/engine.go
tags: [decision, policy, v2, m5]
timestamp: 2026-10-18T07:15:00Z
---

# `Action` carries the working directory

AGENTS.md §7 sketches `Action{Source, Agent, Tool, Args, Text}`. The rule `ask-recursive-delete`
must decide whether an `rm -r` target is *outside the current project directory*, which needs the
project directory. The engine is shared and long-lived (the proxy keeps one per process), while the
directory changes per action (the hook payload has its own `cwd`). So the directory belongs to the
action, not to the engine.

**Decision.** Add `Cwd string` to `policy.Action`. The other fields are unchanged.

**When it is unknown** (empty), relative paths stay relative, and every recursive delete counts as
outside, which means *ask*. Missing information makes the engine stricter, never looser.
