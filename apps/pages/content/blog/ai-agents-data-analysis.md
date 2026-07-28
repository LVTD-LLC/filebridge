---
title: "AI Agents for Data Analysis: A Reviewable Workflow"
description: "Use AI agents for data analysis with a reviewable workflow that preserves evidence, validates outputs, and separates findings from actions."
published_at: 2026-07-28
updated_at: 2026-07-28
author: Rasul Kireev
keywords:
  - AI agents for data analysis
  - AI agent data analysis
  - data analyst agent
  - agentic data analysis
topics:
  - data analysis
  - agent workflows
  - reviewable AI
canonical_url: https://rowset.lvtd.dev/blog/ai-agents-data-analysis
image: /static/vendors/images/logo.png
image_alt: Rowset logo
robots: index, follow
---

AI agents can perform useful data analysis when they have an authorized data
interface, a bounded question, tools for deterministic computation, and a
review path. The reliable pattern is to keep source evidence, analysis runs,
findings, decisions, and operational actions separate instead of treating the
agent's final answer as the record of truth.

Use AI agents for these seven analysis jobs:

1. inspect schemas and profile a dataset;
2. find missing, invalid, or duplicate values;
3. translate a bounded question into a query or computation;
4. compare groups, periods, or defined cohorts;
5. surface exceptions for investigation;
6. draft evidence-linked findings and explanations; and
7. prepare a decision proposal for human or policy review.

An agent may do several of these jobs in one run. It should not silently jump
from "I found a pattern" to "I changed the business." The handoff between
analysis and action needs its own durable contract.

## Can AI agents do data analysis?

