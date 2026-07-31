# Agent change control-surface audit

Date: 2026-07-31  
Task: RFO-012  
Scope: authenticated Rowset surfaces on merged `main`

## Result

For the successful operations exercised in this audit, Rowset already provides a
credible review surface:

- Named agent keys are visible on datasets, rows, and mutation history.
- Row updates retain and render exact before-and-after values.
- Dataset instructions are available to both agents and account owners.
- Snapshot exports are available in CSV, JSONL, XLSX, SQLite, and Parquet.
- Archiving preserves a dataset and its rows, and the account owner can restore it.

The clearest confirmed blind spot is unsuccessful work. A resolved agent identity
can attempt a write and receive a permission or validation failure, but the account
owner sees no record of that attempt in Rowset. The other material gaps are
recovery below the dataset level, immutable authorization context, incomplete
presentation of non-row changes, and the status of background work.

These gaps should be addressed as a control surface for delegated work. They do
not justify spreadsheet-style manual authoring or a general operations console.
The follow-up order below is a control-surface risk ranking, based on detectability,
reversibility, and potential owner harm. It is not evidence that failed writes are
the most frequent customer problem; production incidents and user research should
be allowed to change the order.

## Controlled flow

The audit used an isolated Docker stack and a private, sanitized dataset with a
stable `case_id` index. Three local keys represented two read-write agents and one
read-only agent.

| Scenario | Result | Human-visible evidence |
| --- | --- | --- |
| Agent A creates a dataset with two rows | REST `201`; one `dataset.created` mutation | Dashboard and dataset show `Created by Audit Agent A` |
| Agent B updates one row | REST `200`; one `row.updated` mutation | Row shows `Touched by Audit Agent B`; Changes shows exact field diffs |
| Read-only agent attempts the same write | REST `401`; no data change | Operational log only; no dataset change or attempt entry |
| Agent B creates an index collision | REST `409`; no data change | Operational log only; no dataset change or attempt entry |
| Account archives and restores the dataset | Two web `302` responses; rows preserved | Archived badge and Unarchive action; archive and restore mutations |
| Agent B updates description and instructions | REST `200`; previous/current values retained | Changes shows only the field names, not the recorded values |

The mutation count was two after the successful create and update. It remained
two after both failed writes. This isolates the failure-visibility gap from the
existing successful-mutation implementation.

Agent writes in the controlled flow used REST. Hosted MCP must reproduce the same
read-only denial and index-conflict cases before RFO-016 is considered complete;
REST results alone are not MCP evidence.

The authenticated UI was checked at 1280 by 900 and 375 by 844. The dataset and
Changes pages had no document-level horizontal overflow and emitted no browser
console errors. At mobile width, the row-diff table remains intentionally
horizontally scrollable inside its own bounded container.

## Current control-surface matrix

| Capability | Current state | Classification | Decision |
| --- | --- | --- | --- |
| Creation attribution | Supported by immutable actor label and key relation | No confirmed gap | Keep |
| Multi-agent row attribution | Supported on row, dataset, dashboard, and Changes | No confirmed gap | Keep |
| Row before/after values | Supported for `row.updated` | No confirmed gap | Use as the basis for guarded revert |
| Dataset instructions | Supported in a collapsed context panel and API/MCP metadata | No confirmed gap | Keep |
| Recent successful mutations | Supported per dataset; dashboard shows recently updated datasets | No confirmed gap | Do not add a global activity feed from this audit |
| Failed authenticated writes | Only request/auth logs; no owner-visible durable event | Missing data model, auth/service, API/MCP, and UI | RFO-016 |
| Row-update recovery | Required values exist, but no revert service or action exists | Service, API/MCP, and UI | RFO-017 |
| Deleted-row recovery | Deletion history stores only row id and number | Missing private recovery data, service, API/MCP, and UI | RFO-018 |
| Permission at mutation time | Current key permission is in Settings; mutation stores actor name only | Missing immutable audit data and UI | RFO-019 |
| Non-row before/after display | Safe structured metadata often exists, but the presenter ignores it | Presenter/UI | RFO-020 |
| Background operation status | Vector and asset failures are in worker logs or admin-only records | Missing dataset-scoped state, task integration, API/MCP, and UI | RFO-021 |
| Snapshot export | Supported in five formats | No confirmed gap | Keep |
| Dataset archive recovery | Supported and preserves rows | No confirmed gap | Keep |

## Prioritized follow-up tasks

### P0: RFO-016 — failed authenticated writes

Start with resolved-key row writes that are denied for insufficient permission or
rejected for an index conflict or validation error. Cover both REST and hosted MCP.
Show the event in dataset Changes only after ownership is established; otherwise
keep it account-scoped to the resolved key without revealing a foreign target
identifier or whether that target exists.

Each event should name the agent, interface, intended operation, safe failure
category, and one concrete owner next step. Unknown credentials, request payloads,
cell values, headers, raw error strings, and foreign resource identifiers must not
become audit data. Bound storage with aggregation, rate controls, and a finite
retention policy so repeated failures cannot create an unbounded audit stream.

This is the first control-surface task because invisible failures are hard for an
owner to detect and diagnose. Its priority should be rechecked against production
incidents and user research before expanding beyond the first-release operations.

### P1: RFO-017 — guarded row-update revert

Use existing row diffs only after proving that the retained values are complete
enough to replay. A revert must validate current ownership and write permission,
column existence, types, row shape, and index compatibility. The UI should preview
the inverse diff and require confirmation; REST and MCP should require an explicit
revert action tied to the original mutation.

On stale or conflicting state, change nothing and return a clear refusal with the
current values needed to recover. On success, append a new mutation attributed to
the invoking actor and link it to the original change.

### P1, planned: RFO-018 — recoverable non-asset row deletion

Treat deleted-row restoration separately because it requires a new private-data
retention contract. Before implementation can enter Ready, decide a finite
retention duration, owner-only access boundary, permanent purge behavior, and
account- and dataset-erasure behavior. Retained deleted values must stay out of
public previews and ordinary exports.

The first scope should exclude image and audio binaries. Restore must apply the
same current ownership, write-permission, schema, and index-conflict checks as an
ordinary row create, and must append a mutation attributed to the restoring actor.

### P1: RFO-019 — authorization context

Snapshot permission and interface at mutation time. Current key state can add a
revoked indicator, but it must not replace immutable event-time evidence.

### P1: RFO-021 — derived-operation health

Expose dataset-scoped pending or failed indexing and cleanup work without
turning Rowset into a generic job console. Pending state should say that no owner
action is required. Failed state should show an allowlisted safe category and one
next action, without raw exceptions, task arguments, storage paths, or provider
responses.

A successful retry must resolve the corresponding warning while leaving a compact
history entry. Normal first-attempt completion should remain quiet.

### P2: RFO-020 — richer non-row diffs

Render safe, already-recorded transitions through a mutation-specific
presentation layer. Retained description and instruction changes should show
before-and-after values. Mutation types that intentionally omit values should show
an explicit "values not retained" state; this task must not expand retention.

## Explicit non-goals

- Spreadsheet-style cell editing or collaborative authoring
- A global request-log or worker-log viewer
- Public audit data
- Invalid-credential surveillance
- Dataset-wide time travel
- Generic backup infrastructure
- Image or audio undelete in the first row-recovery scope
- A global activity feed without additional evidence that per-dataset history
  and recently updated datasets are insufficient
