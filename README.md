# beads-backend-doltlite

Prototype external backend plugin for Beads using DoltLite as the storage
engine.

This repo makes the plugin seam concrete while upstream Beads maintainers
decide how backend plugins should work. The current prototype is functional: it
owns the DoltLite storage implementation behind a process protocol and supports
Beads issue/config/ready-work flows.

The module path is a normal external repo path. The plugin keeps a local copy of
the v1alpha1 protocol types and the DoltLite-backed storage implementation so it
does not depend on fork-only Beads internals.

The matching Beads core PR is:

- https://github.com/gastownhall/beads/pull/4561

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
./scripts/build.sh
./bin/bd-backend-doltlite capabilities
```

`serve` speaks newline-delimited JSON request/response messages over stdio. It
prints an initial protocol hello response, then handles requests.

Request tracing can be enabled without logging request params:

```bash
./bin/bd-backend-doltlite --trace=/tmp/bd-backend-doltlite.jsonl serve
```

Each trace line is JSONL with timestamp, pid, backend, request ID, method,
success/error code, and duration. Use `--trace-stderr` for stderr tracing, or
set `BEADS_BACKEND_DOLTLITE_TRACE` to a file path, `stderr`, or `off`.

Example:

```bash
tmp="$(mktemp -d)"
mkdir -p "$tmp/.beads"

{
  printf '%s\n' "{\"id\":\"init\",\"method\":\"init\",\"params\":{\"beads_dir\":\"$tmp/.beads\",\"prefix\":\"bp\"}}"
  printf '%s\n' '{"id":"create","method":"create_issue","params":{"session_id":"<from init>","issue":{"id":"bp-1","title":"Plugin smoke","priority":1}}}'
} | go run -tags "libsqlite3 gms_pure_go" ./cmd/bd-backend-doltlite serve
```

The example above is schematic because later requests need the `session_id`
returned by `init`.

## Core Adapter Smoke

The Beads core branch `feat/backend-plugin-architecture` can launch this plugin
through `.beads/metadata.json`:

```json
{
  "backend": "doltlite",
  "dolt_database": "beads",
  "backend_plugin_command": "/absolute/path/to/bd-backend-doltlite"
}
```

Run a temp-workspace smoke test with a `bd` binary built from that branch:

```bash
BD_BIN=/path/to/bd ./scripts/smoke-core-adapter.sh
```

The smoke initializes DoltLite through the plugin process, writes plugin config
metadata, then runs `bd config`, `bd create`, `bd show`, `bd update`, and
`bd ready` through Beads core's process adapter.

## Implemented Methods

- `capabilities`
- `doctor`
- `init`
- `open`
- `close`
- `set_config`
- `get_config`
- `create_issue`
- `get_issue`
- `search_issues`
- `update_issue`
- `add_label`
- `get_labels`
- `ready_work`
- `commit`

This is not yet a full Beads backend transport. It is enough to prove the
session model and a representative issue lifecycle against the real DoltLite
store.

## Capability Intent

DoltLite is expected to support:

- local embedded storage
- transactions
- raw SQL
- lease/work-queue primitives
- maintenance operations
- versioned commits

It is not expected to support Dolt server remotes or server lifecycle commands.

## Current Limitations

- Uses a local `replace` to the Beads `feat/backend-plugin-architecture`
  workspace until the public backend SDK packages land upstream.
- Does not yet expose dependencies, comments, leases, slots, migrations as
  separate protocol methods, or transaction handles.
- Requires Beads core from `feat/backend-plugin-architecture` for
  `backend_plugin_command` client-adapter support.
- Does not yet run the full Beads conformance suite through the process
  transport.

## Upstream PR Notes

This plugin prototype should help upstream review the DoltLite integration as a
set of backend-contract questions:

- Which interfaces belong in core storage contracts?
- Which features should be optional capabilities?
- How should SQL dialect ownership be expressed?
- How should conformance tests be packaged for plugin authors?
- Which lifecycle operations are core-owned versus backend-owned?
