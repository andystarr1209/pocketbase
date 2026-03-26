## Plan: VK ID Backend OAuth2 Integration

TL;DR: Implement VK ID (id.vk.ru) OAuth 2.1 Authorization Code + PKCE backend code-exchange in the existing VK provider implementation. Update `tools/auth/vk.go` to: enable PKCE, build VK ID authorization URLs, exchange authorization codes via `POST https://id.vk.ru/oauth2/auth` with `application/x-www-form-urlencoded` body, call `POST https://id.vk.ru/oauth2/user_info` with a Bearer token to fetch profile, handle refresh and revoke flows, and validate `state` and `device_id` per security requirements.

**Steps**
1. Discovery: review current `tools/auth/vk.go` provider implementation to find differences with VK ID flow (PKCE, endpoints, token response fields). *depends on step 2*
2. Replace endpoints and defaults in `tools/auth/vk.go`:
   - Change `authURL` to `https://id.vk.ru/authorize`
   - Change `tokenURL` to `https://id.vk.ru/oauth2/auth`
   - Change `userInfoURL` to `https://id.vk.ru/oauth2/user_info`
   - Set `pkce: true` and default `scopes` to include `vkid.personal_info` plus optional `email`/`phone`.
3. Authorization URL builder (frontend redirect): implement a method (or reuse existing) that constructs the URL with required params: `response_type=code`, `client_id`, `redirect_uri`, `code_challenge` (S256), `code_challenge_method=S256`, `state`, optional `scope`, and optional `lang_id`/`scheme`.
   - Document that `state` is generated per-request and must be validated on callback.
   - Ensure `code_verifier` generation guidance is added where the client initiates the flow (the repo-side provider should expose a way to create `code_challenge` and return `code_verifier` to the frontend if applicable).
4. Callback handling changes:
   - Verify `state` matches stored value (CSRF protection).
   - Extract `code` and `device_id` from the redirect query params.
   - Do not run any scripts or render embedded resources on the redirect page (document requirement).
5. Server-side token exchange implementation in `tools/auth/vk.go`:
   - Perform `POST https://id.vk.ru/oauth2/auth` with `Content-Type: application/x-www-form-urlencoded` and body parameters: `grant_type=authorization_code`, `code`, `client_id`, `redirect_uri`, `code_verifier`, `device_id`, `state`.
   - Parse returned JSON: `access_token`, `refresh_token`, `id_token`, `expires_in`, `token_type`.
   - On error responses, map VK errors (`invalid_grant`, etc.) to repository `Auth` error types.
6. Fetch user profile:
   - `POST https://id.vk.ru/oauth2/user_info` with header `Authorization: Bearer <access_token>` and parse `user` fields into the repository's `AuthUser` structure (map `user.id`, `user.first_name`, `user.last_name`, `user.avatar`, `user.email` when present).
7. Refresh token flow:
   - Implement `refresh` method to `POST https://id.vk.ru/oauth2/auth` with `grant_type=refresh_token`, `refresh_token`, `client_id`, `device_id`, `state` and update stored tokens.
8. Revoke / logout:
   - Implement a method that `POST https://id.vk.ru/oauth2/logout` with `Authorization: Bearer <access_token>` to revoke session server-side.
9. Error handling and security:
   - Validate `state` before exchanging the `code`.
   - Treat `id_token` as OIDC assertion; parse if needed for additional claims.
   - Ensure `code_verifier` is not logged or persisted insecurely.
   - Fail fast on missing `device_id` where required.
10. Tests and verification:
   - Add unit tests for URL builder, token exchange (mock HTTP), userinfo parsing, and refresh flow. Target new tests under `tools/auth` (e.g., `tools/auth/vk_test.go`).
   - Manual test checklist: register app in VK ID console with exact `redirect_uri`, initiate flow in browser, exchange code on backend via implemented endpoint, and verify tokens and user info.
11. Documentation and examples:
   - Update README or provider docs describing required frontend steps (PKCE `code_verifier` storage, `state` generation) and the required clean redirect page constraints.

**Relevant files**
- [tools/auth/vk.go](tools/auth/vk.go) — modify to implement VK ID flow (authorization, token exchange, userinfo, refresh, revoke).
- `tools/auth/vk_test.go` — new tests for the provider (create).
- (Optional) docs: `.github/prompts/vkid_integration_reference.prompt.md` — reference; add README section if present.

**Verification**
1. Automated: write unit tests that mock external HTTP calls to `https://id.vk.ru/oauth2/auth` and `https://id.vk.ru/oauth2/user_info` and assert correct parameter encoding (form body) and parsed outputs.
2. Local manual: register a VK ID app, set `redirect_uri`, run a local server exposing the callback, perform full auth flow, confirm backend receives `code`+`device_id`, exchanges tokens successfully, and user profile maps correctly.
3. Security checks: ensure `state` validation prevents CSRF (test attempting callback with invalid state), ensure `code_verifier` mismatch returns `invalid_grant` and is surfaced to operator.

**Decisions & Assumptions**
- We'll implement backend code exchange only (no SDK usage).
- PKCE is mandatory and will be enabled in the provider implementation; the frontend is responsible for generating `code_verifier` and `code_challenge` unless the repo's OAuth helpers generate them.
- The repository stores access/refresh tokens using existing token storage patterns; no new persistence layer will be added unless requested.

**Further Considerations**
1. Should the provider persist `device_id` and `state` server-side, or rely on frontend session storage? Recommendation: store `state` server-side (short TTL) and accept `device_id` from callback to forward to token exchange.
2. Decide whether to parse and validate `id_token` (JWT) for additional claims; option A: parse it for subject verification, option B: ignore unless needed.
3. Rate limits: if backend origin may exceed default limits, plan for IP whitelisting with VK support.
