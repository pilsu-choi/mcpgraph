---
type: Policy Rule Set
title: Runtime policy rules (default.yaml)
description: The five rules shipped in policies/default.yaml, what each matches, and the tricky cases its tests cover.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/policies/default.yaml
tags: [rule, policy, runtime, v2, m5]
timestamp: 2026-10-18T07:15:00Z
---

# Runtime policy rules (`policies/default.yaml`)

Rules are evaluated per action, in this order, and the first match wins. The order follows the
AGENTS.md table.

| Rule | Matcher | Verdict | Tricky cases covered |
| --- | --- | --- | --- |
| `block-destructive-sql` | `sql: [drop_table, drop_database, truncate, delete_without_where]` | deny | Mixed case; `/**/` and `--` comments between keywords; `WHERE 1=1`, `WHERE TRUE`, `... OR 'a'='a'`, `(1=1)`; WHERE only in a comment; WHERE after `;` or `&&`; nested `bash -c`; MySQL `DELETE t FROM t`. Allowed: `IS NULL`, `1=1 AND id=7`, `DROP INDEX`, shell `truncate -s 0`, prose "delete from" |
| `ask-recursive-delete` | `recursive_delete: outside_project` | ask | `/`, `/*`, `~`, `$HOME/x`, `..`, `.` (the project itself), sudo, `/bin/rm`, `-r -f`, `-R`, `--recursive`, after `--`, `$VAR`, unknown cwd, xargs, `sh -c`. Allowed: `build`, `./x`, `dist/*`, `*.egg-info`, `rm -f`, `echo rm -rf /` |
| `block-secret-read` | `paths: [~/.ssh/**, ~/.aws/credentials, **/.env, **/.env.*]` except `.env.example/.sample/.template` | deny | `~`, `$HOME`, absolute paths, `<.env` redirection, `--env-file=.env`, Read tool `file_path`, MCP args. Allowed: `.envrc`, `~/.aws/config`, `/etc/ssh` |
| `ask-git-force-push` | `command:` regex per simple command | ask | `-f`, `-fu`, `--force-with-lease[=...]`, `+main`, `git -C dir push`, chained. Allowed: `--follow-tags`, branch `feature-f`, `-f` belonging to a later `rm`, `git fetch --force` |
| `ask-unknown-tool` | `unknown_tool: true`, `when: {sources: [proxy]}` | ask | Unknown tool or server → ask; hook actions are never "unknown tools"; without an inventory (`KnownTool == nil`) it never fires |

Each test runs through the shipped YAML, not a test copy: `internal/policy/default_test.go`. The
run log lists every case by name ([`docs/logs/12-policy-tests.log`](https://github.com/pilsu-choi/mcpgraph/blob/dev/docs/logs/12-policy-tests.log)).

**Order note.** Because `ask-recursive-delete` comes before `block-secret-read`,
`rm -rf ~/.ssh` is *ask*, not *deny*. Moving the deny rules first would be stricter. That is an
owner decision, so the order stays as in the spec for now.

Posture rules (R1–R7) are in the [rules index](index.md).
