# Hook payloads

Inputs for `mcpgraph hook <agent>` tests (`internal/hook`, `cmd/mcpgraph/hook_test.go`).
Tests run with `--home /home/testuser`, so `/home/testuser/...` paths stand for the user's home.

| File | Origin |
| --- | --- |
| `claude-code/captured-bash-echo.json` | **Captured** from a real Claude Code 2.1.302 run (see the m9 milestone page). |
| every other `claude-code/*.json` | **Hand-written** from the field list in the Claude Code hooks docs. |
| every `codex/*.json` | **Hand-written** from the Codex PreToolUse input schema (`codex-rs/hooks/schema/generated/pre-tool-use.command.input.schema.json`); never captured from a real Codex run unless the m9 milestone page says so. |
| `*/malformed.json` | Deliberately broken JSON (truncated); contains a fake secret to prove errors do not echo input. |

Secrets in these files are fake (`internal/testutil.FakeSecrets`).
