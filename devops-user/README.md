# devops-user (C-side User Service)

An **independent service** alongside `devops-system` (B-side admin backend), serving App / mini-program and other C-side users.

## Gateway-Side Configuration

The C side uses a **separate domain + a separate jwt-auth plugin instance**, fully isolated from the B side:

| Resource | File | Role |
|---|---|---|
| Main route | `deploy/higress/ingresses/devops-user.yaml` | `user-api.devops.local/user` (jwt-auth mounted) |
| Anonymous route | `deploy/higress/ingresses/devops-user-public.yaml` | login / register / jwks.json (exempted) |
| Verification plugin | `deploy/higress/wasmplugins/jwt-auth-user-2.0.0.yaml` | `issuer: devops-user` |

**Why a separate domain**: both services serve `/.well-known/jwks.json`, and under the same host longest-prefix matching
cannot tell them apart, hitting one at random. And C/B-side traffic profiles differ (user volume, rate-limit dimensions, gray-release needs),
requiring separate governance.

### Three Must-Not-Change Items (All Have Caused Pitfalls)

1. **`metadata.name` cannot be `jwt-auth`** — pilot would take the Istio native branch
   (`JwtRule: ""`), and all-in-one 2.2.3 has no `jwt_authn` filter, so nothing loads.
2. **`url` must use `localhost:8002`** — `oci://` requires the controller to pull and persist, which does not trigger under the file backend.
3. **`matchRules.config` cannot inline the `consumers` array** — pilot fails to parse and **silently skips
   the whole plugin** (no logs at all); nor can it be just `{}` (that replaces `defaultConfig` entirely, so even real tokens get
   403 `Access Denied`). It must carry `allow` pointing back to the consumer in `defaultConfig`.

To verify the plugin is really loaded:

```bash
docker compose -f deploy/docker-compose-dev.yml exec higress \
  sh -c 'grep -c "fetching image plugins/jwt-auth" /var/log/higress/gateway.log'
```

Or check whether a token-less request returns **plain text** `Jwt is missing` (the envelope format `{"code":401,...}`
means it is the application layer; the plugin is not in effect).

### Anonymous Paths Must Not Be Merged

The three paths in `devops-user-public.yaml` must be written in full:

- Merging into `/user/api/v1/auth` exempts `logout` too (anyone could kick others offline)
- Merging into `/user/api/v1` exempts `profile` too (**severe privilege escalation**)

### Cross-Side Token Isolation Is Verified

The C-side and B-side issuers differ; a B-side token accessing C-side endpoints returns `Jwt verification fails`,
and vice versa — this is the real effect of issuer isolation. Never set either issuer to the same value.

## Quick Start

```bash
cd devops-user
cp config/.env.example config/.env   # or write it by hand, see below
go run . server
# Local requests must go through the gateway: JWT is verified only at the gateway, and the
# backend only trusts the identity headers injected by the gateway.
#   curl -H "Host: user-api.devops.local" http://127.0.0.1:8080/user/api/v1/oauth/providers
# The API docs can be viewed through the gateway as well: http://127.0.0.1:8080/user/apidoc
```

`config/.env` needs `NACOS_REGISTER_IP` set (the gateway uses it to call back to this process):

```bash
# Windows + Docker Desktop
NACOS_REGISTER_IP=host.docker.internal
```

> ⚠️ **Do not use the bridge gateway (e.g. 172.19.0.1)**: tested — container access to `172.19.0.1:10000` returns 000
> (unreachable), while `host.docker.internal:10000` returns 200.
> On Linux, use the host's intranet IP instead (the non-loopback address found via `ip addr`).

`.env` is already covered by `.gitignore`'s `**/config/.env` and will not be committed by mistake.

## Relationship with devops-system

**Framework capabilities fully reuse `devops-infra`, zero copying**:

| Capability | Location |
|---|---|
| HTTP engine / graceful shutdown / `/metrics` | `devops-infra/http` |
| BaseModel (primary key + audit + soft delete + ID-generation hook) | `devops-common/model` (shared with devops-system) |
| OpenAPI assembly and doc endpoints | `devops-infra/http/apidoc` |
| Unified response envelope | `devops-common/response` |
| Error exits and status codes | `devops-infra/errs` + `http/humaerr` |
| JWT middleware suite `Kit` | `devops-infra/security/humakit` |
| RS256 signing/verification | `devops-infra/security/jwt` |
| Redis session state | `devops-infra/security/session` |
| Traffic-ingress guard | `devops-infra/security/gateway` |

