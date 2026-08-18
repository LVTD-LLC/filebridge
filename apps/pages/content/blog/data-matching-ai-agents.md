---
title: "Data Matching for AI Agents: A Reviewable Workflow"
description: "Use data matching with AI agents to generate candidate pairs, review uncertain links, and preserve approved canonical IDs without silent merges."
published_at: 2026-08-18
updated_at: 2026-08-18
author: Rasul Kireev
keywords:
  - data matching
  - record linkage
  - entity resolution
  - data deduplication
topics:
  - agent workflows
  - data quality
  - row identity
canonical_url: https://rowset.lvtd.dev/blog/data-matching-ai-agents
image: /static/vendors/images/logo.png
image_alt: Rowset logo
robots: index, follow
---

Data matching compares records to decide whether they describe the same real-world entity. In an
AI-agent workflow, use the agent to prepare and explain candidate pairs, not to silently merge
records. Preserve both source rows, route uncertain matches to review, and publish an approved
canonical ID only after the evidence meets a written decision rule.

Data matching is also called record linkage or entity resolution. The same methods can find
duplicates inside one dataset or connect records across several systems. For example, an agent may
need to decide whether `contact_731` in a CRM and `cus_1842` in billing belong to the same person.

This guide uses a five-step **MATCH contract**:

1. **Materialize** source records without changing them.
2. **Assemble** plausible candidate pairs.
3. **Test** each pair against explicit evidence.
4. **Confirm** uncertain decisions through review.
5. **Hand off** only approved canonical identity mappings.

## In this guide

