---
type: Lesson
title: Trailing-quote trimming broke a tautology; prose matched DELETE FROM
description: Two SQL classifier bugs found by the table tests: trimming a closing shell quote also ate the quote of 'a'='a', and "delete from" in a git grep looked like SQL.
tags: [lesson, policy, sql, m5]
timestamp: 2026-10-18T07:15:00Z
---

# Trailing-quote trimming broke a tautology; prose matched DELETE FROM

**Context.** The SQL classifier reads whole shell commands such as `psql -c "DELETE FROM users WHERE id = 1"`.
The WHERE condition therefore ends with the closing `"` of the shell quoting, and the first version
trimmed every trailing `"` and `'`.

**Bug 1.** In `DELETE FROM users WHERE id = 5 OR 'a'='a'`, that trim removed the last `'`. The
condition became `'a'='a`, its two sides no longer compared equal, and the tautology was missed.
The statement was allowed. **Fix:** `trimUnbalancedQuote` drops a final quote only when that quote
character occurs an odd number of times.

**Bug 2.** `git log --grep "delete from"` was denied as a DELETE without WHERE. **Fix:** a table
name (a letter, `_`, `"`, a backtick or `[`) must follow `FROM`.

**Lesson.** Both bugs were caught only because the tests use realistic inputs (a whole shell
command, prose that happens to contain SQL words), not just bare SQL. Keep negative cases that
*look* dangerous.
