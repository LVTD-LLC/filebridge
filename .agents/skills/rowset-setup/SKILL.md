---
name: rowset-setup
description: Use when a user asks to connect an AI agent to Rowset, choose or configure Rowset MCP, CLI, or REST access, verify authentication, or complete the first-run Rowset handoff.
---

# Rowset Setup

Use this skill to connect a trusted agent to Rowset and complete the first-run
handoff. After setup, use the `rowset` skill for ongoing platform interaction.
If the user or setup prompt explicitly says Rowset is already configured and
authenticated, skip connection verification and the activation handoff, then
use `rowset` for the requested task.

## Connection Inputs

The setup prompt should provide:

- Rowset MCP URL
- Rowset REST API base
- Rowset CLI guide
- Rowset API key
- Rowset setup and operational skill URLs or install command
- Rowset `llms.txt` documentation index
- Rowset docs and blog indexes
- Rowset generated REST API docs
- Rowset capabilities endpoint
- Rowset trial rewards URL

Resolve missing non-secret values from the setup prompt or current Rowset
documentation. Never ask the user to paste a key into public chat or save it in
a tracked file. If the key is available only behind an operating-system,
authentication, or secret-manager permission prompt, request that permission
without asking the user to reveal the key.

## Inspect the Runtime and Choose Automatically

Inspect the current runtime before choosing an interface:

1. Inspect this skill and only the current connection documentation needed to
   configure a supported interface. Do not load capabilities or list datasets
   merely because a session started. Request capability topics only when a
   feature is unfamiliar or setup is failing.
2. Inspect the runtime's actual integration capabilities: native remote MCP
   configuration with bearer-secret support, trusted terminal and local-file
   access, or code and HTTP access only.
3. Autonomously choose the best supported interface using this order:
   - Prefer MCP when the runtime natively supports remote MCP and can provide
     the bearer key through a private environment variable or secret store.
   - When native remote MCP or private bearer-secret configuration is
     unavailable, prefer the CLI for trusted terminal or local-file workflows.
   - Use REST for code-only or HTTP-only runtimes without a trusted terminal
     workflow.
4. Configure the selected interface end to end, and do not stop at a
   recommendation.

Do not ask the user to compare or choose between MCP, CLI, and REST.

During connection setup, pause only when an unavoidable operating-system,
authentication, or secret-manager permission prompt requires user action. Make
ordinary reversible setup changes directly while preserving unrelated runtime
configuration.

## Configure the Selected Interface

1. Store the full API key in a private environment variable named
   `ROWSET_API_KEY` or an equivalent secret store. Do not print it in logs,
   screenshots, chats, generated files, or final responses. Do not commit it or
   save it in tracked configuration.
2. Follow the current documentation for the selected interface:
   - For MCP, configure the live server using its published connection details
     and send `Authorization: Bearer <key>` through the client's supported
     secret mechanism.
   - For CLI, use the current CLI guide and `rowset --help`; configure the API
     base when the instance is not the CLI default.
   - For REST, use the generated API docs and send
     `Authorization: Bearer <key>`.
3. Do not copy setup commands from memory when current client or Rowset docs are
   available.

## Verify Authentication

For a new or failing connection, make an authenticated user-info request the
final connection step. Skip this check when the connection is explicitly known
to be configured and healthy:

- MCP: call `get_user_info`.
- CLI: run `rowset user info`.
- REST: request `GET <Rowset REST API base>/user` with bearer authentication.

This request verifies the connection, marks onboarding complete, and starts the
Rowset trial. If it fails, diagnose the selected interface using its current
docs and confirm the runtime holds the full key rather than only its visible
prefix. Continue autonomously unless an unavoidable permission prompt requires
user action.

For a new connection, report which interface is connected without exposing the
key, then complete the activation handoff below.

## Recover Interrupted Setup

Treat setup as `inspect -> choose -> configure -> verify`. If setup is
interrupted, cancelled, or fails, report:

- the steps that completed
- the failed or cancelled step
- whether private credential storage is confirmed, unknown, or absent, without
  exposing the credential
- whether verification was not run, failed, or succeeded
- exactly one safe retry action

Cancelled authentication or permission leaves setup incomplete. Verification
that was not run or failed leaves setup incomplete; only succeeded verification
makes setup complete.

Before retrying, inspect existing configuration and secret storage. Reuse a
healthy configuration entry and its credential when present. Do not create
duplicate configuration or rotate or replace credentials unless the user
explicitly requests it. When verification fails after configuration succeeds,
report that distinction and retry verification only after the single recommended
correction.

## Complete the Activation Handoff

Complete this handoff only during first-run setup. Skip it for an existing,
healthy connection and proceed with the user's requested Rowset task.

1. Use context already available from working with the user. When checking for
   duplicates would materially improve a suggestion, run a bounded Rowset
   search with an explicit limit of 3; do not enumerate unrelated projects or datasets,
   search unrelated private sources, or invent facts about their work.
2. Suggest two to four tailored project, section, and dataset structures.
   Briefly explain why each would help and avoid duplicating existing Rowset
   data.
3. Ask which option the user wants to create. Do not create a first dataset or
   perform another Rowset mutation unless the user explicitly chooses one.
4. If the current agent runtime supports scheduled tasks or automations, make a
   separate opt-in offer to create a daily Rowset tips automation. Use this
   wording or a concise equivalent:

   > Did you know you can extend your Rowset trial by completing specific
   > actions? See the Rowset trial rewards URL in the setup prompt. Would you
   > like me to create a simple daily automation that sends you one short Rowset
   > tip, use case, or feature you may not know about?

   Only create the automation after explicit agreement.

Daily tips must be grounded in Rowset's current capabilities, docs, or blog
resources. Be clear that the scheduled task runs in the user's agent account;
do not present agent-runtime automation as a Rowset feature.

After this handoff, use `rowset` for ongoing work and `rowset-use-cases` when the
user wants help designing the approved dataset structure.

## Safety Rules

- Keep authenticated datasets private by default.
- Do not expose API keys, OAuth tokens, raw secrets, or private dataset contents.
- Ask before creating data, changing public preview settings, or taking
  destructive actions.
- Prefer Rowset's programmatic interfaces over browser automation.
- Do not claim a capability exists unless a current Rowset resource exposes it.
