# beads-backend-doltlite

External Beads backend plugin that stores Beads data in DoltLite, with a small
Gas City companion surface for DoltLite-specific runtime layout decisions.

This repository is the DoltLite proof implementation for the Beads backend
plugin process architecture proposed in:

- https://github.com/gastownhall/beads/pull/4561

It is also the natural place for Gas City integration code that is specific to
DoltLite. Gas City can continue to use its direct DoltLite fast paths where they
matter, while Beads storage goes through the backend plugin and shared
configuration remains inspectable by this repository.

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

Gas City companion tools live beside the backend process:

```text
gc / Gas City init
  -> .beads/metadata.json
  -> gc-doltlite layout/health
  -> table placement contract for DoltLite + attached SQLite ops DB

gc runtime store access
  -> Gas City backend capability adapter
  -> Gas City backend plugin protocol over stdio
  -> plugin-owned DoltLite store
```

## Build

The plugin links against a DoltLite native library. Point `DOLTLITE_LIB` at the
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

### Zig Native Build

The Zig build is the reproducible native-link path for plugin development and
release experiments. It downloads the DoltLite release recorded in
`build/doltlite.lock`, verifies the archive checksum, then uses `zig cc` as the
CGO compiler and linker while building all three Go binaries. It does not build
DoltLite itself. The currently locked Zig version is 0.16.0:

```bash
zig build
```

Outputs are written under `zig-out/`:

```text
zig-out/bin/bd-backend-doltlite
zig-out/bin/gc-doltlite-fastpath
zig-out/bin/gc-doltlite
zig-out/build-provenance.json
```

To use an already installed or downloaded library, provide its directory:

```bash
zig build -Ddoltlite-lib=/path/to/doltlite-lib
```

Run the focused linked regression suite with the same native library:

```bash
zig build test
```

The default build rejects an archive with a different checksum and rejects a
Zig version other than the version in the lock file. Updating DoltLite is
therefore an explicit release URL and checksum change rather than accidental
reuse of an unknown native archive.

## Conformance Testing

The release gate for this plugin lives in the Beads repository. Build this
plugin, point `BEADS_DOLTLITE_PLUGIN_COMMAND` at the resulting binary, and run
the `doltlite-plugin` end-to-end conformance profile from Beads:

```bash
OUT=/tmp/bd-backend-doltlite-conformance ./scripts/build.sh

cd /data/projects/doltlite-gascity/beads-doltlite
BEADS_DOLTLITE_PLUGIN_COMMAND=/tmp/bd-backend-doltlite-conformance \
BEADS_DOLTLITE_PLUGIN_ARGS=serve \
BEADS_CONFORMANCE_PROFILES=doltlite-plugin \
CGO_ENABLED=1 \
go test -tags 'gms_pure_go e2e' ./test/conformance/ -timeout 90m -count=1
```

The test builds a real `bd`, initializes both the embedded Dolt reference and
this plugin backend, runs the same CLI scenarios, and requires normalized output
to match. A full run currently takes about 15 minutes. See
`conformance/README.md` for the focused iteration command.

## Commands

The Beads backend plugin binary supports:

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

## Gas City Companion

The repository also builds a `gc-doltlite` helper. It is a read-only companion
surface for Gas City code that needs DoltLite-specific knowledge without baking
that knowledge directly into Gas City core.

```bash
go build -o ./bin/gc-doltlite ./cmd/gc-doltlite
./bin/gc-doltlite --scope=/path/to/city layout
./bin/gc-doltlite --scope=/path/to/city health
```

`gc-doltlite` reads `<scope>/.beads/metadata.json`, resolves the configured
backend plugin command, finds the `ops` attached SQLite database, and emits the
intended table layout as JSON.

Supported profiles:

- `ledger`: keep all synchronized issue and workflow state in DoltLite; use the
  attached SQLite ops DB only for local caches, diagnostics, and metrics.
- `local-runtime`: keep durable issue/config state in DoltLite and place local
  Gas City workflow/runtime tables in attached SQLite.
- `mirror`: use `local-runtime` table placement plus DoltLite mirror tables for
  summarized runtime state that should travel with remotes.

The default is `ledger`, which preserves the strongest cross-machine semantics.
Gas City can opt into another profile in metadata:

```json
{
  "backend": "doltlite",
  "dolt_database": "gascity",
  "backend_plugin_command": "/absolute/path/to/bd-backend-doltlite",
  "backend_plugin_args": ["serve"],
  "attached_databases": [
    {"alias": "ops", "path": ".gc/ops.sqlite"}
  ],
  "gascity": {
    "doltlite_profile": "ledger"
  }
}
```

This does not yet move tables by itself. It gives Gas City and the plugin a
single contract for what a profile means before table routing is wired into the
runtime paths.

The repository also carries a process-owned Gas City backend implementation:

```bash
go build -o ./bin/gc-doltlite-fastpath ./cmd/gc-doltlite-fastpath
./bin/gc-doltlite-fastpath capabilities
./bin/gc-doltlite-fastpath serve
```

`gc-doltlite-fastpath` is the intended replacement for linking `gc` directly
against `libdoltlite`. In the target shape, the `gc` binary stays unlinked and
uses its backend capability adapter to talk to this long-lived plugin-owned
process over newline-delimited JSON. The plugin process owns all direct DB
access, including reads and writes, and is the only component that needs
`libdoltlite`.

The process speaks `gascity.backend.v1alpha1`, a backend-neutral protocol for
Gas City internals that need direct store access without shelling through `bd`.
The current surface exposes session open/close plus issue get, search,
ready-work, wisp listing, counts, create/update/close/reopen/delete, storage
tier create, conditional release, batched dependency reads, labels,
dependencies, and metadata slot operations.

Gas City has a pure-Go `beads.Store` client adapter for this protocol. The
remaining direct-linked `DoltliteReadStore` code can become a fallback and then
be removed as more backend-specific helper operations move into this protocol.

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

## DoltLite Remote Server Tests

Use the local DoltLite remote server harness to exercise HTTP remotes in the
test suite:

```bash
./scripts/test-doltlite-remotesrv.sh
```

The script auto-detects the DoltLite checkout at
`/data/projects/doltlite-gascity/doltlite`, exports `DOLTLITE_REMOTESRV`, and
links Go tests against the same local `libdoltlite`. Override detection with:

```bash
DOLTLITE_ROOT=/path/to/doltlite ./scripts/test-doltlite-remotesrv.sh
```

By default it runs the focused remote parity test. Pass normal `go test`
arguments to run a different linked test target.

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
