---
type: Decision
title: Deny rules before ask rules in the default policy
description: policies/default.yaml lists deny rules first so an action matching both kinds is denied (rm -rf ~/.ssh).
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/policies/default.yaml
tags: [decision, policy, owner]
timestamp: 2026-10-18T07:25:00Z
---

# Deny rules before ask rules in the default policy

**Decided by the owner on 2026-10-18.**

The policy is evaluated in order, and the first match wins. The spec's table order put
`ask-recursive-delete` before `block-secret-read`, so `rm -rf ~/.ssh` was *asked* instead of
*denied*.

The new order is:

1. `block-destructive-sql`
2. `block-secret-read`
3. `ask-recursive-delete`
4. `ask-git-force-push`
5. `ask-unknown-tool`

A table case pins the result: `rm -rf ~/.ssh` is denied by `block-secret-read`.

General rule for future policies: put stricter verdicts first. Order only decides between rules
that both match, and in that case the safer answer should win.
