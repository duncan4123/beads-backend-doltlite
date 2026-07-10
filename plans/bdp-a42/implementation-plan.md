---
schema: gc.build.plan.v1
workflow:
  id: bdp-wisp-ioz
  formula: build-basic
methodology:
  pack: gascity
  name: build-basic
producer:
  formula: build-basic
  stage: plan
  attempt: 1
status: approved
trace:
  upstream:
    - path: /data/projects/doltlite-gascity/rigs/beads-backend-doltlite-plugin/plans/bdp-a42/requirements.md
      hash: sha256:9d7c448b6e392a671457b94cf545bb557b694af9ff3ba3efac00ae56dff6d2f1
      ids:
        - REQ-001
        - REQ-002
        - REQ-003
        - REQ-004
        - REQ-005
        - REQ-006
        - REQ-007
        - REQ-008
        - REQ-009
        - REQ-010
  coverage:
    - id: REQ-001
      status: covered
    - id: REQ-002
      status: covered
    - id: REQ-003
      status: covered
    - id: REQ-004
      status: covered
    - id: REQ-005
      status: covered
    - id: REQ-006
      status: covered
    - id: REQ-007
      status: covered
    - id: REQ-008
      status: covered
    - id: REQ-009
      status: covered
    - id: REQ-010
      status: covered
---

# Implementation Plan: Support Labels in DoltLite Backend Issue Updates

## Trace Coverage

| ID | Status |
| --- | --- |
| REQ-001 | covered |
| REQ-002 | covered |
| REQ-003 | covered |
| REQ-004 | covered |
| REQ-005 | covered |
| REQ-006 | covered |
| REQ-007 | covered |
| REQ-008 | covered |
| REQ-009 | covered |
| REQ-010 | covered |

## Summary

Teach the existing `update_issue` path to treat `updates.labels` as a relational replacement field instead of an issue-table column. The shared `issueops` transaction will validate and normalize the accepted runtime shapes, update ordinary issue fields and the complete desired label set atomically, route labels to `labels` or `wisp_labels`, and preserve the existing update lifecycle. The provider and both plugin entry points already pass JSON-decoded update maps through `provider.Session.UpdateIssue`, so no protocol shape or command dispatch change is required.

The externally visible behavior will be:

- omitted `labels` leaves the existing label set unchanged;
- present `labels` replaces the complete set, including clearing on an empty array;
- `[]string` and JSON-decoded `[]interface{}` string arrays are accepted;
- empty strings and duplicates are discarded consistently with current set semantics;
- malformed values fail before persistence and mixed updates commit or roll back as one transaction;
- the returned issue and subsequent reads expose the final alphabetically hydrated label set.

## Current System

### Request and provider boundary

`backend/plugin/protocol.go` and `internal/gcbackend/protocol.go` model update parameters as `map[string]interface{}`. Both `cmd/bd-backend-doltlite/main.go` and `cmd/gc-doltlite-fastpath/main.go` decode JSON and delegate `update_issue` to `provider.Session.UpdateIssue`. `internal/provider/provider.go` currently normalizes only `metadata`, calls `Store.UpdateIssue`, optionally creates a Dolt commit, and then hydrates the result with `Store.GetIssue`.

This boundary already preserves the Gas City request shape: a JSON labels array arrives as `[]interface{}`. The failure happens later, so introducing a new protocol field or special-case command handler would duplicate behavior and leave in-process callers inconsistent.

### Shared update transaction

`internal/storage/issueops/update.go` owns row routing, field validation, timestamp and row-lock updates, event creation, and blocked-state recomputation. `IsAllowedUpdateField` intentionally lists physical issue columns. `updateIssueInTx` currently iterates the complete update map, rejects `labels`, and otherwise builds a single `UPDATE issues|wisps` statement. This is the source of `invalid field for update: labels`.

`internal/storage/doltlite/issues.go` wraps that helper in `withConn(..., true)`, so the row update and any additional SQL performed by `issueops` can share one SQL transaction. `internal/storage/doltlite/transaction.go` exposes the same helper for caller-managed transactions, but its dirty-table tracker currently marks only `issues` and `events` for an update.

### Existing label behavior and hydration

