---
type: Runbook
title: GitHub repository
description: Where the code lives and how it was published.
tags: [runbook]
timestamp: 2026-10-18T06:45:00Z
---

# GitHub repository

- URL: https://github.com/pilsu-choi/mcpgraph (public).
- Working branch: `dev`. All milestone commits and the docs commit were pushed there. `main` was
  not created; open a PR from `dev` when ready.
- Created through the GitHub REST API (`POST /user/repos`) with the token already stored in
  `~/.git-credentials`, because `gh` is not installed.
- `.goreleaser.yaml` still has `release: disable: true`. Remove it to publish releases from tags.
