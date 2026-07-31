---
title: Start with your first agent dataset
description: Connect a trusted AI agent to Rowset and create one useful API-backed dataset.
keywords: Rowset tutorial, getting started, MCP, dataset API
---

# Start with your first agent dataset

This guide connects a trusted agent to Rowset and creates one small dataset the
agent can inspect and update later.

You will use the dashboard for setup, then the agent will inspect its runtime
and automatically configure the best supported interface. Public previews stay
off unless you explicitly ask to share a read-only browser page.

Use this as the shortest path. After it works, use the broader
[dataset guide](/docs/datasets) when you need projects, relationships, image
columns, exports, or public previews.

## Before you start

You need a Rowset account, a trusted agent runtime, and a private place to store
an API key such as an environment variable or secret store.

## 1. Copy the Rowset setup prompt

Sign in to Rowset and copy the dashboard agent setup prompt.

The docs show a masked example:

```text
{{ agent_setup_prompt_masked }}
```

The dashboard preview masks the API key. The copy button includes the real key,
so treat the copied prompt like a password.

## 2. Let the agent choose automatically

The agent should use this fixed priority:

- MCP when the runtime supports native remote MCP and private bearer-secret
  configuration
- otherwise CLI for a trusted terminal or local-file workflow
- REST for a code-only or HTTP-only runtime

It should configure that interface end to end without asking you to compare the
options. It should pause only for an unavoidable operating-system,
authentication, or secret-manager permission prompt.

If setup is interrupted, treat it as `inspect -> choose -> configure -> verify`.
The agent should report completed steps, the failed or cancelled step, whether
private credential storage is confirmed, unknown, or absent, whether
verification was not run, failed, or succeeded, and exactly one safe retry
action. Cancelled authentication or permission leaves setup incomplete.
Verification that was not run or failed leaves setup incomplete; only succeeded
verification makes setup complete. Before retrying, inspect existing
configuration and secret storage. Do not create duplicate configuration or
rotate or replace credentials unless explicitly requested.

## 3. Configure the selected interface

The agent stores the key as `ROWSET_API_KEY` or in an equivalent secret store,
then follows the current guide for the selected interface.

For MCP and REST, Rowset expects a bearer token:

```http
Authorization: Bearer {{ api_key_placeholder }}
```

Private REST requests accept only `Authorization: Bearer <key>`.

Current interface references:

- [Connect over MCP](/docs/connect-mcp)
- [Use Rowset from the CLI](/docs/use-cli)
- [Dataset API](/docs/dataset-api)

## 4. Configure only what the connection needs

Follow the current connection guide for the selected interface. Do not load
capabilities or list datasets merely because a session started. Use exact tool,
command, or endpoint schemas for the operation at hand. If a feature is
unfamiliar or setup is failing, request the compact capability topic index with
`get_rowset_capabilities`, `rowset capabilities`, or `GET /api/capabilities`,
then load only the relevant topics. Use cases and full mode remain opt-in.

## 5. Verify access and complete onboarding

Make authenticated user-info the final setup action: call `get_user_info` over
MCP, run `rowset user info` through the CLI, or request `GET /api/user` through
REST. A successful response verifies the connection and completes onboarding.
MCP reads and API-key creation stay trial-neutral, so the MCP trial starts on the
first dataset or project mutation; CLI and REST user-info requests start it immediately.

For first-run activation, use already-authorized context from the current
conversation, repository and steering documents, active task, and sources
already authorized for that task. Produce one high-confidence project
recommendation with one to three concrete datasets, and explain the evidence in
one short sentence. Do not enumerate unrelated private resources. If evidence
is weak or contradictory, ask only:

> What are you working on that you want Rowset to help organize?

Return immediately after asking the weak-context question and wait for the
answer. Do not create the recommended project or datasets until the user
confirms. End a strong recommendation by asking, "Would you like me to create
that project and those datasets?" Defer the daily Rowset tips offer until the
project decision is resolved.

After verification, begin the requested task. If the user supplied a dataset
key or URL, inspect it directly: MCP `get_dataset` accepts either value; for CLI
or REST, extract the dataset key from the URL before using `rowset dataset get`
or `/api/datasets/{dataset_key}`. If the relevant dataset is unknown, search
with an explicit limit of 3, select one result, then load its full context.

## 6. Create one dataset

Ask the agent to create a small dataset with a stable index column. For a first
run, choose a workflow with a natural key:

- `email` for a personal CRM
- `task_id` for an agent task board
- `feedback_id` for feedback triage
- `sku` for a product catalog

If the source has no stable key, ask the agent to let Rowset generate
`rowset_id`.

Example prompt:

```text
Create a Rowset dataset named agent_tasks with headers task_id, status, owner,
next_action, and notes. Use task_id as the index column. Add three example rows
and include instructions that status must be todo, doing, blocked, or done.
```

The agent should call `get_dataset` after creation so it has the dataset key,
headers, index column, instructions, and schema context.

## 7. Try one update

Ask the agent to update a row by index value, then read it back:

```text
Update TASK-001 to status doing, then fetch that row by task_id and summarize
what changed.
```

You now have a private dataset the agent can continue using in later sessions.

## Next steps

- [How Rowset datasets work](/docs/datasets) for index columns,
  projects, relationships, schema, exports, and previews.
- [Work with rows](/docs/work-with-rows) for read, search, create, update, and
  delete patterns.
- [Connect over MCP](/docs/connect-mcp) for a focused MCP setup guide.
- [Use Rowset from the CLI](/docs/use-cli) for terminal access through REST.
- [MCP tool reference](/docs/mcp-tools) when an agent needs exact tool groups.
- [Use cases](/use-cases) for starter dataset shapes.