`internal/storage/issueops/labels.go` already routes reads and standalone add/remove operations through `WispTableRouting`, and `GetIssueInTx` in `internal/storage/issueops/get_issue.go` hydrates labels from `labels` or `wisp_labels` in the same transaction. The domain-layer `SetLabels` behavior in `internal/storage/domain/label.go` establishes replacement semantics by diffing the desired set, ignoring empty strings, and avoiding duplicates, but it is not part of the plugin-backed storage update transaction.

Standalone `AddLabelInTx` and `RemoveLabelInTx` each emit label-specific events. Reusing them for a replacement inside `update_issue` would emit multiple events and obscure the operation as one logical update. The update path should therefore use a transaction-local, no-event set-replacement helper and retain the single full update event already produced by `updateIssueInTx`.

## Proposed Implementation

### 1. Parse relational update fields without mutating the caller map

Update `internal/storage/issueops/update.go` to recognize `labels` before building SQL clauses.

- Add a focused parser, colocated with label operations or update handling, that distinguishes absence from a present empty collection and returns a normalized `[]string`.
- Accept `[]string` and `[]interface{}` where every element is a string. Reject scalar, object, nil, and mixed-element inputs with an error that names `labels` and the invalid shape or element index.
- Normalize by dropping empty strings and deduplicating labels. Preserve labels exactly otherwise; do not trim or case-fold because the existing `SetLabels` contract does neither.
- Build a shallow `columnUpdates` map excluding `labels` instead of deleting from the caller-provided map. Use `columnUpdates` for issue-column validation, SQL generation, status transitions, and timestamp helpers. Keep the original update map for the full update event so the event records the client-visible logical request, including labels.
- Validate the entire labels payload before executing row SQL. Unknown fields remain rejected by `IsAllowedUpdateField`; `labels` remains outside that row-column allowlist so it can never become a quoted SQL column accidentally.

This keeps protocol normalization generic while making both typed storage callers and JSON-decoded plugin callers obey the same contract. It covers REQ-001, REQ-002, REQ-005, and the input portion of REQ-009.

### 2. Replace labels inside the existing update transaction

Add a transaction-local helper in `internal/storage/issueops/labels.go`, for example `SetLabelsInTx`, that accepts the already selected label table, issue ID, and normalized desired labels.

- Read the current set using `GetLabelsInTx`.
- Compute additions and removals as sets, using deterministic ordering when issuing SQL so tests and traces remain stable.
- Delete labels no longer desired and insert new labels with the repository's existing parameterized SQL style. Ignore empty and duplicate desired entries defensively even though the update parser already normalizes them.
- Do not create label-added or label-removed events in this helper. `updateIssueInTx` will retain one `EventUpdated`, `EventStatusChanged`, `EventClosed`, or `EventReopened` event containing the original update payload.

In `updateIssueInTx`, obtain `labelTable` together with the existing issue and event tables from `WispTableRouting`. Run the ordinary row update first, then call `SetLabelsInTx` when labels were present, and only then record the full update event and perform derived-state recomputation. All operations use the same `DBTX`; any label SQL or event failure therefore causes `withConn` or the enclosing `RunInTransaction` to roll back the row change and label change together.

Labels-only updates should still execute the standard issue-row update of `updated_at` and `row_lock` and should emit one update event. This preserves lease conflict detection, actor attribution, hooks, and the semantics that `update_issue` is a real issue mutation. It covers REQ-002, REQ-003, REQ-004, REQ-006, and REQ-007.

### 3. Preserve commit, hook, and dirty-table behavior

No change is needed in `internal/provider/provider.go`, `internal/storage/hook_decorator.go`, or either command dispatcher beyond tests: `Session.UpdateIssue` will continue to normalize metadata, invoke one store update, optionally call `Commit`, and return a freshly hydrated issue. The hook decorator will continue firing `on_update` once after a successful store call and never after an error.

Update `embeddedTransaction.UpdateIssue` in `internal/storage/doltlite/transaction.go` so the dirty-table tracker accounts for every possible routed table touched by the shared helper. Mark `issues`, `wisps`, `events`, `wisp_events`, `labels`, and `wisp_labels`, matching the conservative pattern already used by `CreateIssue`. This ensures an optional Dolt version commit includes relational label mutations for either partition. If desired during implementation, the `UpdateResult.IsWisp` value may be used to narrow marks after success, but correctness takes priority over minimizing the dirty set.

