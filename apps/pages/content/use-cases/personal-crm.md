---
title: Agent-managed personal CRM
description: Use Rowset as an agent-managed personal CRM for people, companies, conversations, follow-ups, and relationship context.
keywords: agent CRM, personal CRM, Rowset use case
---

# Agent-managed personal CRM

Use Rowset when you want a trusted agent to maintain relationship context without
turning every follow-up into a manual spreadsheet chore.

For a complete implementation with separate people, interaction, and commitment
records, follow the [AI agent CRM guide](/blog/ai-agent-crm).

Keep recalled preferences in agent memory and current contact fields in the CRM
dataset. The guide to [AI agent memory vs structured
state](/blog/ai-agent-memory-vs-state) shows how to choose the authoritative
home for each fact.

## Starter shape

Create a `people` dataset. Use `email` as the index when contacts have reliable
email addresses, or `person_id` when one person can have several addresses.

People dataset indexed by email or person_id.

| email | name | company | contact_category | last_interaction | next_contact | notes |
| --- | --- | --- | --- | --- | --- | --- |
| alex@example.com | Alex Morgan | Northstar Labs | A | 2026-07-01 | 2026-07-22 | Asked for implementation examples |
| sam@studio.dev | Sam Lee | Studio Dev | B | 2026-06-24 | 2026-08-24 | Intro from May conference |
| nora@acme.com | Nora Patel | Acme | C | 2026-06-28 | 2026-12-28 | Wants security details |

## Agent jobs

- Add people and companies from meeting notes, emails, or chat summaries.
- Update relationship stage after each conversation.
- Find stale promises before they become dropped balls.
- Export a CSV or JSONL snapshot when you want a backup or handoff.

## Dataset context and semantic schema

Add instructions that define stage meanings, follow-up rules, and what counts as
private notes. Mark `email` as an email column, `last_interaction` as a date,
and `next_action` as free text. Keep the agent honest: it should update rows
only from trusted notes or direct user instruction.

## Calculate follow-up dates

Create `next_contact` as a date formula when each contact category has a fixed
cadence:

```text
SWITCH(
  {contact_category},
  "A", DATEADD({last_interaction}, 3, "weeks"),
  "B", DATEADD({last_interaction}, 2, "months"),
  "C", DATEADD({last_interaction}, 6, "months"),
  "D", DATEADD({last_interaction}, 12, "months")
)
```

Add a boolean formula such as
`AND({next_contact}, TODAY() >= {next_contact})` to make due contacts directly
filterable through MCP, REST, and the dataset view. Formula columns calculate
live and remain read-only; your agent only updates source fields such as
`last_interaction` and `contact_category`.

## Connect it

Use [MCP access](/docs/connect-mcp) first. If MCP is unavailable, use the
[Dataset API](/docs/dataset-api) with a bearer API key. Public previews should
stay off unless you deliberately want a read-only relationship board.
