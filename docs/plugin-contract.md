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

## First Protocol Surface

```text
capabilities
doctor
init
open/session
close/session
```

Storage methods can be added after the session model is settled. The hardest
method is transaction execution because Go callbacks do not cross a process
boundary. The process plugin should use transaction/session handles instead of
callback-based APIs.