**Business-shape differences (intentional, not omissions)**:

| Dimension | devops-system (B side) | devops-user (C side) |
|---|---|---|
| API prefix | `/system/api/v1/*` | `/user/api/v1/*` |
| Port | 9999 | 10000 |
| Redis DB | db 0 | db 1 (avoids keyspace pollution) |
| Tenant model | single-database multi-tenant (`TenantPlugin` + casbin) | **none** (single tenant) |
| Permissions | RBAC permission points (`kit.Perm("system:user:list")`) | **none**; uses `kit.Logged()` + service-layer row-level filtering by `claims.UID` |
| JWT issuer / kid | `devops` / `devops-system-rs256-1` | `devops-user` / `devops-user-rs256-1` |

> ⚠️ **The issuer must differ from devops-system's**. When the two issuers are the same, they match each other
> in the gateway jwt-auth's consumers, letting a B-side token access C-side endpoints (and vice versa).

## C-side Row-Level Ownership Checks

No RBAC. Endpoints **do not accept** client-passed user IDs; identity comes only from the JWT:

```go
//api/v1/profile.go —— updateBody has no id / user_id fields
type updateBody struct {
    Nickname *string `json:"nickname" required:"false"`
    // ...
}

// service takes claims from ctx, so it can only modify its own data
func (s *Profile) Update(claims *auth.AccessClaims, in UpdateInput) (...) {
    s.users.update(claims.UserID, func(u *model.User) { /* ... */ })
}
```

Allowing the client to pass an ID and then comparing it would amount to handing the privilege-escalation verdict to client parameters.

## Current Implementation Status

Working end to end: **register → login → read profile → update profile → logout**, including OpenAPI docs and the JWKS endpoint.
Data lands in the MySQL `t_user` table (auto-created at startup), with primary keys generated by devops-id.

### Completed

- **GORM integration**: `model` (table models + ID-generation hook), `dao` (data access),
  `dbinit` (startup-time table creation, embedded SQL scripts).
