---
title: "MCP OAuth vs API Keys: Choose the Right Auth"
description: "Compare MCP OAuth with API keys and choose an authorization model for trusted agents or delegated user access."
published_at: 2026-08-02
updated_at: 2026-08-02
author: Rasul Kireev
keywords:
  - MCP OAuth
  - MCP API key
  - MCP authentication
  - MCP authorization
topics:
  - MCP
  - agent access
  - security
canonical_url: https://rowset.lvtd.dev/blog/mcp-oauth-vs-api-keys
image: /static/vendors/images/logo.png
image_alt: Rowset logo
robots: index, follow
---

Choose MCP OAuth when a distinct user must grant a separately operated client access to that
user's data. Choose a provisioned API key when one operator controls the account, the agent, and
the credential lifecycle inside the same trust boundary. Both may use an HTTP `Bearer` header;
the difference is who grants authority and how that authority expires.

Use this quick decision table:

| Situation | Better starting point | Why |
|---|---|---|
| One operator connects a known internal agent | Provisioned API key | Direct setup, no interactive consent flow |
| Many users connect third-party MCP clients | MCP OAuth | Per-user consent, scopes, discovery, and token lifecycle |
| Local STDIO server reads credentials from its runtime | Environment credential | This is the current MCP guidance for local STDIO servers |
| Remote service needs user-specific identity and revocation | MCP OAuth | Access can be tied to a user, client, resource, and grant |
| Scheduled worker runs under one service owner | Provisioned key or workload identity | There may be no person available for an authorization-code flow |

The useful question is not "Does it send a bearer token?" OAuth access tokens and API keys can
both appear as `Authorization: Bearer <credential>`. Ask whether the connection represents a
delegation from an end user to an independently operated client. That is the boundary this guide
calls the **delegation test**.

## In this guide