Yes. A data-analysis agent can inspect files, query a database, execute code,
calculate summaries, compare segments, and explain results. Google's current
Agent Development Kit codelab demonstrates a data analyst agent that explores
uploaded files and queries BigQuery through a toolset ([Google ADK codelab,
checked July 2026](https://codelabs.developers.google.com/devsite/codelabs/build-agents-with-adk-data-analyst-agent?hl=en)).

Access to tools is only the start. A useful analysis also needs:

- an exact question and decision context;
- a known source and snapshot time;
- schema and field meaning;
- reproducible queries or computations;
- evidence for each finding;
- validation against expected conditions; and
- a rule for what the agent may do with the result.

The Model Context Protocol (MCP) gives models schema-defined tools for operations
such as querying databases, calling APIs, and running computations. Its tool
specification also recommends keeping a person able to deny tool invocations
([MCP tools specification, checked July
2026](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)).
MCP can make a capability discoverable. It does not prove that a query is
appropriate, a result is correct, or an action is authorized.

## Seven ways to use AI agents for data analysis

### 1. Inspect schemas and profile the data

Start by asking the agent to describe what exists, not what it means. It can
list tables or files, inspect field names and types, count rows, measure missing
values, identify duplicate keys, and report ranges or allowed values.

Use deterministic code or database functions for exact counts. Let the model
organize the profile and explain anomalies, but do not ask it to estimate totals
from a sample or a truncated context window.

A profile should record the source locator, snapshot time, query or script
version, and result. That makes it possible to compare the same checks after a
transformation or on a later run.

### 2. Find quality problems without overwriting the source

An agent can classify data-quality issues into deterministic fixes, reviewable
proposals, and prohibited changes. Trimming documented whitespace is different
from merging two customer records that happen to look similar.

Keep the raw source unchanged. Store each proposed semantic change with the
source key, field, old value or protected evidence reference, proposed value,
rule version, and reason. The [safe AI data-cleaning
workflow](/blog/ai-data-cleaning-agent) explains that proposal boundary in
detail.

This is where agent analysis is often most valuable: it can group messy cases
and explain why they may belong together while deterministic checks preserve
the facts around them.

### 3. Translate a bounded question into a query

Natural-language-to-SQL or code generation is useful when the question and data
scope are explicit. "Compare activated trial accounts by signup week for the
last 90 days" is bounded. "Find something interesting in production" is not.

Before execution, the agent should show:

- the question it is answering;
- the source tables, files, or datasets;
- filters and time boundaries;
- join keys and exclusions;
- the aggregation grain;
- the planned query or computation; and
- the checks that will make the result plausible.

Prefer read-only credentials for analysis. Apply row limits, timeouts, and cost
controls in the tool or database layer rather than relying on a prompt.

### 4. Compare defined groups or periods

Agents are good at generating a repeatable comparison plan and narrating the
result. The plan still needs stable definitions.

For a period comparison, record the timezone, date field, incomplete-period
policy, and whether the source can be updated after the fact. For a cohort
comparison, record the inclusion rule, exclusion rule, and denominator. For a
business metric, link the metric definition rather than letting the agent infer
it from a column name.

The finding should state what was compared and under which definition. A
sentence such as "conversion fell" is incomplete without the population,
window, denominator, and evidence query.

### 5. Surface exceptions for investigation

An agent can turn deterministic exception lists into a review queue. Examples
include unexpected category values, records that violate an invariant,
outliers beyond a documented threshold, or two systems that disagree about the
same stable ID.

The agent should not label every unusual value an error. An exception is a
candidate for investigation until the relevant rule and source evidence support
a conclusion.

Store one finding per exception class or affected record, depending on the
review job. Give each finding a stable ID so a reviewer can approve, reject, or
request more information without referring to "the third bullet in the agent's
message."

### 6. Draft findings with evidence and limitations

The strongest role for a language model is often the layer between calculation
and decision. It can explain a result, name plausible interpretations, and show
which evidence would distinguish them.

Every finding should carry:

- a stable `finding_id`;
- the exact `run_id`;
- the question answered;
- a concise claim;
- evidence references or result rows;
- the method or query version;
- known limitations;
- a validation status; and
- a review status.

Provenance is the thread connecting a result to how it was produced. W3C PROV
defines provenance around the entities, activities, and people involved in
producing data so its quality, reliability, or trustworthiness can be assessed
([W3C PROV Overview](https://www.w3.org/TR/prov-overview/)). A small workflow
does not need the full standard, but it does need enough identifiers to
reconstruct the path.

### 7. Prepare a decision proposal, not an automatic action

Analysis can support a pricing change, inventory adjustment, customer outreach,
roadmap decision, or operational update. Those actions have different evidence
and approval requirements.

Create a separate decision record that points to the finding and states the
proposed action, affected object, expected state, risk tier, reviewer, and
decision. Then let an executor apply only the approved fields and read the
destination back.

NIST's AI Risk Management Framework calls for documented, repeatable testing,
evaluation, verification, and validation, including evaluation under
conditions similar to deployment ([NIST AI RMF Core, checked July
2026](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)). A plausible
answer in a chat window is not a deployment-like test.

## The seven-stage analysis contract

Use this sequence for analysis that may influence operational work:

```text
question -> snapshot -> plan -> evidence -> finding -> decision -> action
```

Each stage answers a different question:

| Stage | Required record | Question it answers |
|---|---|---|
| Question | analysis request | What decision or understanding is this analysis meant to support? |
| Snapshot | source locator and version | Which exact data did the run use? |
| Plan | query, code, fields, and checks | How will the agent answer without changing the source? |
| Evidence | query result or protected reference | What observations support the claim? |
| Finding | versioned analytical claim | What does the run conclude, with which limits? |
| Decision | authenticated review | Is a proposed follow-up accepted, rejected, or blocked? |
| Action | bounded execution and read-back | What changed, and was the destination verified? |

The contract prevents three common collapses:

1. **Source into interpretation:** a model-generated label replaces the original
   record.
2. **Finding into decision:** a correlation or anomaly is treated as a business
   instruction.
3. **Decision into execution:** an approved idea is applied to a changed target
   without re-reading current state.

Do not compress the stages merely because one agent can call every tool. A
single runtime can still write separate records and enforce separate
permissions.

## A reviewable data-analysis structure in Rowset

Rowset is not a warehouse, notebook, SQL engine, or model runtime. Let the agent
read files, query BigQuery or another database, and run computations with the
appropriate tools. Use Rowset when the workflow needs private structured state
for analysis runs, findings, review decisions, and verified follow-up.

A practical setup uses four datasets:

### `analysis_sources`

Index by `snapshot_id`. Store the protected source locator, source version or
time boundary, schema version, capture time, and content hash where appropriate.
Do not copy sensitive raw data into Rowset when a protected locator is enough.

### `analysis_runs`

Index by `run_id`. Store the question, `snapshot_id`, method or query reference,
agent or model configuration, start and finish times, validation status, and
safe execution metadata.

### `analysis_findings`

Index by `finding_id`. Store the `run_id`, claim, evidence reference,
limitations, severity or importance under a defined policy, and review status.
Create a new finding version when the source, method, or interpretation changes.

### `analysis_decisions`

Index by `decision_id`. Store the `finding_id`, proposed action, exact target,
allowed fields, decision, reviewer, reason, and decision time. A separate
executor can add execution and verification references after an approved
action.

Use the [schema-design guide](/docs/design-schema) to define field meaning,
types, and allowed values. Use [hosted MCP](/docs/connect-mcp) when the agent
benefits from tool and schema discovery, or the [Dataset
API](/docs/dataset-api) when an analysis script already speaks HTTP. In both
paths, inspect the dataset before writes and update by a stable index.

## Treat source data and tool output as untrusted

Files, database text, web pages, support tickets, and tool results can contain
instructions aimed at the model. They are data to analyze, not authority that
can change the analysis contract or grant access.

OWASP's AI Agent Security guidance recommends least-privilege tools, validation
of external inputs and structured outputs, human oversight for high-impact
actions, and separation of decisions from irreversible execution ([OWASP AI
Agent Security Cheat Sheet, checked July
2026](https://cheatsheetseries.owasp.org/cheatsheets/AI_Agent_Security_Cheat_Sheet.html)).

Apply those controls outside the prompt:

- give the analysis step read-only access when possible;
- validate query parameters and structured outputs;
- keep credentials in the host, not in model-visible data;
- exclude messaging, deletion, publishing, and financial tools from analysis;
- bind approvals to an exact action and current target state; and
- retain safe evidence without copying secrets or private payloads into logs.

The [human-in-the-loop agent workflow](/blog/human-in-the-loop-ai-agents)
provides a reusable approval record. The [idempotent update
guide](/blog/idempotent-ai-agent-updates) covers read-before-write,
desired-state updates, and destination verification.

## How to evaluate AI agents for data analysis

Do not grade the workflow by how convincing the explanation sounds. Test the
parts separately:

1. **Source fidelity:** can the run identify the exact data snapshot?
2. **Query validity:** does the query match the stated population, grain, joins,
   filters, and metric definition?
3. **Calculation reproducibility:** does independent code return the same
   result?
4. **Finding support:** does each claim point to sufficient evidence?
5. **Limitation coverage:** does the finding disclose missing data, ambiguous
   definitions, and unsupported causal interpretations?
6. **Decision integrity:** can only an authenticated reviewer or policy approve
   the proposed follow-up?
7. **Execution verification:** does the executor read the destination back and
   compare the approved fields?

Keep regression cases for observed failures. If a schema, query generator,
model, tool policy, or metric definition changes, rerun the relevant cases
before trusting the same workflow in production.

## When not to use an AI agent for data analysis

Use a fixed report, tested query, or deterministic pipeline when the question
and output never change. An agent adds value when it must inspect unfamiliar
schema, adapt a plan, group ambiguous cases, or explain findings. It also adds
another failure surface.

Do not let an analysis agent:

- use unrestricted production credentials;
- make destructive source changes during exploration;
- invent missing values, metric definitions, or causal explanations;
- approve its own consequential recommendation;
- expose sensitive source rows in general logs or public previews; or
- treat confidence as authorization.

For high-stakes, regulated, safety-critical, or financially material analysis,
involve the relevant domain, data, security, and compliance experts. The
workflow above improves traceability; it does not certify the analysis.

## Frequently asked questions

### Can AI agents do data analysis?

Yes. An AI agent can inspect schemas, run queries or code, calculate summaries,
compare groups, and explain evidence. Reliable use requires authorized data
access, reproducible computation, source provenance, output validation, and a
review boundary before consequential actions.

### What is the best AI agent for data analysis?

The best choice depends on the data interface and job. Prefer an agent that can
use your required file, SQL, warehouse, notebook, or MCP tools; run
deterministic computations; show its query or method; cite result evidence; and
operate under read-only or narrowly scoped permissions.

### Can ChatGPT be used for data analysis?

ChatGPT can analyze supplied files and use connected tools when those
capabilities are available in the selected product and workspace. The same
controls still apply: verify the source, inspect the method, reproduce important
calculations, and keep operational actions behind separate authorization
([OpenAI data-analysis guide, checked July
2026](https://help.openai.com/en/articles/8437071-data-analysis-with-chatgpt/)).

### Who are the big four AI agents?

There is no stable, authoritative "big four" for data-analysis agents. The
phrase can refer to model providers, assistant products, agent frameworks, or
analytics vendors, and those categories change. Compare products by data
connectors, computation, evidence, permissions, and deployment needs instead of
repeating an arbitrary four-name list.

## Start with one bounded analysis

Choose a question that can be checked independently. Give the agent read-only
access to one source, require it to show the plan and evidence, and store the
finding separately from the decision. Only add an execution tool after the
review and verification path works.

The [Rowset quickstart](/docs/quickstart) shows how to create the structured
work surface. [Rowset pricing](/pricing) includes a seven-day full-product trial
for testing the pattern with private MCP or REST access.
