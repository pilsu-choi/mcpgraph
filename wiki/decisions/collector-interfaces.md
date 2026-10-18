---
type: Design Decision
title: Collectors and parsers are small interfaces
description: Collector{Name, Collect} and parser{App, Parse} keep adding a source or app local to one file.
tags: [decision, design, extensibility]
timestamp: 2026-10-18T06:10:36Z
---

# Collectors and parsers are small interfaces

## Decision
- `collector.Collector`: `Name() string`, `Collect(ctx, chan<- Event) error`.
- `config.parser` (unexported): `App() string`, `Parse([]byte) ([]serverConfig, error)`.

## Why
- To add an app, you add one parser file, one line in `paths.go` and one fixture
  ([runbook](../runbooks/add-an-app.md)).
- Go interfaces are satisfied implicitly, so parsers are plain empty structs with two methods.
- The process collector takes a `Lister` interface, so tests use a fake process table and never
  touch the real OS.
