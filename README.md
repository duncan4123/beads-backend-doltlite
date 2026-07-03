# beads-backend-doltlite

Prototype external backend plugin for Beads using DoltLite as the storage
engine.

This repo is intentionally small. Its purpose is to make the plugin seam
concrete while upstream Beads maintainers decide how backend plugins should
work. The current prototype is functional: it opens the existing Beads
DoltLite storage implementation behind a process protocol and supports a
minimal issue workflow.

The module path is temporarily under `github.com/steveyegge/beads/...` so this
prototype can import the current Beads `internal/storage/doltlite` package.
That dependency is the exact pressure point a future public backend SDK should
remove.

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
DOLTLITE_LIB=/path/to/doltlite/build \
CGO_LDFLAGS="-L${DOLTLITE_LIB} -Wl,-rpath,${DOLTLITE_LIB} -ldoltlite" \
go run -tags "libsqlite3 gms_pure_go" ./cmd/bd-backend-doltlite capabilities
```

`serve` speaks newline-delimited JSON request/response messages over stdio. It
prints an initial protocol hello response, then handles requests.

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

- Imports Beads internals through a temporary module path and local `replace`.
- Does not yet expose dependencies, comments, leases, slots, migrations as
  separate protocol methods, or transaction handles.
- Does not yet have a Beads-core client shim.
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
