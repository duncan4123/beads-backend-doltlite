# beads-backend-doltlite

Prototype external backend plugin for Beads using DoltLite as the storage
engine.

This repo is intentionally small. Its purpose is to make the plugin seam
concrete while upstream Beads maintainers decide how backend plugins should
work.

## Proposed Integration

Core Beads remains responsible for command semantics, validation, hooks,
workflow behavior, and the backend contract. A backend plugin supplies a
provider that can initialize, open, diagnose, and report capabilities for one
storage engine.

For the deepest integration, the plugin should hook at the backend provider
boundary:

```text
Beads config -> backend provider lookup -> provider open/init -> storage store
```

The plugin should not wrap individual `bd` commands.

## Prototype Command

```bash
go run ./cmd/bd-backend-doltlite capabilities
go run ./cmd/bd-backend-doltlite doctor
go run ./cmd/bd-backend-doltlite serve
```

`serve` currently prints a protocol hello message. It is a placeholder for the
eventual stdio JSON-RPC or gRPC transport.

## Capability Intent

DoltLite is expected to support:

- local embedded storage
- transactions
- raw SQL
- lease/work-queue primitives
- maintenance operations
- versioned commits

It is not expected to support Dolt server remotes or server lifecycle commands.

## Upstream PR Notes

This plugin prototype should help upstream review the DoltLite integration as a
set of backend-contract questions:

- Which interfaces belong in core storage contracts?
- Which features should be optional capabilities?
- How should SQL dialect ownership be expressed?
- How should conformance tests be packaged for plugin authors?
- Which lifecycle operations are core-owned versus backend-owned?
