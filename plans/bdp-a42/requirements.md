---
schema: gc.build.requirements.v1
workflow:
  id: bdp-wisp-ioz
  formula: build-basic
methodology:
  pack: gascity
  name: build-basic
producer:
  formula: build-basic
  stage: requirements
  attempt: 1
status: approved
trace:
  upstream:
    - path: beads/bdp-a42
      hash: bead:bdp-a42
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
    - path: beads/bdp-wisp-5x1
      hash: bead:bdp-wisp-5x1
    - path: beads/bdp-wisp-ioz
      hash: bead:bdp-wisp-ioz
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

# Requirements: Support Labels in DoltLite Backend Issue Updates

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

## Problem Statement

Gas City records order run cursors and outcomes by sending issue updates through the Beads backend plugin contract. Those updates can contain a `labels` field. The DoltLite backend currently passes the update map to the generic issue update path, whose allowed-field validation covers row-backed issue columns but rejects `labels` with `storage_error: invalid field for update: labels`. As a result, otherwise valid tracking updates fail and Gas City repeatedly retries nudge-on-route and nudge-mail-sweep work for beads such as `dg-wisp-qvg` and `dg-wisp-e6k`.

The backend must treat labels as a supported issue-update field even though they are stored in the related `labels` or `wisp_labels` table. The change must preserve existing update behavior, route label writes to the correct table for regular issues and wisps, and prove compatibility with the Gas City request shape.

## W6H

- Who: Gas City order tracking callers, Beads backend-plugin clients, and maintainers of the DoltLite storage implementation.
- What: accept a `labels` array in the existing plugin-backed issue update request and persist it as the issue's complete desired label set.
- When: whenever an `update_issue` request includes `updates.labels`, including cursor and outcome tracking updates during routing and mail sweeps.
- Where: the `beads-backend-doltlite-plugin` rig, across the plugin request boundary, provider normalization, shared issue-update operation, and DoltLite transaction path.
- Why: Gas City tracking must not fail merely because its update includes labels, and callers must observe the persisted set on the returned or subsequently fetched issue.
- How: extend the existing update contract to handle the relational labels field with the repository's established label-set semantics while preserving one logical update operation.
- How much: one focused compatibility fix with regression, storage conformance, and plugin contract coverage; no redesign of the broader label API.

## User Stories

- REQ-001: As Gas City order tracking, I can submit an issue update containing `labels` without receiving `invalid field for update: labels`.
- REQ-002: As a plugin client, I can set the complete desired label set and observe that exact normalized set on the updated issue.
- REQ-003: As a Gas City operator, cursor and outcome tracking updates that combine labels with ordinary fields complete successfully instead of entering repeated retry cycles.
- REQ-004: As a maintainer, I can rely on the same update behavior for regular issues and ephemeral wisp records.

## Technical Stories

- REQ-005: The backend update boundary recognizes `updates.labels` as a collection of strings, removes it from row-column processing, and reports malformed label values as a clear request or storage error rather than constructing invalid SQL.
- REQ-006: Label replacement and any ordinary field changes in the same request execute atomically, with label writes routed to `labels` for regular issues and `wisp_labels` for active wisps.
- REQ-007: The existing update lifecycle remains intact: actor attribution, updated timestamps, row-lock behavior, event/hook behavior, optional Dolt commit behavior, and returned issue hydration continue to work.
- REQ-008: Storage conformance coverage exercises replacement, clearing, mixed-field updates, duplicate/empty normalization, invalid payloads, and rollback behavior without regressing existing allowed update fields.
- REQ-009: A focused plugin or provider contract test reproduces the JSON-decoded Gas City request shape, including `[]interface{}` values produced by generic JSON decoding, and verifies persistence through the public update operation.
- REQ-010: Relevant focused and repository-level build/test suites pass, and the requirements remain traceable through the build-basic artifact contract.

## Behavior Requirements

