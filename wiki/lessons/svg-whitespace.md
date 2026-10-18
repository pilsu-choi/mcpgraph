---
type: Lesson
title: SVG captures lost column alignment
description: Chromium collapsed runs of spaces inside SVG text despite xml:space=preserve.
tags: [lesson]
timestamp: 2026-10-18T06:50:00Z
---

# SVG captures lost column alignment

**Symptom:** after rasterising `docs/images/01-scan-fixtures.svg`, the tabwriter columns were
collapsed into single spaces.

**Tried:** `xml:space="preserve"` and `style="white-space:pre"`. Neither was enough when the SVG
was inlined in HTML.

**Fix:** `termsvg` emits each space as `&#160;` (a non-breaking space), which no renderer collapses.

**Takeaway:** check generated images by actually rendering them.