- **bcrypt password hashing** (cost=10), with explicit rejection of passwords longer than 72 bytes (bcrypt's limit).
- **Three unique constraints**: `uk_username` / `uk_email` / `uk_mobile`,
  with conflicts reported by **specific constraint** rather than a blanket "username already registered".

### To Be Completed

- **Third-party tokens not encrypted at rest**: `t_user_oauth.access_token` / `refresh_token`
  are currently stored in plaintext; production must switch to application-layer AES-GCM encryption (master key in secrets management).
- **"Keep this device" on password change is incomplete**: `ChangePassword` supports keeping the current device's refresh,
  but **the access side still kicks everything** (`session.Store` only has the forward index `user → sids`,
  with no `sid → refreshHash` reverse lookup, so the session to keep cannot be located),
  so users keeping their device still need to log in once; only the refresh can be exchanged for a new access directly.
- **No admin entry for bans**: `Profile.SetDisabled` / `SetDisabledByUsername` are ready
  in the service layer and force all sides offline, but **deliberately expose no HTTP endpoint** —
  if C-side users could self-unban, bans would be meaningless. Waiting for integration with the admin side (devops-system) or an ops backend.

## Third-Party Login (OAuth2)

Three providers supported: GitHub / Google / WeChat (open platform website app QR scan), standard authorization-code flow.

### Endpoints

| Endpoint | Auth | Description |
|---|---|---|
| `GET /user/api/v1/oauth/providers` | anonymous | list available providers and authorization URLs (with one-time state) |
| `GET /user/api/v1/oauth/{provider}/callback` | anonymous | query-mode callback (**all three use this**) |
| `POST /user/api/v1/oauth/{provider}/callback` | anonymous | form_post-mode callback (unused by any provider currently, reserved) |
| `POST /user/api/v1/oauth/bind/start` | login required | logged-in user initiates binding |
| `GET /user/api/v1/oauth/bind` | login required | list bindings (no tokens included) |
| `DELETE /user/api/v1/oauth/bind` | login required | unbind |

### Two Login Paths

Determined by `BindUserID` in the state:

- **Login** (`BindUserID == 0`): existing binding → sign dual tokens directly; no binding → auto-create the account or report "please log in before binding" per
  `auto_register`.
- **Binding** (`BindUserID > 0`): attach to the current logged-in user. If the third-party account is already bound to someone else, reject
  (double guarantee via the `uk_provider_provider_id` unique index).

### Five Security Essentials

1. **state must be verified and one-time** (atomic via Lua `GETDEL`) — without verification, login can be CSRF-hijacked;
   without one-time use, the same state can be replayed. Tested: forged/no state both return 401.
2. **`provider_user_id` uses the third party's stable identifier** (GitHub numeric ID, Google `sub`, WeChat `unionid`),
   **not username/email/nickname** — the first two can be changed, WeChat nicknames can be duplicated,
   and using them as the binding key causes mis-binding and impersonation.
3. **Google verifies aud/iss via tokeninfo**; **WeChat must explicitly check `errcode`** —
   ⚠️ WeChat **returns HTTP 200 even when all APIs error**, with the error only in the body's `errcode`.
   Checking only the status code would treat "invalid code" as success, then write an empty binding key and mis-bind accounts to each other.
4. **Redirect URLs must go through a whitelist** (`oauth.redirect_whitelist`) — otherwise open-redirect phishing:
   the victim is redirected to the attacker's site after login to steal tokens. When the whitelist is empty, **always deny**.
5. **Auto-created accounts have no usable password**: a 32-byte random strong password is generated, eliminating logins with an "empty/default password".

### Four Special WeChat Constraints

The WeChat authorization endpoint is **not a standard OAuth2 endpoint**, so `WeChatProvider` constructs the URL manually,
without using `oauth2.Config.AuthCodeURL`:

| Constraint | Description |
|---|---|
| The endpoint is `/connect/qrconnect` | not the common `/oauth2/authorize` |
| The URL must end with `#wechat_redirect` | **without it WeChat directly reports "the link cannot be accessed"** |
| `redirect_uri` is encoded only once | `url.Values.Encode()` encodes `%` into `%25`, causing double encoding, hence manual construction |
| `scope` is fixed at `snsapi_login` | any other value is rejected; configuring `scopes` is also forcibly overwritten internally |

The binding key prefers `unionid` (unique across apps under the same open platform account), falling back to `openid` when absent;
if both are empty, **login is rejected**.

⚠️ **WeChat returns no email and does not allow requesting one**, so pure WeChat users' `email` is always empty —
"recover password via email" is unavailable to them; a separate bind-phone/email flow is needed.
The avatar URL expires once the user changes their avatar; it currently serves only as a display snapshot.

### Data Tables

`t_user_oauth`, with two unique indexes, neither dispensable:

- `uk_provider_provider_id` — prevents "one third-party account bound to multiple local users" (account hijacking)
- `uk_user_provider` — prevents "the same user binding the same provider repeatedly"

There is also the local user table `t_user` (differences from `t_system_users`: **no `tenant_id`**,
no roles/departments, `disabled` replaces `status`). ⚠️ Its email/mobile fields must use pointer fields
(`*string`), storing `NULL` rather than empty strings when unfilled — MySQL unique indexes do not constrain multiple `NULL`s,
whereas empty strings would make the second user without a bound email conflict (the application-layer `model.NormalizePtr` handles normalization).

### ⚠️ Current Limitations

- **WeChat not verified end to end with real credentials**: the API implementation and unit tests are complete (12 items, including errcode
  and empty-identifier protection), but constrained by "requires business qualification + a registered domain", there are no real AppID/Secret
  for real QR-scan verification. Integration steps are in `docs/oauth2-guide.md`.
- **Third-party tokens not encrypted**: see "To Be Completed".
- **WeChat users have no email**: see the WeChat constraints above.

## Authentication System

### Dual Tokens (access + refresh)

| | access | refresh |
|---|---|---|
| Form | RS256 JWT | **256-bit random opaque string** (base64url, 43 chars) |
| Storage | stateless | Redis, server stores **only the SHA-256 hash** |
| Validity | `jwt.access_expire` (default 2h) | `jwt.refresh_expire` (default 168h) |
| Invalidation | wait for expiry | precisely invalidated on logout / rotation / password change |

> ⚠️ **Never reuse an access token as a refresh token**: an access token is a stateless JWT that works until expiry once obtained,
> which amounts to giving an attacker an indefinitely renewable credential, and logout/ban cannot truly invalidate it.

Rotation (`POST /user/api/v1/auth/refresh`) uses a **Lua script atomically** to complete
"query old → delete old → write new" — step-by-step execution would let concurrent refreshes each get the same old token and each sign a new
token (a replay vulnerability). Tested: a used refresh immediately returns 401.

Logging out with the `refresh_token` invalidates it too; otherwise the old refresh can still exchange for a new token (equivalent to not logging out).

### Login Failure Rate Limiting and Account Locking

bcrypt can only slow down single brute-force attempts, not stop high-speed attempts. The C side has open registration and enumerable usernames,
so two dimensions are stacked (**either alone can be bypassed**):

| Dimension | Default threshold | Blocks what |
|---|---|---|
| IP rate limiting | 20 failures within 5 minutes → 429 | one IP batch-trying different usernames |
| Account locking | 5 failures within 15 minutes → locked for 15 minutes | multiple IPs concentrating on one account |

Config is in the `login_lock` section. The client IP is validated against XFF via `server.trusted_proxies` before adoption —
otherwise an attacker can forge `X-Forwarded-For` to bypass IP rate limiting.

"User does not exist" also counts as one failure (otherwise usernames can be enumerated at no cost); an already-banned account does **not** count as a failure
(it is not a user input error, and counting it would wrongly ban normal users' IP counts).

### Anonymous Endpoints (Must Be Gateway-Exempted + Application-Allowed)

| Path | Why anonymous |
|---|---|
| `POST /user/api/v1/auth/register` | registration |
| `POST /user/api/v1/auth/login` | no token yet at this moment |
| `POST /user/api/v1/auth/refresh` | **access has already expired**; requiring a JWT would mean it can never be refreshed |
| `GET /.well-known/jwks.json` | public key distribution |

⚠️ logout / profile / profile/password **cannot** be merged into these prefixes:
merging into `/user/api/v1/auth` would exempt logout too (anyone could kick others offline);
merging into `/user/api/v1` would exempt profile and password change too (**severe privilege escalation**).

## Directory Structure

```
devops-user/
├── main.go                # humaerr.Install + banner + cmd.Execute
├── banner.txt             # go:embed
├── config/
│   ├── bootstrap.yaml         # common baseline (server/nacos)
│   ├── devops-user-dev.yaml   # environment overlay (incl. dev JWT private key)
│   └── .env               # NACOS_REGISTER_IP (not committed)
└── internal/
    ├── cmd/root.go        # assembly: db → create tables → ID generation → Redis → JWT → Kit → service
    ├── config/            # root Config and proprietary config sections
    ├── auth/              # auth facade (C-side semantic adaptation)
    ├── middleware/        # Kit aliases + English envelope text
    ├── model/             # GORM table models (BaseModel reuses devops-common/model)
    ├── dao/               # data access (no tenant filtering)
    ├── dbinit/            # startup-time table creation (embedded sql/schema)
    ├── service/           # business use cases + bcrypt
    ├── api/v1/            # huma handlers (incl. ClientIP middleware)
    ├── refreshtoken/      # refresh token storage (opaque string + hash, Lua atomic rotation)
    ├── loginlock/         # login failure rate limiting and account locking
    ├── router/            # route registration
    └── store/             # Redis assembly (delegates to infra/cache/redis)
```

## Roadmap

- [x] User login/registration, OAuth2.0 third-party login: github, google, wechat
- [ ] User profile management: change password, change email, change phone, change avatar, change nickname
- [ ] User profile analytics, e.g.
  - [ ] Login count, login time, login IP, login device, login geolocation
  - [ ] Registration time, registration IP, registration device, registration geolocation
  - [ ] User activity analytics, e.g. DAU, WAU, MAU
  - [ ] User retention analytics, e.g. next-day retention, 7-day retention, 30-day retention