- [What MCP OAuth standardizes](#what-mcp-oauth-standardizes)
- [Run the five-question delegation test](#delegation-test)
- [Compare MCP OAuth and API keys](#oauth-vs-api-keys)
- [Know when an API key is enough](#when-api-keys-fit)
- [Know when MCP OAuth is the right requirement](#when-oauth-fits)
- [Avoid security shortcuts in both models](#shared-security-controls)
- [Understand where Rowset fits](#where-rowset-fits)
- [MCP OAuth FAQ](#mcp-oauth-faq)

<a id="what-mcp-oauth-standardizes"></a>
## What does MCP OAuth actually standardize?

MCP OAuth standardizes how a remote HTTP MCP client discovers an authorization server, obtains
user-approved access, requests a token for the intended MCP resource, and presents that token to
the server. It does not make every MCP server use OAuth, and it does not replace the server's own
permission checks.

The current MCP authorization specification makes authorization optional for MCP
implementations. When an HTTP-based implementation supports authorization, it should conform to
the MCP profile. Local STDIO implementations should instead obtain credentials from their
environment
([MCP authorization specification, checked August 2026](https://modelcontextprotocol.io/specification/draft/basic/authorization)).

For a protected remote server, the current flow has these parts:

1. The client calls the MCP server without a usable access token.
2. The server returns `401 Unauthorized` and points to protected resource metadata.
3. The client reads that metadata to find the permitted authorization server and scopes.
4. The client discovers the authorization server's endpoints and capabilities.
5. A user signs in and approves an authorization request, normally through an authorization-code
   flow protected by PKCE.
6. The client exchanges the code for an access token intended for the MCP server.
7. The client retries the MCP request with `Authorization: Bearer <access-token>`.
8. The MCP server validates the token, resource audience, expiry, and required permissions.

The [official MCP authorization tutorial, updated July 28,
2026](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/authorization)
shows the full discovery and token flow. The specification currently requires authorization
servers in this profile to implement OAuth 2.1, requires MCP servers to publish OAuth protected
resource metadata, and requires clients to send the OAuth `resource` parameter.

That resource parameter matters. It lets the authorization server issue a token for the intended
MCP server rather than a generic token that might be replayed against another service. RFC 8707
recommends using the most specific practical resource URI and audience-restricting the issued
token
([IETF RFC 8707](https://datatracker.ietf.org/doc/html/rfc8707)). The MCP specification also says
a server must not accept or transit a token that is not valid for its own resource.

<a id="delegation-test"></a>
## Run the five-question delegation test

The delegation test separates a real OAuth requirement from a general need for private access.
Answer these five questions before choosing an implementation.

### 1. Who grants the access?

Use OAuth when an end user needs to grant a client access to the user's resources without handing
the client the user's long-lived account credential. The authorization grant represents that
delegation.

Use a provisioned key when the account operator is also the person authorizing the known agent.
The operator creates a separate credential for the agent, chooses its permission level, stores it
in the runtime, and revokes or rotates it when the relationship ends.

### 2. Who controls the client?

OAuth earns its complexity when clients and resource servers have different operators. A SaaS
customer may connect an MCP client built by one company to data hosted by another. The resource
server cannot safely treat every client as preconfigured, and the client should not ask the user
to paste a permanent service credential into an arbitrary prompt.

If the same team controls the MCP server, agent runtime, and deployment secret store, a
provisioned key can be a simpler trust contract. Simpler does not mean public or unbounded: issue a
separate key, limit its permissions, and keep it out of prompts, logs, URLs, and source control.

### 3. Is an interactive user available?

The authorization-code flow assumes a browser or user-agent step in which a person signs in and
approves access. PKCE then binds the returned authorization code to the client instance that
started the flow. OAuth security guidance explains that a stolen code cannot be redeemed without
the matching verifier
([IETF OAuth 2.0 Security Best Current Practice, RFC 9700](https://www.rfc-editor.org/rfc/rfc9700)).

A scheduled worker, server-side agent, or isolated automation may have no person available when a
run starts. That does not automatically justify a permanent unrestricted secret. It means the
workload needs a non-interactive credential strategy: a narrowly permissioned provisioned key,
short-lived workload token, or another machine-to-machine mechanism supported by the service.

### 4. Whose identity must appear in the audit trail?

If the system must distinguish Alice authorizing Client A from Bob authorizing Client B, use a
model that carries user and client identity through token issuance and validation. OAuth scopes,
resource indicators, token subject, client identity, and grant records can support that
separation when implemented correctly.

A shared API key cannot prove which person initiated a request. A separately issued key can
identify a particular integration or agent, but the server only knows the key's assigned identity.
If several people or processes share it, their actions collapse into one principal unless the
application adds stronger authenticated context.

### 5. How must access end?

Write the end condition before the connection starts. OAuth commonly uses expiring access tokens,
refresh-token policy, grant revocation, and per-client consent records. A provisioned key normally
remains usable until it expires, is rotated, is disabled, or the server changes its permissions.

The practical comparison is not "revocable versus permanent." Both can be revocable. OAuth gives
you a standardized delegated grant and short-lived token lifecycle; an API-key design needs its
own issuance, visibility, rotation, and revocation controls.

<a id="oauth-vs-api-keys"></a>
## MCP OAuth vs API keys: what changes?

| Dimension | MCP OAuth | Provisioned API key |
|---|---|---|
| Authority source | User-approved grant through an authorization server | Operator creates a credential directly |
| Typical client relationship | Third-party or separately operated client | Known agent, integration, or internal worker |
| Discovery | Protected-resource and authorization-server metadata | Configuration supplied out of band |
| User interaction | Usually browser login and consent | Usually none after provisioning |
| Credential sent to MCP | Short-lived access token, often with refresh flow | API key until expiry, rotation, or revocation |
| Resource binding | MCP profile requires resource indicators and server-side validation | Depends on the service's key design |
| Permission model | OAuth scopes plus server-side policy | Permission fields attached to the key plus server-side policy |
| Client onboarding | Metadata, client ID documents, pre-registration, or compatible registration | Copy key into an approved secret store |
| Audit identity | Can distinguish resource owner, client, and token grant | Identifies the provisioned key/integration |
| Main operational cost | Authorization server, consent UX, metadata, token lifecycle | Secure provisioning, storage, rotation, and key inventory |

Do not infer the model from token syntax. A JWT is one possible OAuth access-token format, not a
requirement for every access token. An API key can also be presented with the Bearer scheme. The
server must validate the credential according to the system that issued it and enforce
permissions on every operation.

<a id="when-api-keys-fit"></a>
## When is an API key enough for an MCP server?

A provisioned API key is a reasonable starting point when all of these conditions hold:

- one operator or team controls the service account and the agent deployment
- the client is known before access is granted
- no third-party client needs an end user's delegated consent
- the service can issue a separate key with the smallest useful permissions
- the runtime has a real secret store or private environment configuration
- the operator can inventory, rotate, revoke, and audit the key

This often describes an internal assistant, a founder-operated automation, a scheduled agent, or
a self-hosted workflow. It does not describe a public integration where arbitrary users connect
arbitrary clients to their private accounts.

Treat the API key as the agent's credential, not the human's password. Do not reuse an owner
session cookie or paste a broad personal credential into the model context. Provisioning a
separate key lets the service attach an explicit permission level and revoke the agent without
changing the user's login.

If you are still deciding whether the agent should connect over a discoverable tool protocol or a
plain HTTP API, use the [MCP versus REST decision guide](/blog/mcp-vs-rest-ai-agents). The auth
model and interface choice are related, but they are not the same decision.

<a id="when-oauth-fits"></a>
## When should a remote MCP server require OAuth?

Require MCP OAuth when the product needs delegated, user-specific access across an administrative
boundary. Strong signals include:

- many customers connect their own MCP clients
- the client operator is different from the MCP server operator
- each user must see and approve requested permissions
- the server must distinguish users and clients in policy and audit records
- access should expire without rotating a shared service credential
- the product already has an authorization server or identity platform that can issue valid
  resource-bound tokens

The official MCP tutorial strongly recommends authorization when a server accesses user-specific
data, needs per-user auditing or rate limits, exposes APIs requiring consent, or operates in an
enterprise environment. OAuth is not decoration around an API key. A conforming implementation
needs protected-resource metadata, authorization-server discovery, client onboarding, PKCE,
resource indicators, token validation, scopes, and correct `401`/`403` challenges.

Do not build a half-flow that accepts tokens minted for another service. The MCP specification
requires servers to accept only tokens valid for their own resource. This prevents token
passthrough, where a server receives a token for an upstream API and forwards or misuses it as if
it were the intended audience.

<a id="shared-security-controls"></a>
## What security controls do both models need?

OAuth and API keys both fail if the server treats possession of any valid credential as permission
to do everything. Keep these controls outside the model prompt:

1. **Validate every request.** Check the credential, active state, intended resource, permission,
   and requested object or account boundary.
2. **Use least privilege.** Separate read, write, and administrative access. Do not give a research
   agent permission to delete rows or create more credentials.
3. **Keep secrets out of content.** Store credentials in the client runtime or a secret manager,
   never in prompts, dataset rows, screenshots, analytics, or source control.
4. **Log safe identifiers.** Record the principal, client or key ID, requested operation, result,
   and correlation ID without logging the raw credential.
5. **Plan revocation and recovery.** Know how to disable the grant or key, how quickly cached
   access ends, and what the agent should do after a `401` or insufficient-scope response.
6. **Require separate approval for consequential actions.** Authentication proves who or what is
   calling. It does not prove that a deletion, payment, publication, or external message is
   appropriate now.

The [safe agent-data sharing guide](/blog/share-ai-agent-data-safely) covers the boundary between
private MCP or REST access and optional read-only public previews. A public URL or preview password
is not a substitute for private agent authentication.

<a id="where-rowset-fits"></a>
## Where does Rowset fit in the MCP OAuth decision?

Rowset currently uses provisioned bearer API keys for hosted MCP, REST, and CLI access. It does not
claim to implement the MCP OAuth authorization flow. The account operator creates a key, stores it
in the trusted agent runtime, and connects to Rowset's hosted MCP endpoint. The
[MCP setup guide](/docs/connect-mcp) shows the current configuration.

Rowset keys have three permission levels: Read, Read + write, and Admin. These permissions are
account-wide rather than dataset-specific. Read + write includes destructive dataset tools, so a
narrow agent runtime should filter the available tools and ask before destructive actions. Use
Admin only when the trusted automation must create other agent keys. The
[agent-access guide](/docs/configure-agent-access) documents those boundaries.

That model fits a user who owns the Rowset account and deliberately provisions a trusted agent.
It is not a replacement for delegated OAuth when a separate end user needs to authorize a
third-party client. If your application needs that relationship, put an OAuth-capable service or
your own conforming authorization layer in front of the appropriate resource instead of
describing a copied Rowset key as user consent.

If the provisioned-key model matches your trust boundary, you can [start a 7-day Rowset
trial](/pricing), create the smallest useful key permission, and verify the connection before the
agent changes data.

<a id="mcp-oauth-faq"></a>
## MCP OAuth FAQ

### Is OAuth required for every MCP server?

No. The current MCP specification says authorization is optional. HTTP implementations that
support authorization should conform to the MCP authorization profile. Local STDIO servers should
obtain credentials from their environment rather than implement the remote HTTP OAuth profile.

### What is MCP authentication?

MCP authentication is the process by which a server establishes the identity represented by a
credential. The MCP authorization profile focuses on obtaining and validating OAuth access to a
protected resource. Authentication and authorization are related, but a valid identity still
needs server-side permission checks for the requested tool, resource, and user data.

### How does MCP use JWT?

An OAuth authorization server may issue a JWT access token, and an MCP server may validate its
signature and claims such as issuer, audience, expiry, and scopes. JWT is not mandatory for every
MCP credential. The server can also validate an opaque access token through the issuing
authorization system.

### Can an MCP API key use the Bearer header?

Yes. A service can accept an API key as an HTTP bearer credential. That header format does not
make the key an OAuth token. OAuth adds a standardized grant, discovery, resource targeting,
consent, scope, and token lifecycle around the bearer access token.

### Should a local MCP server use OAuth?

Usually not for the local STDIO connection itself. Current MCP guidance says STDIO servers should
retrieve credentials from the environment. A local server may still use OAuth to access an
upstream service on the user's behalf, but that is a separate authorization relationship.

## The decision in one sentence

Use MCP OAuth for delegated access across a user-client-service boundary. Use a provisioned API
key for a known trusted agent inside one operator's boundary, then apply least privilege, private
secret storage, explicit revocation, and server-side authorization either way.
