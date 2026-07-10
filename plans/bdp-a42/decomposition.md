---
schema: gc.build.decomposition.v1
workflow:
  id: bdp-wisp-ioz
  formula: build-basic
methodology:
  pack: gascity
  name: build-basic
producer:
  formula: build-basic
  stage: decompose
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
    - path: /data/projects/doltlite-gascity/rigs/beads-backend-doltlite-plugin/plans/bdp-a42/implementation-plan.md
      hash: sha256:24eeb134bd499f284ddbc73c795af8349a1933169d4225b0fb292dfae81354f3
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

# Decomposition: Support Labels in DoltLite Backend Issue Updates

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

The approved plan is decomposed into four implementation units. The first owns shared issue-update semantics; the second and third can proceed in parallel once those semantics exist; the fourth performs repository and public-process verification after both implementation branches complete. Each unit has a narrow file or verification boundary and traces to explicit requirements.

## Selected Downstream Formulas

- Implementation formula: `implement`
- Per-item formula: `do-work-item`
- Implementation target: `gc.implementation-worker`
- Review formula: `review`
- Review-fix formula: `fix-loop-base`

## Implementation Convoy

- Convoy ID: `bdp-6e6`
- Convoy title: `bdp-a42 label update implementation`
- Work units: `bdp-cu7`, `bdp-wdk`, `bdp-3ht`, `bdp-2vr`
- Dependency flow: `bdp-cu7` blocks both `bdp-wdk` and `bdp-3ht`; both block `bdp-2vr`.

## Work Items

### WI-1 — Support transactional label replacement in issue updates (`bdp-cu7`)

- Plan sections: Proposed Implementation 1-2.
- Files: `internal/storage/issueops/update.go`, `internal/storage/issueops/labels.go`, and focused issueops tests.
- Requirements: REQ-001, REQ-002, REQ-005, REQ-006.
- Deliverable: parse typed and JSON-decoded label collections without mutating the caller map; normalize values; reject malformed shapes before writes; replace the desired label set without per-label events inside the existing transaction; preserve one logical update lifecycle and rollback behavior.
- Verification: `go test ./internal/storage/issueops -run 'Test.*(Update.*Labels|SetLabels)' -count=1`.
- Dependencies: none.

### WI-2 — Verify DoltLite label routing and transaction commit tracking (`bdp-wdk`)

- Plan sections: Proposed Implementation 3 and DoltLite-specific portions of 4.
- Files: `internal/storage/doltlite/transaction.go` and DoltLite-specific tests.
- Requirements: REQ-004, REQ-006, REQ-007.
- Deliverable: prove durable and wisp label-table routing, conservative dirty-table tracking, mixed-update rollback, hydrated results, and optional committed-state visibility.
- Verification: `go test ./internal/storage/doltlite -run 'Test.*Update.*Labels' -count=1`.
- Dependencies: WI-1 (`bdp-cu7`).

### WI-3 — Add conformance and provider contracts for label updates (`bdp-3ht`)

- Plan sections: Proposed Implementation 4 shared and public-boundary coverage.
- Files: `internal/storage/conformance/conformance.go`, `internal/provider/provider_test.go`.
- Requirements: REQ-003, REQ-008, REQ-009.
- Deliverable: cover replacement, clearing, omission, normalization, malformed values, mixed updates, hydration, and a Gas City-compatible `encoding/json` request shape through the public provider operation.
- Verification: `go test ./internal/storage/conformance ./internal/provider -run 'Test.*Update.*Labels' -count=1`.
- Dependencies: WI-1 (`bdp-cu7`).

### WI-4 — Run repository and plugin process proof for label updates (`bdp-2vr`)

- Plan sections: Verification.
- Files: no owned implementation files; verification evidence only.
- Requirements: REQ-010 and final evidence for REQ-001 through REQ-009.
- Deliverable: run focused packages, full tests and vet, build both plugin entry points with DoltLite linkage, and run focused then full `doltlite-plugin` public-process conformance. Implementation failures return to their owning work item.
- Verification: the complete command set in the approved plan's Verification section.
- Dependencies: WI-2 (`bdp-wdk`) and WI-3 (`bdp-3ht`).

