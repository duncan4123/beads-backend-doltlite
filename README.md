# beads-backend-doltlite

External Beads backend plugin that stores Beads data in DoltLite.

This repository is the DoltLite proof implementation for the Beads backend
plugin process architecture proposed in:

- https://github.com/gastownhall/beads/pull/4561

The plugin runs as a separate process and speaks the Beads backend protocol over
stdio. Beads core remains responsible for CLI behavior, validation, hooks,
workflow semantics, and choosing which backend provider to open. This repository
owns the DoltLite-specific implementation behind that backend boundary.

## Current Shape

The plugin is intentionally self-contained:

- It has a normal external Go module path:
  `github.com/duncan4123/beads-backend-doltlite`.
- It keeps a local copy of the current v1alpha1 backend protocol types while
  the upstream SDK boundary is still under review.
- It carries the DoltLite-backed storage implementation needed by the plugin.
- It does not require fork-only Beads internals at build time.

At runtime, a Beads CLI built with the plugin-process adapter launches
`bd-backend-doltlite`, opens a backend session, and forwards storage operations
over the protocol.

```text
bd command
  -> Beads backend provider registry
  -> plugin-process storage adapter
  -> bd-backend-doltlite over stdio
  -> DoltLite-backed storage
```

## Build

The plugin links against a DoltLite shared library. Point `DOLTLITE_LIB` at the
directory containing that library:

```bash
DOLTLITE_LIB=/path/to/doltlite/build ./scripts/build.sh
```

The default output is:

```text
./bin/bd-backend-doltlite
```

You can override the output path:

```bash
DOLTLITE_LIB=/path/to/doltlite/build OUT=/tmp/bd-backend-doltlite ./scripts/build.sh
```

The build script uses these defaults unless overridden:

```text
CGO_ENABLED=1
GO_TAGS="libsqlite3 gms_pure_go"
```

## Commands

The plugin binary supports:

```bash
bd-backend-doltlite capabilities
bd-backend-doltlite doctor
bd-backend-doltlite serve
```

`serve` is the mode used by Beads core. It prints an initial protocol hello
response, then reads newline-delimited JSON requests from stdin and writes
newline-delimited JSON responses to stdout.

## Beads Configuration

A Beads workspace can opt into this backend by writing plugin metadata in
`.beads/metadata.json`:

```json
{
  "backend": "doltlite",
  "dolt_database": "beads",
  "backend_plugin_command": "/absolute/path/to/bd-backend-doltlite",
  "backend_plugin_args": ["serve"]
}
```

Tracing can be enabled by passing trace arguments before `serve`:

```json
{
  "backend": "doltlite",
  "dolt_database": "beads",
  "backend_plugin_command": "/absolute/path/to/bd-backend-doltlite",
  "backend_plugin_args": [
    "--trace",
    "/tmp/bd-backend-doltlite.jsonl",
    "serve"
  ]
}
```

Trace lines are JSONL records containing timestamp, process ID, backend, request
ID, method, success/error code, and duration. Request params are not logged.

Tracing can also be controlled with:

```bash
bd-backend-doltlite --trace=/tmp/bd-backend-doltlite.jsonl serve
bd-backend-doltlite --trace-stderr serve
BEADS_BACKEND_DOLTLITE_TRACE=/tmp/bd-backend-doltlite.jsonl bd-backend-doltlite serve
```

Use `BEADS_BACKEND_DOLTLITE_TRACE=off` to disable environment-driven tracing.

## Implemented Surface

The plugin implements the current protocol surface needed for normal Beads
storage use, including:

- initialization, open, close, diagnostics, and capabilities
- config and metadata operations
- issue create, read, update, close, reopen, delete, and batch create
- ready-work queries and issue counts
- labels, comments, dependencies, blocking queries, and dependency trees
- lease claim, heartbeat, and reclaim operations
- transactions and transaction-scoped issue/config/comment/dependency methods
- merge slots and generic slot operations
- compaction, flattening, and maintenance entry points
- versioning, branch, history, diff, merge, remote, backup, federation, and sync
  methods exposed by the copied DoltLite storage implementation

The capability response currently advertises embedded storage, transactions,
raw SQL support at the backend level, leases, maintenance, versioning, branching,
and concurrent writers. It does not advertise Dolt server remotes.

## Smoke Test

The Beads core branch from PR #4561 can launch this plugin through
`backend_plugin_command`. After building a compatible `bd` binary and this
plugin, run:

```bash
BD_BIN=/path/to/bd ./scripts/smoke-core-adapter.sh
```

The smoke test initializes a temporary DoltLite-backed Beads workspace through
the plugin process, writes plugin config metadata, then exercises representative
`bd` commands through Beads core's process adapter.

## Conformance

The `conformance/` directory documents the intended upstream shape for backend
conformance tests. The desired long-term test harness should open the plugin
through the same process adapter used by Beads core, then run shared backend
behavior tests against that public contract.

## Design Questions for Upstream

This repository is meant to make the plugin boundary reviewable. The main
questions for Beads maintainers are:

- Which protocol types and helpers should become a stable public backend SDK?
- Which backend features should be required versus optional capabilities?
- How should raw SQL and SQL dialect ownership be represented?
- How should conformance tests be packaged for plugin authors?
- Which lifecycle operations belong in Beads core versus backend plugins?
