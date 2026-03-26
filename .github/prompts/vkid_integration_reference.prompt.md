# VK ID OAuth 2.1 Integration — Technical Reference for LLM Agents

## Overview

VK ID is VK's OAuth 2.1-based authorization service for websites and mobile apps.  
Integration is possible via **REST API directly** or via the **VK ID SDK** (which wraps the same API internally).  
The protocol used is **Authorization Code Flow + PKCE** (mandatory). Implicit flow is NOT supported.

---

## 1. Architecture & Token Flow

### High-Level Flow

1. User clicks "Login via VK ID" on your app.
2. App redirects user to `https://id.vk.ru/authorize?response_type=code` with required parameters.
3. VK ID authenticates the user (phone/email/password/SSO on the device).
4. User consents to data access (only on first login).
5. VK ID redirects back to your `redirect_uri` with `code` + `state` + `device_id`.
6. Your backend/frontend exchanges `code` for tokens via POST to `/oauth2/auth`.
7. You receive: **Access token + Refresh token + ID token**.
8. Use Access token for API calls; refresh when expired.

### Token Types

| Token | Purpose | Lifetime |
|-------|---------|---------|
| **Access token** | Authenticate API requests (`/oauth2/user_info`, VKontakte API) | Short-lived |
| **Refresh token** | Obtain new Access + Refresh token pair | Long-lived |
| **ID token** | OpenID Connect identity assertion (JWT) | One-time use |

---

## 2. Web Authorization Flows (4 variants)

VK ID on Web supports four integration paths:

| # | Method | Code Exchange Location |
|---|--------|----------------------|
| 1 | Via SDK | Frontend |
| 2 | Via SDK | Backend |
| 3 | Without SDK | Frontend |
| 4 | Without SDK | Backend |

**Recommended**: SDK with backend code exchange (most secure).

### Flow Step-by-Step (Without SDK, Backend Code Exchange)

**Step 1 — Authorization Request (GET)**

```
GET https://id.vk.ru/authorize
  ?response_type=code
  &client_id=<app_id>
  &redirect_uri=<your_callback_url>
  &state=<random_opaque_string>
  &code_challenge=<base64url(SHA256(code_verifier))>
  &code_challenge_method=S256
  [&scope=<space-separated scopes>]
  [&prompt=<login|consent|none>]
  [&provider=<auth_provider>]
  [&lang_id=<language_code>]
  [&scheme=<light|dark>]
```

**Required parameters:**

| Parameter | Description |
|-----------|-------------|
| `response_type` | Must be `code` |
| `client_id` | Your app's identifier (from app settings) |
| `redirect_uri` | Must match registered URI exactly |
| `code_challenge` | `BASE64URL(SHA256(code_verifier))` — RFC 7636 |
| `code_challenge_method` | Must be `S256` |

**Optional parameters:**

| Parameter | Description |
|-----------|-------------|
| `state` | CSRF protection token (strongly recommended) |
| `scope` | Permissions to request (space-separated) |
| `prompt` | `login` = force re-auth, `consent` = force re-consent |
| `provider` | Auth method hint |
| `lang_id` | UI language |
| `scheme` | `bright` (light) or `space_gray` (dark) theme |

**Step 2 — Redirect Callback**

VK ID redirects to `redirect_uri` with:
```
?code=<authorization_code>
&state=<echoed_state>
&device_id=<device_identifier>
```

- **Validate `state`** matches what you sent (CSRF prevention).
- The `authorization_code` is **single-use** and **short-lived**.
- The page receiving the code **must not** contain scripts, images, styles, or any embedded content (security requirement — prevents code leakage).

**Step 3 — Token Exchange (POST)**

```
POST https://id.vk.ru/oauth2/auth
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code
&code=<authorization_code>
&client_id=<app_id>
&redirect_uri=<same_as_step1>
&code_verifier=<original_code_verifier>
&device_id=<device_id_from_callback>
&state=<state_value>
```

> ⚠️ **Web (browser) note**: Send token in request **body only**, with `Content-Type: application/x-www-form-urlencoded`. Sending token in `Authorization` header triggers CORS preflight (OPTIONS), which the browser won't execute the main request for. JSON content-type also breaks this.

**Step 4 — Token Response**

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "id_token": "...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

**Step 5 — Token Refresh**

```
POST https://id.vk.ru/oauth2/auth
Content-Type: application/x-www-form-urlencoded

grant_type=refresh_token
&refresh_token=<current_refresh_token>
&client_id=<app_id>
&device_id=<device_id>
&state=<new_state>
```

Returns new `access_token` + `refresh_token`.

---

## 3. PKCE — Proof Key for Code Exchange

PKCE is **mandatory** for all apps that exchange authorization codes for tokens.

### Generation Steps

```python
import secrets, hashlib, base64

# 1. Generate code_verifier: random string, 43–128 chars, URL-safe
code_verifier = base64.urlsafe_b64encode(secrets.token_bytes(32)).rstrip(b'=').decode()

# 2. Generate code_challenge: SHA-256 hash, base64url-encoded
code_challenge = base64.urlsafe_b64encode(
    hashlib.sha256(code_verifier.encode()).digest()
).rstrip(b'=').decode()

# code_challenge_method = "S256"
```