Direct `DoltliteStore.UpdateIssue` continues to rely on embedded DoltLite transaction commit behavior. Existing standalone `AddLabel`, `RemoveLabel`, and domain `SetLabels` APIs remain unchanged. This covers the lifecycle and commit portions of REQ-004 and REQ-007.

### 4. Add layered regression coverage

Extend `internal/storage/conformance/conformance.go` with update-label cases that every storage implementation must satisfy:

- replace an existing set, clear it with an empty array, and omit labels while updating another field;
- accept both `[]string` and `[]interface{}` string arrays;
- normalize duplicate and empty labels;
- reject a non-array value and a non-string array element without changing ordinary fields or labels;
- combine a title or metadata update with label replacement and verify the hydrated result.

Add focused embedded DoltLite tests under `internal/storage/doltlite` for implementation-specific guarantees:

- create both a durable issue and a no-history/ephemeral wisp, update labels, and verify the correct physical table plus hydrated result;
- install a test-only SQL trigger or equivalent deterministic constraint that rejects one label, submit a mixed title-and-label update, and verify both the row field and original labels remain unchanged after rollback;
- when exercising `RunInTransaction` with a commit message, verify label-bearing updates are included in the resulting committed state.

Add a provider contract test in `internal/provider/provider_test.go` that initializes a real session, constructs the update map by JSON unmarshalling a Gas City-style request fragment, calls `Session.UpdateIssue`, and checks both the returned issue and a later `GetIssue`. Include labels alongside representative cursor/outcome metadata or ordinary tracking fields. This directly proves the generic `[]interface{}` path that both stdio dispatchers use without duplicating command-level tests.

These tests cover REQ-008 and REQ-009 while preventing regressions in REQ-001 through REQ-007.

## Decomposition-Ready Work Items

The implementation should be decomposed into the following beads. Each bead has a narrow file boundary, explicit requirement coverage, and a focused proof command.

| Work item | Scope and files | Requirements / acceptance criteria | Depends on | Focused proof |
| --- | --- | --- | --- | --- |
| WI-1: transactional label replacement | Add label-shape parsing and no-event set replacement in `internal/storage/issueops/update.go` and `internal/storage/issueops/labels.go`; integrate it into the existing update transaction without mutating the caller's label entry | REQ-001, REQ-002, REQ-005, REQ-006; typed and JSON-decoded inputs, omission, replacement, clear, normalization, malformed input, and rollback | none | `go test ./internal/storage/issueops -run 'Test.*(Update.*Labels|SetLabels)' -count=1` |
| WI-2: routed DoltLite transaction and commit tracking | Update `internal/storage/doltlite/transaction.go` and DoltLite-specific tests for regular/wisp routing, dirty tables, atomic failure, and committed state | REQ-004, REQ-006, REQ-007 | WI-1 | `go test ./internal/storage/doltlite -run 'Test.*Update.*Labels' -count=1` |
| WI-3: shared and public-boundary contracts | Extend `internal/storage/conformance/conformance.go` and `internal/provider/provider_test.go` with replacement, malformed-input, hydration, and JSON-decoded Gas City request cases | REQ-003, REQ-008, REQ-009 | WI-1 | `go test ./internal/storage/conformance ./internal/provider -run 'Test.*Update.*Labels' -count=1` |
| WI-4: repository and process-level proof | Run repository gates, build both process entry points with the documented DoltLite linkage, run the focused Beads plugin profile, then run the full profile before publish | REQ-010 and final evidence for REQ-001 through REQ-009 | WI-2, WI-3 | commands in Verification below |

WI-1 owns the semantic decision and shared transaction behavior. WI-2 and WI-3 can run in parallel after WI-1. WI-4 is verification-only and must not absorb implementation fixes; failures route back to the owning work item.

## Risks and Rollback

