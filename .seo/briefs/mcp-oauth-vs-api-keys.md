# SEO Brief: MCP OAuth vs API Keys

## Selection

- **Title:** MCP OAuth vs API Keys: Choose the Right Auth
- **Slug:** `/blog/mcp-oauth-vs-api-keys`
- **Primary keyword:** `mcp oauth`
- **Measured demand:** 390 US searches/month, KD 9, CPC $38.48 (DataForSEO, 2026-08-02)
- **Intent:** Navigational with a practical implementation/decision sub-intent
- **Type:** Comparison and decision guide
- **Product-led reason:** The query maps directly to Rowset's hosted MCP setup and its current
  bearer API-key boundary. The post helps an operator decide whether Rowset's provisioned-key
  model fits a trusted agent or whether the application needs delegated OAuth instead.

## SERP teardown

The live US SERP is led by the current MCP authorization tutorial and draft specification,
followed by vendor explainers, an implementation discussion, and OAuth implementation guides.
Table stakes are OAuth roles, protected-resource metadata, authorization-server discovery, PKCE,
scopes, token validation, and revocation. The gap is a neutral decision model for OAuth versus a
provisioned API key, especially for a known operator and trusted non-human agent.

## Information gain

The article introduces the **delegation test**, a five-question framework that separates the
credential format from the trust model. It asks who grants access, who controls the client,
whether interactive consent is required, what identity must be audited, and how access must end.
This framework is absent from the reviewed top results, which mostly explain how to implement
OAuth.

## Entity and question map

- Model Context Protocol (MCP)
- OAuth 2.1
- bearer token and API key
- protected resource metadata (RFC 9728)
- authorization-server discovery
- resource indicators and audience restriction (RFC 8707)
- authorization code flow and PKCE
- scopes, consent, expiration, rotation, and revocation
- remote HTTP MCP versus local STDIO MCP
- MCP client, protected resource, authorization server, resource owner
- PAA: What is MCP authentication? How does MCP use JWT? Is OAuth required for MCP? What should
  local MCP servers use?

## Claim ledger

| ID | Claim | Primary source | Independent check / locator | Status |
|---|---|---|---|---|
| auth-01 | MCP authorization is optional at the protocol level; HTTP implementations that support it should conform, while STDIO implementations should use environment credentials. | https://modelcontextprotocol.io/specification/draft/basic/authorization | https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/authorization | verified |
| auth-02 | Under the current MCP authorization profile, authorization servers must implement OAuth 2.1 and MCP servers must publish protected resource metadata. | https://modelcontextprotocol.io/specification/draft/basic/authorization | https://www.rfc-editor.org/rfc/rfc9728 | verified |
| auth-03 | MCP clients must use the OAuth resource parameter, and servers must reject tokens not valid for their own resource. | https://modelcontextprotocol.io/specification/draft/basic/authorization | https://datatracker.ietf.org/doc/html/rfc8707 | verified |
| auth-04 | The MCP OAuth flow still sends the resulting access token in the HTTP `Authorization: Bearer` header. | https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/authorization | https://modelcontextprotocol.io/specification/draft/basic/authorization | verified |
| auth-05 | PKCE binds an authorization code to the client instance and prevents a stolen code from being redeemed without the verifier. | https://www.rfc-editor.org/rfc/rfc9700 | https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/authorization | verified |
| auth-06 | Rowset's hosted MCP currently uses provisioned bearer API keys with Read, Read + write, and Admin permission levels; permissions are account-wide. | https://rowset.lvtd.dev/docs/connect-mcp | `apps/pages/content/docs/connect-mcp.md` and `apps/pages/content/docs/api-overview.md` | verified |
| auth-07 | Rowset does not currently claim the MCP OAuth authorization flow; its key is created by the account operator and stored in the trusted agent runtime. | https://rowset.lvtd.dev/docs/configure-agent-access | `.seo/brand.md`, repo `AGENTS.md`, and live setup docs | verified |

## Counter-evidence and limits

- OAuth is not automatically required for every MCP implementation; the current specification
  explicitly makes authorization optional.
- API keys are not automatically insecure. A random, revocable, permissioned key kept in a secret
  store can be appropriate inside one administrative trust boundary.
- OAuth does not remove the need for server-side permission checks, correct audience validation,
  secret-safe logging, or approval for consequential actions.
- Rowset's current key permissions are account-wide, not dataset-specific. Agent runtimes should
  filter tools and use the smallest available key permission.

## Internal-link plan

- `/docs/connect-mcp` — hosted MCP setup
- `/docs/configure-agent-access` — API-key permissions and setup prompt
- `/blog/mcp-vs-rest-ai-agents` — protocol/interface choice
- `/blog/share-ai-agent-data-safely` — private access and sharing boundary
- `/pricing` — trial and product next step

Inbound links will be added from `/docs/connect-mcp` and
`/blog/share-ai-agent-data-safely`.

## Side checks

- **AI SEO:** Direct answer first; self-contained comparison table; current dated protocol links;
  PAA-shaped FAQ; clear entities; existing renderer emits `BlogPosting` schema and freshness fields.
- **Product-led SEO:** The post solves an access-model decision tied to Rowset's actual MCP setup,
  states the product boundary, links to the setup surface, and qualifies rather than overclaims fit.