- [What is data matching?](#what-is-data-matching)
- [When should an AI agent match records?](#when-agent-matches-records)
- [The MATCH contract](#match-contract)
- [Exact, fuzzy, and probabilistic matching](#matching-methods)
- [A reviewable candidate-pair schema](#candidate-pair-schema)
- [How to run the workflow with Rowset](#data-matching-rowset)
- [How to evaluate match quality](#evaluate-data-matching)
- [Common data-matching failures](#data-matching-failures)
- [Data matching FAQ](#data-matching-faq)

<a id="what-is-data-matching"></a>
## What is data matching?

Data matching is the process of identifying records in one or more datasets that refer to the same
entity. A match may connect two source records, identify a duplicate inside one file, or associate
several aliases with one canonical customer, product, supplier, ticket, or other operational
object.

The Python Record Linkage Toolkit describes record linkage as bringing together information from
records believed to belong to the same entity. Its documented workflow covers cleaning, indexing,
comparison, classification, and evaluation
([toolkit documentation, checked August 2026](https://recordlinkage.readthedocs.io/en/latest/about.html)).
An evidence review for the US Agency for Healthcare Research and Quality likewise distinguishes
deterministic and probabilistic linkage and explains that the appropriate method depends on the
available identifiers, their quality, and the cost of mistakes
([AHRQ record-linkage overview, checked August 2026](https://www.ncbi.nlm.nih.gov/books/NBK253312/)).

Data matching and data deduplication overlap, but they are not identical:

| Task | Input | Output |
|---|---|---|
| Deduplication | one dataset | groups of rows that may represent the same entity |
| Record linkage | two or more datasets | candidate links between source records |
| Entity resolution | records and evidence | a decision about which records belong to one entity |
| Identity crosswalk | approved decisions | exact source-ID-to-canonical-ID mappings |

The last step matters for agents. A similarity score is evidence; it is not a durable identity.
Once a decision is approved, store it in a reviewed
[crosswalk table](/blog/crosswalk-table-ai-agents) so later agent actions use exact IDs instead of
repeating a fuzzy match.

<a id="when-agent-matches-records"></a>
## When should an AI agent match records?

Use an AI agent when matching requires context that deterministic code cannot express cleanly, but
keep the final authority proportional to the risk.

Good agent-assisted cases include:

- comparing company names after punctuation and legal suffixes are normalized
- explaining why two product records appear related despite different descriptions
- grouping support contacts that share verified external references
- preparing candidate pairs for a person who knows the business context
- converting reviewer decisions into consistent, exact mappings

Prefer ordinary code or database constraints when the rule is exact. If every source preserves the
same immutable `customer_id`, an exact join is faster, cheaper, and easier to test than a model. If
two rows have different stable IDs but a trusted external mapping already exists, use that mapping.
Do not ask an agent to rediscover a known relationship.

The risk test is simple: what happens if the pair is wrong? A false product match may attach the
wrong supplier data. A false customer match may expose one person's history to another. A false
ticket match may close unresolved work. Higher-impact decisions need stronger evidence and a
reviewer; a confident explanation from a model does not lower the consequence.

<a id="match-contract"></a>
## Use the MATCH contract for reviewable record linkage

MATCH separates the reversible work of finding candidates from the consequential work of
declaring identity.

### 1. Materialize the source records

Copy or reference the source records without overwriting them. Preserve each source system, source
ID, source version, and collection time. Normalize comparison fields into separate columns while
keeping the original values.

For example, keep both `display_name = "Acme, Inc."` and `normalized_name = "acme"`. The normalized
value helps comparison; the original value remains evidence. Never replace a source identifier
with a generated guess.

This follows the same reversible boundary as the
[AI data-cleaning workflow](/blog/ai-data-cleaning-agent): source rows are evidence, while proposed
changes and match decisions live in separate records.

### 2. Assemble plausible candidate pairs

Do not compare every row with every other row unless the datasets are tiny. Generate a bounded set
of plausible pairs using stable clues such as an external reference, normalized domain, postal
code, supplier namespace, or product family.

Record-linkage literature calls this **blocking**. The AHRQ overview explains that blocking reduces
the comparison space by considering pairs that share selected characteristics. It also warns that
blocking choices affect results, so the method should be documented and often run in multiple
passes when source values may be missing or wrong.

Keep the blocking rule with every run:

```text
candidate_run = match-2026-08-18-v1
blocking_rules = exact(external_ref) OR exact(domain, postal_code)
source_versions = crm-418, billing-2026-08-18
```

An agent can propose additional candidates, but it should not bypass the candidate boundary and
search unrelated private data.

### 3. Test the evidence

Compare fields independently and preserve the result of each test. Useful evidence may include:

- exact agreement on a stable external identifier
- normalized name agreement
- email-domain or phone agreement, where authorized
- address similarity
- conflicting dates, regions, or ownership fields
- a source document that explicitly connects both IDs

Avoid one opaque `confidence` number as the entire explanation. A pair with a score of `0.92` is not
reviewable unless the workflow also records which features agreed, which conflicted, the matching
method, and the method version.

Treat source text as untrusted data. Notes, descriptions, and imported documents can provide match
evidence, but they cannot change the matching rules, grant access, or instruct the agent to approve
the pair.

### 4. Confirm uncertain decisions

Classify candidates into at least four states:

- `proposed`: the pair has not been reviewed
- `approved`: the evidence meets the written rule or a reviewer accepted it
- `rejected`: the records are known to describe different entities
- `needs_review`: evidence conflicts or remains incomplete

Write review rules before the first run. For example, an exact trusted external reference may be
auto-approved, while a name-and-address similarity requires review. A missing value is not
agreement. Two common names are not proof. A model's ability to produce a plausible story is not
evidence that the records match.

The AHRQ overview describes manual review as a normal validation step and recommends decision
rules that standardize reviewer judgments. Rowset can hold the review queue and decision history;
the agent or matching library still performs the comparison.

### 5. Hand off approved canonical identity

Only approved matches should create or update the operational crosswalk. The handoff turns a
probabilistic decision into an exact lookup:

```text
(source_system, source_id) -> canonical_id
(crm, contact_731)          -> person_009
(billing, cus_1842)         -> person_009
```

Preserve the candidate-pair ID, reviewer, evidence reference, matching version, and approval time
on the mapping. Then agents can resolve aliases by stable index before they read or mutate related
records.

If an approved match later proves wrong, retire the mapping and record the correction. Do not
silently rewrite history. The [AI-agent audit-trail guide](/blog/ai-agent-audit-trail) shows how to
capture the actor, before and after values, reason, evidence, and result.

<a id="matching-methods"></a>
## Should you use exact, fuzzy, or probabilistic data matching?

Choose the simplest method that can meet the error tolerance of the workflow.

| Method | What it does | Good fit | Main limitation |
|---|---|---|---|
| Exact / deterministic | applies explicit agreement rules | stable IDs and clean fields | misses valid pairs when values differ |
| Fuzzy comparison | measures string or value similarity | names, addresses, and descriptions | similar text can describe different entities |
| Probabilistic | weighs agreements and disagreements | incomplete or noisy identifiers | thresholds and training need validation |
| Model-assisted review | explains or classifies candidate evidence | contextual edge cases | output can be persuasive without being correct |

The AHRQ overview notes that deterministic methods are straightforward when identifiers are
complete and reliable, while probabilistic methods can help when information is incomplete or
noisy. This is a tradeoff, not a maturity ladder. More complex matching is not automatically
better.

Use a cascade: exact rules first, then fuzzy or probabilistic methods for unresolved candidates,
then human review for consequential ambiguity. Every stage should produce evidence that the next
stage can inspect.

<a id="candidate-pair-schema"></a>
## What should a data-matching review dataset contain?

Use one row per candidate pair. Give the pair a deterministic index so retries update the same
decision instead of creating another candidate.

| Column | Purpose | Example |
|---|---|---|
| `match_id` | stable candidate-pair identity | `crm:contact_731|billing:cus_1842` |
| `left_system` / `left_id` | first source record | `crm` / `contact_731` |
| `right_system` / `right_id` | second source record | `billing` / `cus_1842` |
| `status` | decision state | `needs_review` |
| `method` | exact, fuzzy, probabilistic, or agent-assisted | `deterministic_then_agent` |
| `method_version` | reproducible rule or prompt version | `match-v3` |
| `evidence` | agreements and conflicts | `domain exact; address conflict` |
| `score` | optional model or linkage score | `0.86` |
| `canonical_id` | approved destination identity | blank until approved |
| `reviewed_by` / `reviewed_at` | decision provenance | `rasul` / timestamp |

The index must be order-stable. If `A|B` and `B|A` mean the same candidate, sort or otherwise
canonicalize the components before encoding them. The
[composite-key guide](/blog/composite-primary-key-ai-agents) covers collision-safe encoding for a
single-index interface.

<a id="data-matching-rowset"></a>
## How do you run a data-matching workflow with Rowset?

Rowset is the structured review and handoff layer, not a built-in entity-resolution engine. Your
agent, script, or matching library reads authorized sources and generates candidates. Rowset keeps
the candidate rows private, exposes them through authenticated MCP or REST, and stores the final
review state with stable indexes and durable instructions.

Use three datasets when the workflow is consequential:

1. **Source registry:** authorized systems, versions, and collection boundaries.
2. **Match candidates:** candidate pairs, evidence, method version, and review state.
3. **Identity crosswalk:** approved source aliases mapped to canonical IDs.

Before creating them, inspect the [schema-design guidance](/docs/design-schema) and choose a stable
index for each dataset. Put decision rules in dataset instructions. An agent using
[Rowset MCP access](/docs/connect-mcp) can inspect the schema and instructions, update a candidate
by exact index, and read it back after an uncertain response. The equivalent
[Dataset API](/docs/dataset-api) supports the same private row workflow over HTTP.

Keep public previews off for sensitive identity work. If a reviewer needs a human-readable export,
share only the minimum fields and follow the workflow's privacy policy. Public preview is not an
authentication mechanism.

<a id="evaluate-data-matching"></a>
## How do you evaluate data-matching quality?

Measure false approvals and missed matches separately. A workflow that approves nearly every pair
may have high coverage and dangerous precision. A workflow that approves only exact IDs may be
precise but leave useful matches unresolved.

Build a reviewed validation set that was not used to tune the rules. For each method version,
record:

- approved pairs that reviewers confirm are true matches
- approved pairs that reviewers find are false matches
- known matches that the workflow missed
- candidates routed to review
- decision changes after review

The AHRQ overview discusses sensitivity and positive predictive value as useful linkage measures
and emphasizes the tradeoff between missed matches and false matches. Choose the priority from the
business consequence. A research exploration may tolerate more candidates; an agent about to
change a customer's account should demand stronger evidence.

Do not publish a universal threshold such as “approve everything above 0.9.” Scores are specific to
the data, features, method, and validation set. Version the threshold and re-evaluate it when a
source schema or population changes.

<a id="data-matching-failures"></a>
## Common data-matching failures

### Merging before review

If the workflow overwrites or deletes source rows as soon as a candidate appears, a false match is
hard to unwind. Keep sources, candidates, decisions, and canonical mappings separate.

### Treating names as identity

Names change, collide, and vary by locale. Use them as evidence, never as the only operational key
for a consequential merge.

### Losing negative evidence

An exact name match can coexist with a conflicting region, birth date, supplier namespace, or
ownership field. Preserve disagreements; do not store only the features that support approval.

### Letting one agent propose and approve

The same model can repeat its initial mistake with a more confident explanation. Use deterministic
approval rules or an independent reviewer for uncertain cases.

### Re-running matches on every action

Matching belongs in a controlled workflow. Once approved, use the stable crosswalk. Repeating fuzzy
matching during each operational action produces inconsistent identity decisions.

<a id="data-matching-faq"></a>
## Data matching FAQ

### What is a data matching example?

Suppose billing stores `cus_1842` and a CRM stores `contact_731`. A matching workflow compares
authorized evidence, creates a candidate pair, and routes ambiguity to review. After approval, a
crosswalk maps both source IDs to `person_009`, which later agents resolve through exact lookup.

### Is data matching the same as record linkage?

The terms are often used interchangeably. Record linkage commonly emphasizes connecting records
across sources, while deduplication usually means finding repeated entities inside one dataset.
Entity resolution includes the decision and canonical-identity layer around those matches.

### Can an AI agent merge duplicate records automatically?

Only when a tested deterministic rule authorizes the merge and the consequence is acceptable. For
fuzzy, conflicting, or high-impact matches, let the agent propose candidates and require review
before changing operational identity.

### Does Rowset perform fuzzy matching?

No. Rowset stores private structured datasets, candidate evidence, review states, instructions,
and approved mappings. An external agent, script, or record-linkage library performs the matching.
This boundary keeps the matching method replaceable and the decisions inspectable.

### What is the safest first data-matching project?

Start with a small, non-sensitive dataset and a known validation sample. Run exact rules first,
send uncertain pairs to review, and test that rejected candidates never reach the operational
crosswalk. Rowset's [7-day hosted trial](/pricing) includes MCP and REST access for a private pilot.

Data matching should narrow uncertainty, not hide it. Materialize the evidence, assemble bounded
candidates, test each pair, confirm consequential decisions, and hand off exact canonical IDs only
after approval. That sequence lets an AI agent help with messy records without making silent
identity changes.