**Rules:**
- `code_verifier`: chars `[a-zA-Z0-9_-]`, length 43–128
- `code_challenge`: `BASE64URL(SHA256(code_verifier))`, RFC 7636
- **Never** include both `code_verifier` and `code_challenge` in the same request
- Store `code_verifier` client-side; send it only at token exchange

---

## 4. API Reference — Key Endpoints

**Base URL**: `https://id.vk.ru`

### 4.1 Authorization Endpoint

```
GET /authorize
```
Initiates auth flow. Described in Section 2.

### 4.2 Token Endpoint

```
POST /oauth2/auth
```
Used for:
- `grant_type=authorization_code` — exchange code for tokens
- `grant_type=refresh_token` — refresh expired access token

### 4.3 User Info

```
POST /oauth2/user_info
Authorization: Bearer <access_token>
```

Returns user profile data according to granted scopes.

**Example response fields** (depending on scope):
- `user.id` — VK user ID
- `user.first_name`, `user.last_name`
- `user.avatar` — profile picture URL
- `user.phone` — phone number (if `phone` scope granted)
- `user.email` — email (if `email` scope granted)
- `user.birthday` — date of birth (if `birthday` scope granted)

### 4.4 Revoke Token / Logout

```
POST /oauth2/logout
Authorization: Bearer <access_token>
```

Revokes the user's authorization for your app. Logs the user out of the current session.

### 4.5 Available Scopes

| Scope | Data |
|-------|------|
| `vkid.personal_info` | Name, avatar (default) |
| `email` | Email address |
| `phone` | Phone number |
| `birthday` | Date of birth |

---

## 5. Security Requirements & Best Practices

### Mandatory
- **PKCE** (`S256` method) — required for all code exchanges
- **`state` parameter** — generate a random opaque value per request; validate on callback to prevent CSRF
- **`redirect_uri` must be exact** — must match the registered URI in app settings
- **Redirect page must be clean** — no JavaScript, scripts, images, styles, or any embedded resources on the page that receives the auth code (prevents code theft)

### Strongly Recommended
- Exchange code on **backend** — never expose `client_secret` in frontend JS
- Store `code_verifier` only in memory/session — never persist it
- Tokens should be stored securely (httpOnly cookies for web)
- Validate `state` before any processing on callback

### Rate Limits (Backend Integration)
- Default: **15,000 requests/day per IP**
- If you expect higher traffic: email `devsupport@corp.vk.com` to raise limits
- All backend API requests go from a restricted IP list — plan for IP whitelisting if needed

---

## 6. SDK Integration (Quick Path)

If using the **VK ID SDK** (JavaScript/npm), it wraps the above API:

1. Install via `<script>` tag or package manager
2. SDK handles: authorization URL construction, PKCE generation, redirect handling, token exchange
3. Two modes: **frontend code exchange** (SDK manages everything) or **backend code exchange** (SDK gives you the `code`, your server exchanges it)

**Frontend SDK code exchange** — SDK generates PKCE internally, redirects user, receives callback, and returns tokens directly to your JS.

**Backend SDK code exchange** — SDK redirects and receives callback; it gives your code the `authorization_code` + `device_id`, your server calls `/oauth2/auth` to get tokens. This is the more secure approach.

---

## 7. Error Handling

When authorization fails, VK ID redirects to `redirect_uri` with `error` instead of `code`:

```
?error=access_denied
&error_description=User+denied+access
&state=<echoed_state>
```

Common errors:
| Error | Meaning |
|-------|---------|
| `access_denied` | User cancelled or denied consent |
| `invalid_request` | Missing/invalid parameters |
| `invalid_client` | Bad `client_id` |
| `invalid_grant` | Code expired, already used, or `code_verifier` mismatch |
| `server_error` | VK ID internal error |

Token exchange errors return JSON:
```json
{
  "error": "invalid_grant",
  "error_description": "Code has expired or already been used"
}
```

---

## 8. Integration Checklist

- [ ] App registered in VK ID developer console with correct `redirect_uri`
- [ ] `client_id` obtained from app settings
- [ ] PKCE implemented: generate `code_verifier` + `code_challenge` (S256) per request
- [ ] `state` generated randomly and validated on callback
- [ ] Redirect callback page contains no scripts/assets
- [ ] Token exchange happens server-side (preferred)
- [ ] `access_token` stored securely; `refresh_token` logic implemented
- [ ] Logout calls `/oauth2/logout` to revoke server-side session
- [ ] Rate limits considered; contact support if >15k req/day per IP needed

---

## 9. Key URLs Summary

| Purpose | URL |
|---------|-----|
| Authorization | `https://id.vk.ru/authorize` |
| Token Exchange / Refresh | `https://id.vk.ru/oauth2/auth` |
| User Info | `https://id.vk.ru/oauth2/user_info` |
| Logout / Revoke | `https://id.vk.ru/oauth2/logout` |
| Developer Support | `devsupport@corp.vk.com` |
