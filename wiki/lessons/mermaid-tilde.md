---
type: Lesson
title: Two tildes in a Mermaid label become strikethrough
description: Mermaid parsed '~/.ssh ... ~/.aws' as markdown strikethrough and rendered an error box.
tags: [lesson]
timestamp: 2026-10-18T06:50:00Z
---

# Two tildes in a Mermaid label become strikethrough

**Symptom:** a node labelled `~/.ssh/id_*<br/>~/.aws/credentials` parsed fine but rendered as
"Unsupported markdown: del".

**Cause:** Mermaid labels support markdown, and `~...~` is strikethrough.

**Fix:** reword the label ("SSH private keys / AWS credentials"). The parser alone did not catch
this; rendering in headless Chromium did.

**Takeaway:** a parse check is not a render check.
