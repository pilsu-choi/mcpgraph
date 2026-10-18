---
type: Lesson
title: redact.Text is wrong for raw JSON lines
description: Redacting a config diff line by line with redact.Text broke the output; redact only inside decoded string literals and re-encode them.
tags: [lesson, go, json, redaction, install]
timestamp: 2026-10-18T07:55:00Z
---

# `redact.Text` is wrong for raw JSON lines

The install diff prints lines from config files, which hold secrets (hard rule 2). The first version
ran each raw line through `redact.Text`. That function expects prose or a command line, and on JSON it:

- trimmed the indentation;
- treated quotes and commas as part of words, so a masked value came out as `****KE",` with the
  closing quote and comma inside the mask's last four characters;
- URL-escaped a trailing `"]` into `%22%5D`.

**Fix.** `redactJSONLine` in `internal/install/diff.go` finds the string literals on a line, decodes
them, redacts, and encodes them again. A `"KEY": "value"` pair goes through `redact.Value(key, value)`.
Other strings on the line are judged together with `redact.Args`, so `"--api-key", "VALUE"` is caught.
Strings that do not change keep their exact bytes. A line that cannot be read this way falls back to
`redact.Text`. `TestNoSecretInOutputOrManifest` checks the diffs and the manifest.

**Takeaway.** A redactor works on the data model it was written for. For structured text, decode the
values, redact them, and put the punctuation back untouched. See also
[redaction must be idempotent](redaction-idempotent.md).
