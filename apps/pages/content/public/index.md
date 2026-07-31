# Rowset: one backend for AI agent workflows

Rowset is the open-source, self-hostable backend for AI agent workflows. It gives agents private,
searchable structured data through MCP, REST, or CLI without requiring you to build another backend.

## What agents can do

- Store typed, indexed rows with stable business keys or generated `rowset_id` values.
- Find a known row exactly or search by meaning with semantic search.
- Connect through hosted MCP, REST, or the Rowset CLI.
- Export snapshots as CSV, JSONL, XLSX, SQLite, or Parquet.
- Share an optional read-only public preview when people need browser access.

Datasets are private by default. Authenticated agent access uses a bearer API key; public previews
are an explicit, read-only sharing option rather than an authentication method.

## Start in three steps

1. [Create an account]({{ signup_url }}) and copy the Rowset setup prompt.
2. Let the agent inspect its runtime and automatically configure the best supported interface.
3. Using already-authorized context, the agent recommends one high-confidence project with one to
   three concrete datasets, then asks, "Would you like me to create that now?"

If there is not enough context, the agent asks, "What are you working on right now?" instead of
inventing a generic starter. The agent does not create the recommended project or datasets until
you say yes. After you say yes, it creates or reuses the private project and datasets, verifies
their schemas and stable indexes, and reports what is ready. On a negative answer, create nothing.
You do not need to compare MCP, CLI, and REST or design the first dataset structure yourself.

Start with [the quickstart]({{ site_url }}/docs/quickstart) or explore
[Rowset use cases]({{ site_url }}/use-cases).

The full product is available in a 7-day trial. [See pricing]({{ site_url }}/pricing).
