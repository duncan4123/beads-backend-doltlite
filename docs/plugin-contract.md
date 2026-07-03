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
