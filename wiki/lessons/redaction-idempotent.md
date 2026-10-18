---
type: Lesson
title: Redacting twice masked the mask
description: The audit log re-redacts summaries; without idempotency ****dXyz became ****, so the same decision looked different in the engine and the log.
tags: [lesson, redact, audit, m6]
timestamp: 2026-10-18T07:25:00Z
---

# Redacting twice masked the mask

**Context.** `audit.Log.Append` redacts the summary again as defence in depth, even though the
policy engine already redacted it.

**Bug.** `PGPASSWORD=FakePassw0rdXyz` became `PGPASSWORD=****dXyz` in the engine. On the second
pass, `****dXyz` (8 characters, under the 12-character reveal limit) was treated as a short secret
under a secret-named key, and became `****`. Nothing leaked, but the same decision read
differently in the engine and in the audit log, and three code paths had the same flaw: `Detect`,
`maskUnlessSafe` (the value after `--api-key`) and the word after `Bearer`.

**Fix.** A value that starts with `****` counts as already masked in all three paths.
`TestTextIsIdempotent` checks that `Text(Text(x)) == Text(x)`.

**Lesson.** When a sanitizer runs at several layers, make it idempotent and test that it is. The
trade-off is that a real secret starting with `****` would not be masked, which no real token
format does.
