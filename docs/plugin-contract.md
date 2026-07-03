# Backend Plugin Contract Sketch

## Core-Owned Behavior

- CLI command semantics
- validation and user-facing errors
- hooks and workflow behavior
- conformance contract
- capability-gated command routing

## Backend-Owned Behavior

- storage open/init
- physical schema and migrations
- SQL dialect
- engine diagnostics
- maintenance operations
- optional versioning implementation

## Implemented Prototype Surface

```text
capabilities
doctor
init
open
close
set_config
get_config
create_issue
get_issue
search_issues
update_issue
add_label
get_labels
ready_work
commit
```

The prototype uses newline-delimited JSON over stdio. Each request has a
method, optional ID, and params object. Each response echoes the ID and contains
either `result` or `error`.

## Beads Core Wiring

Beads core and the plugin communicate using the v1alpha1 protocol in
`backend/plugin`. This repo keeps a local copy of those types so the plugin can
remain self-contained while the upstream SDK boundary stabilizes:

```go
import backendplugin "github.com/duncan4123/beads-backend-doltlite/backend/plugin"
```

The DoltLite storage implementation also lives in this repo:

```go
import backenddoltlite "github.com/duncan4123/beads-backend-doltlite/internal/storage/doltlite"
```

Beads core can launch this process when `.beads/metadata.json` contains:

```json
{
  "backend": "doltlite",
  "backend_plugin_command": "/absolute/path/to/bd-backend-doltlite"
}
```

The core adapter invokes the command with `serve` by default unless
`backend_plugin_args` is provided. The plugin must write one hello response
before processing requests:

```json
{
  "ok": true,
  "result": {
    "protocol": "beads.backend.v1alpha1",
    "backend": "doltlite",
    "capabilities": {}
  }
}
```

The current adapter forwards the implemented issue/config/ready/label/commit
methods. Full `storage.DoltStorage` coverage still needs protocol methods for
dependencies, comments, events, slots, metadata, sync/remotes, annotations,
history, and transaction handles.

## Transaction Shape

The hardest method is transaction execution because Go callbacks do not cross a
process boundary. The process plugin should use transaction/session handles
instead of callback-based APIs:

```text
begin_transaction -> tx_id
tx_create_issue
tx_add_label
commit_transaction
rollback_transaction
```

This keeps core command semantics in Beads while avoiding Go callback leakage
across the plugin transport.