- `internal/storage/issueops/update.go` is a shared mutation path for regular issues and active wisps. A routing or event-order regression can affect every update caller, so WI-1 must keep `labels` outside `IsAllowedUpdateField`, preserve the original update payload for the single logical event, and run existing non-label update tests.
- `internal/storage/issueops/labels.go` writes relational tables selected through `WispTableRouting`. Tests must assert both positive placement and absence from the wrong table so a wisp/durable partition mistake cannot pass on hydration alone.
- `internal/storage/doltlite/transaction.go` controls the dirty-table set used by optional Dolt commits. Conservative marking is acceptable; missing `labels` or `wisp_labels` is not. The commit test must read from the committed state, not only the live SQL transaction.
- The public plugin and Gas City protocol maps do not change. Accepting `labels` is backward-compatible at the request boundary, while malformed values remain errors through the existing response classification. No schema migration or data backfill is required.
- Rollback is a source revert of the issueops and dirty-table changes. Because there is no schema migration, rollback does not require data conversion. Before reverting after release, confirm no caller has begun relying on labels-in-update; otherwise the old binary will restore the original `invalid field for update: labels` failure.

## Non-Goals

- Do not add labels as a physical column or change the `labels`/`wisp_labels` schemas.
- Do not change the public protocol structs, add merge-mode semantics, or redesign Gas City cursor/outcome tracking.
- Do not alter standalone `add_label`, `remove_label`, or domain-layer set-label APIs and their existing label-specific events.
- Do not make label order a public contract. Storage hydration may remain alphabetically sorted, while assertions compare normalized membership unless testing that existing read behavior.
- Do not trim, case-fold, or otherwise redefine valid non-empty label strings.
- Do not refactor unrelated update fields, hook infrastructure, version-control operations, or wisp promotion/demotion behavior.

## Verification

### Focused tests

Run from `/data/projects/doltlite-gascity/rigs/beads-backend-doltlite-plugin` with the repository's normal CGO/DoltLite environment:

```bash
go test ./internal/storage/issueops ./internal/storage/conformance
go test ./internal/storage/doltlite -run 'Test.*Update.*Labels' -count=1
go test ./internal/provider -run 'Test.*Update.*Labels' -count=1
```

The exact test names may follow local naming conventions, but the focused selection must execute typed and JSON-decoded label shapes, regular and wisp routing, malformed inputs, hydration, and rollback.

### Repository gates

```bash
go test ./...
go vet ./...
OUT=/tmp/bd-backend-doltlite-labels ./scripts/build.sh
CGO_ENABLED=1 \
CGO_LDFLAGS="-L/data/projects/doltlite-gascity/doltlite-work/build -Wl,-rpath,/data/projects/doltlite-gascity/doltlite-work/build -ldoltlite" \
go build -tags 'libsqlite3 gms_pure_go' -o /tmp/gc-doltlite-fastpath-labels ./cmd/gc-doltlite-fastpath
```

If the DoltLite library is not at the repository default, set `DOLTLITE_LIB` and use the same directory in `CGO_LDFLAGS`.

Run the focused public-process conformance proof from `/data/projects/doltlite-gascity/beads-doltlite`:

```bash
BEADS_DOLTLITE_PLUGIN_COMMAND=/tmp/bd-backend-doltlite-labels \
BEADS_DOLTLITE_PLUGIN_ARGS=serve \
BEADS_CONFORMANCE_PROFILES=doltlite-plugin \
CGO_ENABLED=1 \
go test -tags 'gms_pure_go e2e' ./test/conformance/ \
  -run 'TestConformanceE2E/promote|TestExportImportRoundTripE2E/doltlite-plugin' \
  -timeout 20m -count=1
```

Before publish, repeat without `-run` and with `-timeout 90m` to execute the full `doltlite-plugin` profile documented in `conformance/README.md`.

### Acceptance evidence

- Capture a passing provider test whose request map is produced by `encoding/json` and whose response contains the final labels.
- Capture database assertions showing durable labels only in `labels` and wisp labels only in `wisp_labels`.
- Capture the forced-failure test showing no partial title/metadata or label changes.
- Confirm existing non-label update, label add/remove, hook, and optional commit tests remain green.
- Re-run the Gas City tracking reproduction, if available in the implementation environment, and confirm `storage_error: invalid field for update: labels` no longer occurs.

No unresolved design ambiguity blocks decomposition. The plan intentionally selects one full update event rather than per-label events because it preserves the existing `update_issue` hook/event lifecycle and atomic logical-operation contract.
