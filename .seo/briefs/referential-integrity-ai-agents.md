# SEO brief: Referential Integrity for AI-Agent Datasets

- **Primary keyword:** referential integrity
- **Secondary terms:** referential integrity constraint, referential integrity violation,
  orphan records, foreign key constraint, data integrity
- **Intent:** informational / definitional / implementation
- **Type:** definition-led implementation guide
- **Target path:** `/blog/referential-integrity-ai-agents`
- **Research refreshed:** 2026-08-11
- **Measured signal:** 1,600 US searches/month, KD 4, informational intent (DataForSEO;
  keyword data updated July 2026)

## Selection and SERP read

The current US SERP has an AI Overview, People Also Ask, an IBM definition, database-vendor
explainers, Wikipedia, educational pages, and a Microsoft support result. Ranking pages define
foreign keys and delete actions. The gap is a mutation protocol for an AI agent operating across
separate tool calls, where a valid schema does not by itself guarantee correct write ordering,
retry recovery, or deliberate target deletion.

Shortlist:

| Rank | Candidate | Vol | KD | Intent | Score | Decision |
|---|---|---:|---:|---|---:|---|
| 1 | Referential integrity for AI-agent datasets | 1,600 | 4 | informational | 22 | Selected: highly winnable, product-native, and distinct from relationship modeling through its mutation and recovery focus. |
| 2 | Data reconciliation for AI-agent workflows | 720 | 0 | informational | 17 | Deferred: broad intent and substantial overlap with data cleaning, inventory reconciliation, and idempotent updates. |
| 3 | Data provenance for AI-agent records | 1,300 | 11 | informational | 16 | Deferred: credible but overlaps the data-collection and audit-trail guides; needs a narrower product surface. |
| 4 | Data lineage for AI-agent datasets | 2,400 | 11 | informational | 13 | Deferred: the head term expects lineage-platform capabilities Rowset does not provide. |

## Information gain

The post introduces the **VALID integrity contract** for multi-call agent mutations: Verify the
target, Add the parent first, Link the exact index, Inspect the stored result, and Delete
deliberately. It connects conventional foreign-key rules to agent-specific failure modes:
uncertain tool responses, retries, target-index changes, and repairs that must not guess.

## Table stakes and gap

Table stakes:

- define referential integrity directly
- explain parent rows, child rows, primary or unique keys, and foreign keys
- explain insert, update, and delete violations
- compare restrict/no-action, cascade, set-null, and set-default behavior
- distinguish referential integrity from broader data integrity
- answer the live PAA questions

Gap to fill:

- show why multi-call agent workflows need an operating contract beyond a schema
- prescribe parent-first writes and exact target-index lookup
- handle uncertain responses with read-back instead of duplicate or guessed repair
- make target deletion and index renaming deliberate operations
- show an audit path for existing orphan values before enabling enforcement
- map the workflow to Rowset relationships, stable indexes, MCP, and REST

## Entity and question map

- referential integrity and data integrity
- parent row, child row, primary key, unique key, foreign key
- orphan record and dangling reference
- insert, update, delete, and target-index change
- `NO ACTION`, `RESTRICT`, `CASCADE`, `SET NULL`, and `SET DEFAULT`
- nullable references and required references
- Rowset index column, relationships, enforcement, MCP, REST, and read-back
- What is meant by referential integrity?
- What happens if referential integrity is violated?
- What is the difference between data integrity and referential integrity?
- Should an AI agent create a missing parent row automatically?
- How do you enable enforcement on existing data?

## Verified claim ledger

| Claim | Source(s) | Tier / date | Status |
|---|---|---|---|
| A foreign key requires values in the referencing column or columns to match a row in the referenced table, maintaining referential integrity. | [PostgreSQL current constraints](https://www.postgresql.org/docs/current/ddl-constraints.html); [PostgreSQL foreign-key tutorial](https://www.postgresql.org/docs/current/tutorial-fk.html) | primary documentation; checked 2026-08-11 | verified |
| SQL foreign-key actions include `NO ACTION`, `RESTRICT`, `CASCADE`, `SET NULL`, and `SET DEFAULT`; the correct delete action depends on whether child rows can exist independently. | [PostgreSQL current constraints](https://www.postgresql.org/docs/current/ddl-constraints.html); [SQLite foreign keys](https://www.sqlite.org/foreignkeys.html) | two independent primary documentation sources; checked 2026-08-11 | verified |
| PostgreSQL documents `RESTRICT` as stricter than `NO ACTION`, while `CASCADE` propagates deletion to referencing rows. | [PostgreSQL current constraints](https://www.postgresql.org/docs/current/ddl-constraints.html) | primary documentation; checked 2026-08-11 | verified |
| Rowset relationships connect a source column to a target dataset's index; enforcement rejects non-blank missing targets while blanks remain allowed. | Repo docs: `/docs/link-datasets`; `apps/api/services.py`; relationship API tests inspected 2026-08-11 | primary product source | verified |
| Current Rowset enforcement blocks deletion of a referenced target row and blocks changing its referenced index value. | `apps/api/services.py`; `apps/datasets/tests/test_dataset_relationships.py` inspected 2026-08-11; documented in this PR | primary product implementation and tests | verified |
| Rowset agents can inspect relationship metadata with `get_dataset`, resolve links through MCP or REST, and use stable indexes for exact row operations. | Repo docs: `/docs/link-datasets`, `/docs/work-with-rows`, `/docs/dataset-api`; current MCP tool descriptions | primary product source | verified |

## Product-led SEO check

- **User job:** keep agent-written references valid while records are created, retried, renamed,
  or deleted.
- **Product surface:** Rowset dataset relationships with stable target indexes, optional
  enforcement, exact lookup, MCP/REST resolution, and private instructions.
- **Credible angle:** Rowset already enforces relationship writes and protects referenced target
  identity; the article explains how agents should operate that surface safely.
- **Business job:** lead readers from an integrity problem to relationship docs, MCP setup,
  Dataset API, quickstart, and pricing.
- **Moat:** the VALID contract and Rowset-specific failure/recovery workflow are more useful than
  a generic database definition.

## AI SEO check

- 49-word opening definition with direct operational recommendation
- self-contained VALID contract and delete-action decision table
- question-shaped headings based on current PAA
- only primary-source claims, checked and dated August 2026
- concrete parent/child example, Rowset implementation, audit checklist, and FAQ
- entity coverage for parent, child, keys, violations, actions, enforcement, and repair
- existing renderer emits `BlogPosting`, author, canonical, `datePublished`, and `dateModified`;
  the FAQ remains semantic HTML because the current content renderer does not emit `FAQPage`