- When `labels` is absent from `updates`, the current labels must remain unchanged.
- When `labels` is present, its value represents the complete desired label set; labels not present in the submitted set are removed and new labels are added.
- An explicitly empty label array clears all labels. Duplicate labels and empty strings are normalized consistently with the existing `SetLabels` behavior and must not create duplicate or empty rows.
- The accepted runtime shapes must include typed string slices and generic JSON-decoded arrays whose elements are strings. A scalar, object, or array containing a non-string element must fail with a useful error and must not partially update the issue.
- A mixed update containing `labels` and row-backed fields must either apply all requested changes or apply none of them.
- Label persistence must use the issue's active storage partition: regular issues use `labels`; active wisps use `wisp_labels`.
- The updated issue returned by the plugin/provider path, and a subsequent `get_issue`, must expose the final persisted labels.
- Existing `add_label`, `remove_label`, `get_labels`, and updates that do not contain `labels` must retain their current behavior.
- Existing event, hook, actor, dirty-table, and optional commit semantics must continue to represent one successful logical issue update; downstream planning must explicitly identify any unavoidable event-shape compatibility detail.
- Errors from label validation or persistence must remain classifiable by the existing plugin error response and must not be converted into success.

## Example Mapping

| Example | Given | When | Then | Requirement |
| --- | --- | --- | --- | --- |
| Gas City tracking update | A wisp has existing tracking labels | Gas City sends `updates` with cursor/outcome fields and a JSON labels array | The update succeeds, all fields persist, and no invalid-field error is returned | REQ-001, REQ-003, REQ-009 |
| Replace labels | An issue has labels `old` and `keep` | The client updates labels to `keep` and `new` | `old` is removed and the final set is exactly `keep`, `new` | REQ-002, REQ-006 |
| Omit labels | An issue already has labels | The client updates only its status or metadata | Existing labels are unchanged | REQ-002, REQ-007 |
| Clear labels | An issue has one or more labels | The client submits an empty labels array | All labels are removed | REQ-002, REQ-008 |
| Malformed labels | An issue has a title and labels | The client submits a title change with a labels array containing a non-string | The operation returns a useful error and neither the title nor labels change | REQ-005, REQ-006, REQ-008 |
| Wisp routing | The target is an active wisp | The client replaces its labels | Only the corresponding `wisp_labels` set changes and the hydrated wisp shows the result | REQ-004, REQ-006 |

## Acceptance Criteria

- REQ-001: A regression test sends an `update_issue` request containing `updates.labels` and proves the backend no longer returns `invalid field for update: labels`.
- REQ-002: Tests prove replacement semantics: omitted labels remain unchanged, a non-empty array becomes the exact normalized set, and an empty array clears the set.
- REQ-003: A Gas City-style mixed update for order cursor or outcome tracking succeeds and persists both its labels and its other update fields.
- REQ-004: Equivalent tests pass for a regular issue and an active wisp, with data stored and read from the correct label partition.
- REQ-005: Both `[]string` and JSON-decoded `[]interface{}` string collections are accepted; invalid element or container types return a clear error without panic or invalid SQL.
- REQ-006: A forced label persistence failure in a mixed update demonstrates transaction rollback: neither row-backed fields nor the label set are partially changed.
- REQ-007: The public update response and a later issue read contain the final label set, while actor, update/event/hook, dirty-table, and optional commit behavior remain compatible with existing updates.
- REQ-008: Storage conformance tests cover replacement, clearing, normalization, invalid input, atomic rollback, and the existing non-label update cases continue to pass.
- REQ-009: A focused plugin/provider integration or contract test uses the Gas City-compatible serialized request shape and proves run cursor/outcome tracking no longer fails on the labels field.
- REQ-010: Focused package tests, the applicable conformance suite, and the repository's relevant build/test gates pass; this document validates as `gc.build.requirements.v1` with matching YAML and Markdown coverage pairs.

## Out Of Scope

- Changing Gas City's order tracking model, retry policy, cursor format, or outcome format.
- Adding merge-mode flags or changing the standalone `add_label` and `remove_label` operations; update-field labels use replacement semantics.
- Redesigning label schemas, migrations, filtering, inheritance, or cross-repository synchronization.
- Changing label ordering into a public contract; comparisons should treat the result as a normalized set unless an existing API already guarantees order.
- Broad refactoring of generic issue update SQL or unrelated backend-plugin fields.
- Implementing source changes, releasing binaries, opening a pull request, or deploying during this requirements stage.

## Open Questions

- The exact event payload representation for a relational label replacement should be selected during design after comparing existing `UpdateIssue` and standalone label-operation event behavior; the externally required outcome is one compatible logical update without duplicate hooks or partial persistence.
- The narrowest public-boundary test seam—backend plugin subprocess, provider session, or both—should be chosen during planning based on existing test harness cost, but it must exercise generic JSON decoding rather than only a typed in-process map.
- Label ordering is intentionally not specified beyond existing API guarantees; the implementation and tests should compare normalized membership unless current contract tests establish a stable order.
