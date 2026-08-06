# Composite Primary Keys for AI-Agent Datasets

## Selection

- Target: `composite primary key`
- Type: definition / implementation decision guide
- Live US metrics checked 2026-08-06: volume 590, KD 0, informational intent
- SERP shape: AI Overview/PAA plus definition and implementation tutorials; primary documentation
  from PostgreSQL, Django, and Rails is prominent.
- Product-led fit: Rowset exposes one stable index column for MCP/REST by-index operations, so users
  with multi-field source identity need a deterministic adapter rather than generic SQL advice.

## Information gain

The top results explain native database composite keys. This piece adds the SCOPE identity contract:
Scope, Components, Ownership, Percent-encode, and Evolution. It shows how to preserve a durable
identity tuple as one reversible Rowset index without claiming native multi-column key support.

## Table stakes and gap

Table stakes: definition, uniqueness/non-null behavior, examples, when composite identity fits,
tradeoffs with surrogate IDs, implementation example, and FAQ.

Gap: deterministic construction at an agent tool boundary, delimiter-collision prevention,
component authority, encoding-version migration, retry behavior, and Rowset relationship semantics.

## Entity map

Composite primary key, primary key, business key, surrogate key, unique constraint, non-null,
multi-column key, index column, deterministic encoding, percent-encoding, UTF-8, component order,
normalization, tenant/workspace scope, idempotency, relationship, foreign key, MCP, REST, `rowset_id`.

Questions: What is a composite primary key? Can a primary key contain multiple columns? When should
you use a composite key? Is it better than a surrogate key? How should an agent encode a composite
identity when an API accepts one index value?

## Verified claim ledger

| Claim | Source | Tier / date | Verification |
|---|---|---|---|
| PostgreSQL primary keys may span multiple columns; the group must be unique and non-null. | https://www.postgresql.org/docs/current/ddl-constraints.html | primary, checked 2026-08-06 | verified primary |
| Django supports `CompositePrimaryKey`, while current docs list migration and relationship limitations. | https://docs.djangoproject.com/en/6.0/topics/composite-primary-key/ | primary, checked 2026-08-06 | verified primary; release introduction cross-checked at https://www.djangoproject.com/weblog/2025/apr/02/django-52-released/ |
| Percent-encoding represents data characters that would otherwise conflict with URI syntax. | https://www.rfc-editor.org/rfc/rfc3986 | primary, January 2005 | verified primary |
| RFC 8785 defines invariant JSON serialization for repeatable cryptographic operations. | https://www.rfc-editor.org/rfc/rfc8785 | primary, June 2020 | verified primary |
| Rowset accepts one explicit `index_column`, generates `rowset_id` when omitted, and supports by-index operations. | `apps/pages/content/docs/dataset-api.md`; `TECH.md` | product source, checked 2026-08-06 | verified in docs and implementation |
| Rowset relationships store target dataset index values. | `apps/pages/content/docs/link-datasets.md`; `apps/pages/content/docs/dataset-api.md` | product source, checked 2026-08-06 | verified in two repo docs |
| The SCOPE contract and `ck1` encoding are this article's proposed framework, not a Rowset protocol guarantee. | article methodology | original analysis, 2026-08-06 | clearly framed as recommendation |

## Side checks

- AI SEO: direct definition first; self-contained answer blocks; current primary sources; explicit
  entities; visible date through frontmatter; FAQ; renderer-supported `BlogPosting` schema.
- Product-led SEO: solves exact Rowset lookup, retry, and relationship work; links to the Dataset
  API, schema design, identity migration, idempotency, relationships, quickstart, and pricing;
  states that Rowset does not expose native multi-column SQL keys.
