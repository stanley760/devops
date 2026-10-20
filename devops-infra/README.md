# devops-infra Enhanced Middleware Guide

`devops-infra` is the shared infrastructure module of the devops platform. This document targets **business service developers** and explains in detail how to use the already-enhanced middleware capabilities in this module, including:

| Capability | Package path | Description |
| --- | --- | --- |
| HTTP common middleware | `devops-infra/http/middleware` | panic recovery, access logging; `http.Run` works out of the box |
| gRPC common interceptors | `devops-infra/grpc/interceptor` | panic recovery, access logging; `grpc.Run` works out of the box |
| JWT authentication | `devops-infra/security/jwt` | RS256 + kid signing/verification, JWKS output |
| Redis session store | `devops-infra/security/session` | refresh session state (kick offline, rotation, batch revocation) |
| Authentication / authorization middleware | `devops-infra/security/middleware` | framework-agnostic authenticator + Gin middleware trio |
| Casbin authorization decisions | `devops-infra/security/authz` | multi-tenant domain-model RBAC, gorm-adapter persistence |
| Multi-tenant middleware | `devops-infra/tenant` | request-header tenant context injection + GORM row-level isolation |
| Unified response and errors | `devops-infra/http/response`, `devops-infra/errs` | unified envelope, unified error codes |

> This module only makes **decisions**, not **presentation**: middleware produces claims / tenant context / allow-or-deny verdicts; HTTP status-code text and response envelopes are customized by each business service via callbacks (`HumaOptions`) or the `response` package.

---

## Table of Contents

1. [Overall Architecture](#1-overall-architecture)
2. [Quick Start](#2-quick-start)
3. [HTTP Common Middleware](#3-http-common-middleware)
4. [gRPC Common Interceptors](#4-grpc-common-interceptors)
5. [Unified Response and Error Handling](#5-unified-response-and-error-handling)
6. [JWT Authentication: Signing, Verification and Sessions](#6-jwt-authentication-signing-verification-and-sessions)
7. [Authentication / Authorization Gin Middleware](#7-authentication--authorization-gin-middleware)
8. [Casbin Authorization Decisions and Policy Management](#8-casbin-authorization-decisions-and-policy-management)
9. [Multi-Tenant Middleware and Data Isolation](#9-multi-tenant-middleware-and-data-isolation)
10. [Middleware Orchestration Best Practices](#10-middleware-orchestration-best-practices)
11. [Unit Test Support](#11-unit-test-support)
12. [Data Permissions and Middleware Suite](#12-data-permissions-and-middleware-suite)
13. [Appendix: API Cheat Sheet](#13-appendix-api-cheat-sheet)

---

## 1. Overall Architecture

A protected HTTP request passes through the middleware chain in the following order (the same for gRPC):

```
Client
  │  Authorization: Bearer <access_token>
  │  X-Tenant-ID: <tenant_id>
  ▼
┌─────────────────────────────────────────────────────────────┐
│ http.Run auto-mounts: Recovery → AccessLog                  │
│   unified 404/405 envelope, /healthz, /readyz               │
├─────────────────────────────────────────────────────────────┤
│ Business route groups (huma chain, mounted in order):       │
│   ① HumaAuthenticate            Bearer → RS256 verify → session state │
│   ② HumaTenantContext           claims → tenant usability → ctx    │
│   ③ HumaRequirePermission       in-domain Casbin authz (optional)   │
│   ④ HumaRequirePlatformPermission  platform admin authz (optional) │
│   ⑤ response.WriteHumaEnvelope   unified error exit                  │
├─────────────────────────────────────────────────────────────┤
│ Data access layer: tenant.RegisterPlugin auto-appends WHERE tenant_id │
└─────────────────────────────────────────────────────────────┘
```

Three core design principles:

- **fail-closed**: any internal error in the authenticator, session store, Casbin or tenant plugin is treated as "deny" (503 / query error), never allowed through.
- **Decision/presentation separation**: security decisions live in `devops-infra/security/*`; how to respond (text, envelope) is decided by business-side callbacks.
- **Framework-agnostic core**: the authenticator `Authenticator` depends only on `context` and two interfaces (`TokenParser`, `SessionStore`); Gin is just a thin adapter layer, extensible to other frameworks the same way.

---

## 2. Quick Start

Taking a minimal business service as an example, wire up all middleware capabilities in 5 steps:

```go
package main

import (
    "crypto/rsa"
    "log"
    "time"

    infracfg "devops-infra/config"
    httpinfra "devops-infra/http"
    securityjwt "devops-infra/security/jwt"
    securitymw "devops-infra/security/middleware"
    securitysession "devops-infra/security/session"
    "devops-infra/tenant"
)

func main() {
    // Config is loaded via struct mapping (each service composes infracfg.ServerConfig etc. into its own root Config)
    var cfg struct {
        Server infracfg.ServerConfig `mapstructure:"server"`
        JWT    struct {
            SigningKey    string        `mapstructure:"signing_key"`
            KeyID         string        `mapstructure:"key_id"`
            Issuer        string        `mapstructure:"issuer"`
            AccessExpire  time.Duration `mapstructure:"access_expire"`
            RefreshExpire time.Duration `mapstructure:"refresh_expire"`
        } `mapstructure:"jwt"`
    }
    if err := infracfg.LoadInto(&cfg); err != nil {
        log.Fatal(err)
    }

    // ① Build the JWT manager (RS256 private key + self-verifying KeySet)
    privPEM := []byte(cfg.JWT.SigningKey)
    pub, err := securityjwt.PublicKeyFromPrivatePEM(privPEM)
    if err != nil {
        log.Fatal(err)
    }
    keySet, err := securityjwt.NewStaticKeySet(map[string]*rsa.PublicKey{cfg.JWT.KeyID: pub})
    if err != nil {
        log.Fatal(err)
    }
    tokenMgr, err := securityjwt.NewManager(securityjwt.Config{
        PrivateKeyPEM: privPEM,
        KeyID:         cfg.JWT.KeyID,
        Issuer:        cfg.JWT.Issuer,
        AccessExpire:  cfg.JWT.AccessExpire,
        RefreshExpire: cfg.JWT.RefreshExpire,
        KeySet:        keySet,
    })
    if err != nil {
        log.Fatal(err)
    }

    // ② Build the Redis session store and authenticator
    // sessions := securitysession.NewRedisClientStore(redisClient, cfg.JWT.RefreshExpire)
    // authn := securitymw.NewAuthenticator(tokenMgr, sessions)

    // ③ Register the GORM tenant isolation plugin (optional)
    // tenant.RegisterPlugin(gormDB)

    // ④ Assemble the huma middleware chain: authenticate → tenant → permission → handler (see sections 7 and 9 for details)
    _ = securitymw.HumaAuthenticate
    _ = tenant.HumaTenantContext

    // ⑤ One line to start: auto-mounts Recovery/AccessLog/healthz/readyz
    // httpinfra.Run(cfg.Server, func(r *gin.Engine) { /* register routes */ })
}
```

Each of the following sections expands on these one by one.

---

## 3. HTTP Common Middleware

### 3.1 `http.Run`: Out-of-the-Box Service Skeleton

```go
func Run(cfg config.ServerConfig, register Register, readiness ...ReadinessCheck) error
```

`http.Run` already completes the following wiring internally, so **business code does not need to mount it again**:

| Built-in capability | Behavior |
| --- | --- |
| `gin.Logger()` | gin standard request log |
| `middleware.Recovery()` | panic → unified 500 envelope (see below) |
| `middleware.AccessLog()` | structured access log after request completion (Debug level) |
| `SetTrustedProxies` | trusted proxies per the `server.trusted_proxies` config, ensuring `ClientIP` is correct |
| `NoRoute / NoMethod` | 404 / 405 return a unified envelope instead of gin's default plain text |
| `GET /healthz` | liveness probe, returns `{"status":"ok"}` |
| `GET /readyz` | readiness probe, runs readiness checks one by one (2s timeout each); returns 503 with the not-ready items listed |
| Graceful shutdown | listens for SIGINT/SIGTERM, drains in-flight requests for 10s |

Business services only need to provide a route registration callback and readiness checks:

```go
readiness := []httpinfra.ReadinessCheck{
    {Name: "mysql", Check: mysqlClient.Ping},
    {Name: "redis", Check: redisPing},
}
err := httpinfra.Run(cfg.Server, router.Register(...), readiness...)
```

`config.ServerConfig` maps to YAML config:

```yaml
server:
  port: 8080
  read_timeout: 10s
  write_timeout: 10s
  trusted_proxies: ["10.0.0.0/8"]   # gateway/SLB CIDRs
```

### 3.2 `Recovery()`: Unified Panic Handling

```go
middleware.Recovery()
```

- Catches handler panics, logging `slog.Error` (including the panic value, method, path and full stack);
- Returns a unified envelope `{"code":500,"msg":"Internal server error"}` when the response has not been written yet;
- Only sets 500 when the response has been partially written (the envelope can no longer be changed at that point).

> Deliberately not using `gin.Recovery()` — its default output is plain text, which conflicts with the unified response format.

### 3.3 `AccessLog()`: Access Log

```go
middleware.AccessLog()
```

After a request completes, logs via `slog.Debug`: method, path (route template `FullPath()` preferred), status, latency, bytes, client_ip. In production, raising the log level to `info` or above silences it automatically; switch back to `debug` when troubleshooting, with no code changes.

---

## 4. gRPC Common Interceptors

### 4.1 `grpc.Run`: Server Skeleton

```go
grpcinfra.Run(grpcinfra.ServerConfig{
    Listen:     ":9090",
    Reflection: cfg.Env == "dev", // reflection enabled in dev for easier grpcurl debugging
}, func(s *grpc.Server, hc *health.Server) {
    pb.RegisterXxxServer(s, svc)
    hc.SetServingStatus("xxx.Service", healthpb.HealthCheckResponse_SERVING)
})
```

Automatically mounts:

- `interceptor.Recovery()`: panic → `codes.Internal` ("internal server error"), stack only goes to logs, no details leaked to the client;
- `interceptor.AccessLog()`: logs method, status code and latency via `slog.Debug` after each unary call completes;
- `grpc.health.v1` standard health check service;
- SIGINT/SIGTERM graceful shutdown: `GracefulStop` waits up to 10s, force-stops on timeout.

### 4.2 Client Dialing

```go
conn, err := grpcinfra.Dial(ctx, "devops-id:9090", 5*time.Second)
defer grpcinfra.Close(conn)

// Classify connection-class errors so upper layers can decide whether to retry
if grpcinfra.IsUnavailable(err) { ... }
```

`Dial` returns a non-blocking ready connection with built-in `Unavailable` exponential-backoff retries — safe for idempotent ID-generation calls (retries burn IDs at most, never duplicate them).

---

## 5. Unified Response and Error Handling

Middleware and business handlers share the same response convention. All endpoints return an envelope:

```json
{"code": 200, "msg": "ok", "data": {...}}
```

`code` shares the same space as HTTP status codes (200/400/401/403/404/405/500).

### 5.1 Handler Style: `response.Wrap`

Business handlers report failures by **returning an error**; the success path writes the response itself:

```go
func (h *UserHandler) Get(c *gin.Context) error {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        return errs.New(errs.CodeParam, "invalid user ID")        // → 400
    }
    user, err := h.svc.Get(c.Request.Context(), id)
    if err != nil {
        return err                                            // → dispatched by error type
    }
    response.OK(c, user)                                      // → 200 envelope
    return nil
}

// Registration: Wrap is responsible for turning an error into a response
r.GET("/api/v1/users/:id", response.Wrap(userH.Get))
```

`Wrap`'s dispatch rules:

| Handler returns | Response |
| --- | --- |
| `nil` and response already written | passed through as-is |
| `nil` but response not written | 500 (fallback, logged, prevents request hang) |
| `*errs.BizError` (incl. `%w` wrapping) | output by its `Code`/`Msg` |
| JSON binding/validation error (`ShouldBindJSON` returned directly) | 400, with field-level messages |
| other unknown errors | 500, details only in logs |

### 5.2 Error Definitions: `errs`

```go
// Use predefined errors directly
return errs.ErrNotFound                       // 404 resource not found

// Attach business semantics while retaining the underlying cause (visible in logs, invisible to clients)
return errs.Wrap(err, errs.CodeInternal, "failed to sign token")
```

Error code constants: `CodeOK(200)` `CodeParam(400)` `CodeUnauthorized(401)` `CodeForbidden(403)` `CodeNotFound(404)` `CodeMethodNotAllowed(405)` `CodeInternal(500)`.

### 5.3 Common Response Exits

| Function | Purpose |
| --- | --- |
| `response.OK(c, data)` | 200 success; `data` of `nil` means no data |
| `response.OKPage(c, list, total)` | 200 paginated: `data: {list, total}` |
| `response.Fail(c, code, msg)` | failure exit, only 4xx/5xx allowed, via `AbortWithStatusJSON` |
| `response.FailErr(c, err)` | unified error exit (used inside `Wrap`, also directly callable) |

---

## 6. JWT Authentication

Package path `devops-infra/security/jwt`. Uses **RS256 asymmetric signing + kid key identifiers**: the signer holds the private key, verifiers need only the public key (or even just the JWKS endpoint), naturally supporting gateway verification and key rotation.

### 6.1 Building the Signing Manager

```go
// Private key PEM (PKCS#1 or PKCS#8), usually from the config item jwt.signing_key
privPEM := []byte(cfg.JWT.SigningKey)

// Single-signer deployment: export the public key from your own private key and assemble a self-verifying KeySet
pub, err := securityjwt.PublicKeyFromPrivatePEM(privPEM)
if err != nil { log.Fatal(err) }

keySet, err := securityjwt.NewStaticKeySet(map[string]*rsa.PublicKey{
    "devops-system-rs256-1": pub,   // kid → public key; during rotation both old and new keys may be present
})
if err != nil { log.Fatal(err) }

tokenMgr, err := securityjwt.NewManager(securityjwt.Config{
    PrivateKeyPEM: privPEM,          // private key for signing
    KeyID:         "devops-system-rs256-1",  // kid written into the token header
    Issuer:        "devops-system",  // iss, strictly verified when verifying
    AccessExpire:  30 * time.Minute, // access token validity period
    RefreshExpire: 7 * 24 * time.Hour, // refresh session validity period (session.TTL default)
    KeySet:        keySet,           // public key set for verification
})
```

`Config.Validate` requires all fields to be non-empty, returning `ErrInvalidConfig` when missing. Services that only need **verification** may omit `PrivateKeyPEM` (no signing), but `KeySet` must be provided.

### 6.2 Issuing an Access Token

```go
pair, err := tokenMgr.Issue(ctx, securityjwt.Subject{
    UserID:     42,
    Username:   "alice",
    TenantID:   7,
    TenantCode: "tenant-7",
}, sessionID)   // sessionID is generated by the caller (uuid recommended), also serving as the refresh session's sid
```

Returns `TokenPair{AccessToken, SessionID, AccessExpiresIn}`. The claims payload structure:

| Field | JSON key | Meaning |
| --- | --- | --- |
| `UID` | `uid` | user ID |
| `Username` | `username` | username |
| `SessionID` | `sid` | session ID (refresh credential) |
| `TenantID` | `tid` | tenant ID |
| `TenantCode` | `tcode` | tenant code |
| Standard fields | `iss/sub/iat/nbf/exp` | `sub` is the user ID as a string |

### 6.3 Parsing and Verification

```go
claims, err := tokenMgr.ParseAccess(tokenString)
// Success: *Claims; failure: ErrInvalidConfig (config problem) or ErrInvalidToken (forged/expired/unknown kid/issuer mismatch)
```

Strictly verified: RS256 algorithm, header `kid` present and findable in `KeySet`, `iss` matches, `exp` must exist and be unexpired.

### 6.4 JWKS Endpoint (Gateway Verification Integration)

```go
// Expose the RFC 7517 JWKS document for the Higress jwt-auth plugin (jwksFromUrl) or other services to fetch the public key
r.GET("/.well-known/jwks.json", func(c *gin.Context) {
    doc, err := tokenMgr.JWKS()   // {"keys":[{"kty":"RSA","kid":"...","use":"sig","alg":"RS256","n":"...","e":"..."}]}
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "jwks unavailable"})
        return
    }
    c.JSON(http.StatusOK, doc)
})
```

Only public key material (n/e) is output; the private key never leaves the signer. When rotating keys: new tokens are signed with the new `kid`, `KeySet` retains both old and new public keys, and old tokens remain verifiable within `exp`, achieving a smooth transition.

### 6.5 Redis Session Store (`security/session`)

Once issued, a JWT cannot be actively invalidated, so each access token is bound to a server-side session (`sid`). **Every request verifies the session exists** — after logout, password change or user deactivation, old tokens take effect immediately.

```go
// Two construction styles: wrap a cache/redis Client, or pass a go-redis client directly
sessions := securitysession.NewRedisStore(cacheRedisClient, 7*24*time.Hour)
sessions := securitysession.NewRedisClientStore(goRedisClient, 7*24*time.Hour)
```

Full `Store` interface methods:

```go
type Store interface {
    Create(ctx, Session) error                    // create session (on login/refresh)
    Active(ctx, SessionRef) (bool, error)         // whether the session is valid (called every request by the auth middleware)
    Lookup(ctx, sid) (tenantID, userID, ok)       // reverse lookup of ownership by sid (used by the refresh endpoint)
    Matches(ctx, SessionRef, userID) (bool, error)// verify the sid really belongs to that user
    Revoke(ctx, SessionRef) error                 // revoke a single session (logout/rotation)
    RevokeUser(ctx, tenantID, userID) error       // revoke all of a user's sessions (password change/deactivation → kick offline)
    RevokeTenant(ctx, tenantID) error             // revoke all of a tenant's sessions (tenant deactivation → everyone offline)
}
```

Redis keyspace (**segmented by tenant**, for per-tenant cleanup and auditing):

| Key | Value | TTL |
| --- | --- | --- |
| `t{tid}:auth:refresh:{sid}` | `user_id` | session TTL |
| `t{tid}:auth:sids:{uid}` | SET\<sid\> | permanent (for batch revocation indexing) |
| `auth:sid2uid:{sid}` | `"tid:uid"` | session TTL |

Standard usage for login/refresh (see devops-system's `auth.Manager`):

```go
// Login: sign + create session
sid := uuid.NewString()
pair, _ := tokenMgr.Issue(ctx, subject, sid)
_ = sessions.Create(ctx, securitysession.Session{
    TenantID: 7, UserID: 42, SessionID: sid, TTL: cfg.RefreshExpire,
})

// Refresh rotation (aligned with rotation best practices): create the new session first, then revoke the old sid
// Logout: sessions.Revoke(ctx, SessionRef{TenantID: 7, UserID: 42, SessionID: sid})
```

---

## 7. Authentication / Authorization Gin Middleware

Package path `devops-infra/security/middleware`. Three Gin middleware + one framework-agnostic authenticator.

### 7.1 `Authenticator`: Framework-Agnostic Authentication Decision

```go
type TokenParser interface{ ParseAccess(token string) (*securityjwt.Claims, error) }
type SessionStore interface{ Active(ctx, SessionRef) (bool, error) }

authn := securitymw.NewAuthenticator(tokenMgr, sessions) // the two dependencies are exactly implemented by the section 6 components
```

`Authenticate(ctx, bearerHeader)` performs three steps:

1. **Bearer parsing**: requires the `Authorization: Bearer <token>` prefix, otherwise `ErrUnauthorized`;
2. **Token verification**: `ParseAccess` failure → `ErrUnauthorized`; config error → `ErrInfrastructure`;
3. **Session state check**: Redis `Active` query error → `ErrInfrastructure`; session does not exist → `ErrSessionInactive`.

Error classification (determined with `errors.Is`):

| Error | Meaning | Suggested mapping |
| --- | --- | --- |
| `ErrUnauthorized` | missing header / wrong scheme / invalid or expired token | 401 |
| `ErrSessionInactive` | token valid but the server-side session has been revoked (kicked offline) | 401 (prompt re-login) |
| `ErrInfrastructure` | internal issues such as Redis failure or JWT config error | 503 (fail-closed) |

Non-Gin scenarios can call `authn.Authenticate` directly (inter-scheduled-task calls, other framework adapters).

### 7.2 `HumaAuthenticate`: Authentication Middleware

```go
var opts = securitymw.HumaOptions{
    OnUnauthorized: func(hctx huma.Context, err error) {
        response.WriteHumaEnvelope(hctx, http.StatusUnauthorized, "Not logged in or session expired")
    },
    OnForbidden: func(hctx huma.Context, err error) {
        response.WriteHumaEnvelope(hctx, http.StatusForbidden, "No operation permission")
    },
    OnInfrastructureError: func(hctx huma.Context, err error) {
        slog.Error("authentication infrastructure error", "err", err)
        response.WriteHumaEnvelope(hctx, http.StatusServiceUnavailable, "Service temporarily unavailable, please try again later")
    },
}

// Mount on huma Operations that require login
huma.Register(api, huma.Operation{
    Method: http.MethodGet, Path: "/api/v1/me",
    Middlewares: huma.Middlewares{securitymw.HumaAuthenticate(authn, opts)},
}, meHandler)
```

After successful authentication, `*securityjwt.Claims` is injected into the context via `huma.WithValue` and the request proceeds; on failure the corresponding callback is invoked by error classification and the chain is interrupted.

All three `HumaOptions` callbacks may be omitted; the default callbacks present 401/403/503 with a unified envelope (code/msg/data). **Recommended for production**: inject business text like the example above, keeping the envelope consistent site-wide.

To get the current user identity in downstream handlers / middleware:

```go
claims := securitymw.HumaCurrentClaims(ctx)   // *securityjwt.Claims, nil when unauthenticated
```

> Business services typically do one boundary conversion between their own claims type and the shared `Claims` (`FromShared`), so business code does not depend directly on infrastructure packages — see devops-system `internal/auth`.

### 7.3 `HumaRequirePermission`: In-Tenant-Domain Authorization

```go
huma.Register(api, huma.Operation{
    Method: http.MethodGet, Path: "/api/v1/users",
    Middlewares: huma.Middlewares{securitymw.HumaRequirePermission(authz, "system:user:list", opts)},
}, listHandler)
```

Internal logic: takes claims from the context → calls `authorizer.Enforce` with `claims.UID` (sub) and `claims.TenantID` (dom):

- allow → `next(hctx)`;
- Casbin error → `OnInfrastructureError` (503, fail-closed);
- no permission → `OnForbidden` (403);
- claims missing (`HumaAuthenticate` not run first) → `OnUnauthorized`.

### 7.4 `HumaRequirePlatformPermission`: Platform Admin Authorization

```go
// Platform routes do not mount tenant middleware; they operate on global tables
huma.Register(api, huma.Operation{
    Method: http.MethodGet, Path: "/api/v1/tenants",
    Middlewares: huma.Middlewares{securitymw.HumaRequirePlatformPermission(authz, "platform:tenant:list", opts)},
}, listHandler)
```

Dual verification, one identity threshold more than in-domain authorization:

1. `IsPlatformAdmin(uid)`: verifies the `g2` platform role;
2. `EnforcePlatform(uid, perm)`: executes the policy with `dom="*"`.

**Anti-privilege-escalation essentials**: platform permission identifiers like `platform:tenant:*` should only appear on platform routes and must not enter tenant menus/role tables — otherwise a tenant super admin holding `p, super_admin, {tid}, *` would reach platform endpoints via the `p.dom == "*"` wildcard.

### 7.5 Ordering Requirements Between Authentication and Authorization

`HumaRequirePermission` depends on `HumaCurrentClaims` and **must be registered after `HumaAuthenticate`** (huma executes in chain order, and permission middleware must come before the handler):

```go
// ✔ Correct
huma.Middlewares{securitymw.HumaAuthenticate(authn, opts), securitymw.HumaRequirePermission(authz, "p", opts)}
// ✘ Wrong: the permission middleware cannot get claims, everything returns 401
```

---

## 8. Casbin Authorization Decisions and Policy Management

Package path `devops-infra/security/authz`. Based on Casbin + gorm-adapter (policies persisted in the `casbin_rule` table), implementing **multi-tenant domain-model RBAC**.

### 8.1 Initialization

```go
authorizer, err := securityauthz.New(gormDB)   // auto-creates the table and loads existing policies
if err != nil { log.Fatal(err) }
// authorizer also satisfies the Authorizer interface used by middleware;
// pass it directly to HumaRequirePermission / HumaRequirePlatformPermission
```

### 8.2 Domain Model

```
[request_definition]  r = sub, dom, obj        sub=user ID  dom=tenant ID  obj=permission identifier
[policy_definition]   p = sub, dom, obj
[role_definition]     g = _, _, _              g(user, role, tenant): in-tenant role triple
                      g2 = _, _                g2(user, platform_admin): platform role (not domain-bound)
[matchers]            m = (g(r.sub, p.sub, r.dom) || g2(r.sub, p.sub))
                          && (r.dom == p.dom || p.dom == "*") && keyMatch(r.obj, p.obj)
```

- **g triple**: the same `user_id` is a different g row in different tenants, so role membership is naturally isolated by tenant;
- **g2 pair**: platform admins are not domain-bound, and with `p, platform_admin, *, *` they pass across all domains;
- `keyMatch`: permission identifiers support the `*` wildcard, e.g. `p, super_admin, 7, system:*` matches all system permissions of that tenant.

### 8.3 Decision API

```go
ok, err := authorizer.Enforce(ctx, "42", "7", "system:user:create")   // in-domain
ok, err := authorizer.EnforcePlatform(ctx, "42", "platform:tenant:list") // dom="*"
ok, err := authorizer.IsPlatformAdmin(ctx, "42")                      // g2 verdict
```

### 8.4 Policy Maintenance (Backend of the Role/Authorization Management APIs)

Policies use **snapshot rebuild** semantics — delete old rows first, then batch-add new rows, avoiding incremental sync drift:

```go
// Save role permissions: rebuild p rows by (role, tenant); empty perms = clear all of that role's authorizations
_ = authorizer.RebuildRolePolicies("manager", "7", []string{"system:user:list", "system:user:query"})

// Delete role: clear its p rows
_ = authorizer.RemoveRolePolicies("manager", "7")

// Save user roles: rebuild g rows by (user, tenant)
_ = authorizer.RebuildUserGrouping("42", "7", []string{"manager", "auditor"})

// Delete a user's role in a tenant / delete a role's user binding in a tenant
_ = authorizer.RemoveUserGrouping("42", "7")
_ = authorizer.RemoveRoleGrouping("manager", "7")

// Idempotently add a single policy (for seed data initialization)
_ = authorizer.AddPolicyIfAbsent("p", "super_admin", "7", "*")     // p row
_ = authorizer.AddPolicyIfAbsent("g2", "1", "platform_admin")      // g2 row
```

> It is recommended to wrap one more facade layer on the business package (numeric ID ↔ string conversion + fail-closed wrapping), with handlers facing only business semantics — see devops-system `internal/authz`.

---

## 9. Multi-Tenant Middleware and Data Isolation

Package path `devops-infra/tenant`. Three layers of capability: request-header context injection → GORM automatic row-level isolation → controlled exemptions.

### 9.1 Tenant Context

```go
// Injection (already done by middleware; automatically carried when business code passes ctx across goroutines)
ctx = tenant.WithTenant(ctx, 7)          // tenant ID only; ID ≤ 0 returns ErrInvalidTenantID
ctx = tenant.With(ctx, 7, "tenant-7")    // ID + code

// Reading
tid, ok := tenant.TenantID(ctx)          // (7, true)
tenant, ok := tenant.From(ctx)           // Context{ID:7, Code:"tenant-7"}
tid, err := tenant.RequiredTenantID(ctx) // not set → ErrTenantNotFound
code, err := tenant.RequiredTenantCode(ctx) // not set or empty code → ErrTenantNotFound
```

### 9.2 huma Tenant Middleware

```go
// The tenant comes from JWT claims (after HumaAuthenticate):
// the check callback injects tenant usability verdicts (e.g. DB status query + Redis cache); onUnauthorized/onForbidden
// being nil terminates with a unified envelope.
mw := tenant.HumaTenantContext(check, nil, nil)
```

On verdict failure (tenant deactivated/expired/nonexistent) the middleware does not call `next`, and the business-side callback (or default envelope) presents 403.

> Once the check callback returns false the request is denied; all downstream queries protected by the GORM plugin still **error** for lack of tenant context (fail-closed, see below).

A valid tenant is injected into the context via `tenant.With` and the whole context is replaced with `huma.WithContext`; downstream handlers / GORM can both read it.

### 9.3 GORM Automatic Tenant Isolation Plugin

```go
import "devops-infra/tenant"

if err := tenant.RegisterPlugin(gormDB); err != nil {   // called once at assembly time
    log.Fatal(err)
}
```

After registration, it takes effect automatically for **models containing the `tenant_id` field**, with zero changes to business code:

| Operation | Injected behavior |
| --- | --- |
| `Create` | auto-fills `tenant_id` (when the struct's field is zero; explicit assignment takes precedence, batch creation also supported) |
| `Query` | auto-appends `WHERE tenant_id = ?` |
| `Update` | auto-appends `WHERE tenant_id = ?` |
| `Delete` | auto-appends `WHERE tenant_id = ?` |

fail-closed guarantees:

- On query/update/delete with no tenant in the context, the statement returns `ErrContextMissing` and **the query never executes**, eliminating bare cross-tenant queries;
- For models without the `tenant_id` field (e.g. global config tables) → no injection, executes as usual;
- Queries with `Joins` are not auto-filtered for now (join semantics are complex; handle explicitly with `Scope`).

```go
// handler / service layer: just pass gin's ctx through
func (s *UserSvc) List(ctx context.Context) ([]User, error) {
    var users []User
    err := s.db.WithContext(ctx).Where("status = ?", 1).Find(&users).Error
    // Actual SQL: SELECT * FROM users WHERE status = 1 AND tenant_id = 7
    return users, err
}
```

### 9.4 Manual Filtering and Controlled Exemptions

For scenarios not going through GORM callbacks (native SQL, cross-table joins, tests), use `Scope` to manually append tenant conditions:

```go
db := tenant.Scope(gormDB, ctx, "")        // column name defaults to "tenant_id"
db := tenant.Scope(gormDB, ctx, "org_id")  // custom column name
// when the context has no tenant, the returned db already has ErrTenantNotFound injected; later execution always errors — likewise fail-closed
```

Global tasks that genuinely need cross-tenant access (tenant management, data aggregation, scheduled inspection) must **explicitly exempt**:

```go
// Option one: derive a ctx that skips filtering (tenant info retained, for display only)
db.WithContext(tenant.WithSkip(ctx)).Find(&tenants)

// Option two: block-level exemption, disabling isolation during fn's execution
err := tenant.ExecuteIgnore(ctx, func(ctx context.Context) error {
    return db.WithContext(ctx).Find(&allTenants).Error
})
```

Exemptions are **explicit declarations** — the locations where `WithSkip`/`ExecuteIgnore` appear during code review are the locations needing focused privilege-escalation checks.

### 9.5 Tenant Isolation Boundary Cheat Sheet

| Scenario | Behavior |
| --- | --- |
| Querying a table with `tenant_id` without tenant context | errors (`ErrContextMissing`), not executed |
| `tenant_id` unassigned on create | auto-filled with the context tenant |
| `tenant_id` explicitly assigned on create | explicit value retained (not overwritten) |
| Global tables (no `tenant_id` field) | plugin does not intervene |
| `Joins` queries | plugin skips; manual `Scope` needed |
| `WithSkip` / `ExecuteIgnore` | skips filling and filtering |

---

## 10. Middleware Orchestration Best Practices

### 10.1 Recommended Route Layering (Aligned with devops-system Practice)

```go
func Register(...) httpinfra.Register {
    return func(r *gin.Engine) {
        // Public endpoints: JWKS, login, refresh (put first to avoid being intercepted by authentication)
        r.GET("/.well-known/jwks.json", jwksHandler)

        // huma chain mounted in order (example: JWT → tenant → in-domain permission → handler)
        huma.Register(api, huma.Operation{
            Method: http.MethodGet, Path: "/api/v1/users",
            Middlewares: huma.Middlewares{
                securitymw.HumaAuthenticate(authn, opts),
                tenant.HumaTenantContext(check, nil, nil),
                securitymw.HumaRequirePermission(authz, "system:user:list", opts),
            },
        }, userH.List)

        // Platform routes: JWT → PlatformPerm, no tenant middleware (operate on global tables)
        huma.Register(api, huma.Operation{
            Method: http.MethodGet, Path: "/api/v1/tenants",
            Middlewares: huma.Middlewares{
                securitymw.HumaAuthenticate(authn, opts),
                securitymw.HumaRequirePlatformPermission(authz, "platform:tenant:list", opts),
            },
        }, tenantH.List)
    }
}
```

### 10.2 Checklist

- [ ] **Order**: `HumaAuthenticate` → `HumaTenantContext` → `HumaRequirePermission` → handler; permission middleware always registers before the handler;
- [ ] **Envelope consistency**: `HumaOptions` callbacks uniformly go through `response.WriteHumaEnvelope` (the default callbacks also write envelopes when not injected), same format as business errors;
- [ ] **503 is not 401**: Redis/Casbin failures go through `OnInfrastructureError` returning 503; do not disguise infrastructure failures as "not logged in" and mislead the frontend into jumping to the login page;
- [ ] **Every request goes through session verification**: do not bypass `SessionStore.Active` for performance — this is key to instant kick-offline;
- [ ] **Platform permission identifier isolation**: `platform:*` permissions only on platform routes, never in tenant menu tables;
- [ ] **Explicit exemptions**: `WithSkip`/`ExecuteIgnore` may only appear in global management code; forbidden in business queries;
- [ ] **Refresh rotation**: the refresh endpoint creates the new session before revoking the old sid; revocation failure is only logged (the old session expires naturally);
- [ ] **Cross-service propagation**: put tenant context into gRPC metadata / message headers; the consumer rebuilds with `tenant.With`; do not rely on implicit propagation.

---

## 11. Unit Test Support

Security components are all interface-oriented, so stubs suffice for testing, with no real Redis / Casbin needed:

```go
type stubTokenParser struct{ claims *securityjwt.Claims; err error }
func (s *stubTokenParser) ParseAccess(token string) (*securityjwt.Claims, error) { return s.claims, s.err }

type stubSessionStore struct{ active bool; err error }
func (s *stubSessionStore) Active(ctx context.Context, ref securitysession.SessionRef) (bool, error) {
    return s.active, s.err
}

type stubAuthorizer struct{ enforceOK, platformAdmin bool; _ error }
func (s *stubAuthorizer) Enforce(ctx context.Context, sub, dom, obj string) (bool, error) { return s.enforceOK, nil }
func (s *stubAuthorizer) EnforcePlatform(ctx context.Context, sub, obj string) (bool, error) { return true, nil }
func (s *stubAuthorizer) IsPlatformAdmin(ctx context.Context, sub string) (bool, error) { return s.platformAdmin, nil }
```

Complete compilable examples are in `security/middleware/gin_test.go` (covering all branches: missing header, invalid token, session invalidated, infrastructure error, permission denied). Session store tests can use `miniredis` (already bundled in devops-infra's test dependencies). Gin route tests use `gin.TestMode` + `httptest`.

---

## 12. Data Permissions and Middleware Suite

Two directly reusable business-level building blocks: row-level data permission computation (`datascope`) and huma middleware chain assembly (`security/humakit`).
Already in use by devops-system; **new modules (e.g. devops-user) can `require devops-infra` and reuse directly, no copying needed**.

### 12.1 `datascope`: Pure Row-Level Data Permission Computation

Three-layer division of labor: Casbin decides "can this endpoint be called", `tenant` decides "inside or outside the tenant", `datascope` decides "which rows inside the tenant can be seen/operated". This package is **pure computation, no DB access** — role/user/dept values are queried and injected as value objects by the caller.

Semantics (aligned with yudao `t_system_role.data_scope`): `1 all / 2 custom departments / 3 own department / 4 own department and below / 5 self only`. Multiple roles take the most permissive; same-custom takes the union.

```go
// Parsing: convert business query results into value objects, then merge
scope := datascope.Merge(roleScopes, user.DeptID, user.ID, datascope.ChildrenMap(deptNodes))

// Consumption: list query containment (fail-closed — errors when ctx is not injected, never widened to fully visible)
db = datascope.Apply(ctx, db, datascope.Columns{Dept: "dept_id", Owner: "id"})

// Consumption: single-point visibility (privilege escalation returns 404, avoiding existence probing)
if !scope.Visible(row.DeptID, row.ID) { return errs.NotFound(...) }
```

### 12.2 `security/humakit`: Middleware Chain Suite

Chain composition for `JWT → Tenant → Perm → Scope` plus data permission resolution. Decisions live in `security/middleware` and `tenant`; this package only does assembly; **text and business data sources are always injected via `Deps`/`Envelope`**, with no default business binding (nil is always fail-closed).

```go
kit := humakit.NewKit(authn, humakit.Deps{
    Authorizer:      authorizer,
    TenantUsable:    myTenantChecker,                    // deactivated/expired semantics decided by the business
    UserRoles:       myRoleScopes,                       // → []datascope.RoleScope
    User:            myUserRef,                          // → datascope.UserRef
    Depts:           myDeptNodes,                        // → datascope.DeptNode
    SuperAdminRole:  "super_admin",                      // same source as the Casbin seed
    WithClaims:      injectAuditActor,                   // ctx enhancement after authentication (optional)
    Envelope:        myEnglishEnvelope,                  // 401/403 English text (optional)
})

huma.Register(api, huma.Operation{
    Middlewares: kit.ScopedBiz("system:user:list"),      // JWT→Tenant→Perm→DataScope
}, handler)
```

Chain composition: `Logged()` (authn only) / `TenantBiz()` (authn+tenant) / `Biz(perm)` / `ScopedBiz(perm)` (adds data permissions) / `PlatChain(perm)` (platform, no tenant). After role/data changes, call `kit.InvalidateDataScope()` to clear the cache.

### 12.3 `security/authz`: Role Name Config and Seed

Role names are no longer hardcoded: `RoleConfig{PlatformAdmin, SuperAdmin}` is injected via `New(gdb, WithRoles(cfg))`, with **seed writes and `IsPlatformAdmin` verdicts sharing the same source** (otherwise you get the "seed writes a custom name, verdict queries platform_admin" brain split). Defaults to `platform_admin` / `super_admin` when not passed.

```go
a, _ := authz.New(gdb, authz.WithRoles(authz.RoleConfig{
    PlatformAdmin: "ops_admin", SuperAdmin: "ops_super",
}))
_ = a.SeedBootstrap(adminUID, tenantID)   // platform + tenant super admin initial policies (idempotent)
_ = a.SeedTenantAdmin(adminUID, tenantID) // super admin policy on tenant creation (idempotent)
```

> Changing role names requires considering migration of existing `casbin_rule` data.

---

## 13. Appendix: API Cheat Sheet

### HTTP (`devops-infra/http`)

| API | Description |
| --- | --- |
| `http.Run(cfg, register, readiness...)` | start the HTTP service (Recovery/AccessLog/healthz/readyz/graceful shutdown) |
| `http.ReadinessCheck{Name, Check}` | readiness check item |
| `middleware.Recovery()` | panic → 500 unified envelope |
| `middleware.AccessLog()` | structured access log (Debug) |

### Response (`devops-infra/http/response`)

| API | Description |
| --- | --- |
| `response.OK / OKPage` | success envelope / paginated envelope |
| `response.Fail / FailErr` | failure exit / unified error exit |
| `response.Wrap(handler)` | `func(c) error` → `gin.HandlerFunc` |
| `response.BaseResponse[T] / PageData[T]` | envelope structs |

### gRPC (`devops-infra/grpc`)

| API | Description |
| --- | --- |
| `grpc.Run(cfg, register)` | start the gRPC service (Recovery/AccessLog/health/reflection/graceful shutdown) |
| `grpc.Dial(ctx, endpoint, timeout)` | non-blocking dialing, built-in Unavailable retries |
| `grpc.IsUnavailable(err) / Close(conn)` | error classification / close connection |
| `interceptor.Recovery() / AccessLog()` | take interceptors individually |

### JWT (`devops-infra/security/jwt`)

| API | Description |
| --- | --- |
| `NewManager(Config) (*Manager, error)` | build the signing manager |
| `Manager.Issue(ctx, Subject, sid) (TokenPair, error)` | sign an RS256 access token |
| `Manager.ParseAccess(token) (*Claims, error)` | verify and parse |
| `Manager.JWKS() (map[string]any, error)` | output the JWKS document |
| `PublicKeyFromPrivatePEM(pem) (*rsa.PublicKey, error)` | export public key from private key |
| `NewStaticKeySet(map[kid]*rsa.PublicKey)` | static public key set |
| `JWKSDocument(pub, kid)` | generate a JWKS individually |

### Sessions (`devops-infra/security/session`)

| API | Description |
| --- | --- |
| `NewRedisStore(cacheClient, ttl) / NewRedisClientStore(goRedis, ttl)` | build the store |
| `Store.Create / Active / Lookup / Matches` | create session / liveness / reverse lookup / ownership check |
| `Store.Revoke / RevokeUser / RevokeTenant` | single / all of a user / all of a tenant revocation |

### Authentication & Authorization Middleware (`devops-infra/security/middleware`)

| API | Description |
| --- | --- |
| `NewAuthenticator(TokenParser, SessionStore) *Authenticator` | framework-agnostic authenticator |
| `Authenticator.Authenticate(ctx, bearerHeader)` | authentication decision, three error classes via `errors.Is` |
| `HumaAuthenticate(authn, opts)` | huma authentication middleware |
| `HumaRequirePermission(authz, perm, opts)` | in-domain authorization (uid × tid × perm) |
| `HumaRequirePlatformPermission(authz, perm, opts)` | platform authorization (g2 + dom="*") |
| `HumaCurrentClaims(ctx) *securityjwt.Claims` | get current identity in handlers |
| `HumaOptions{OnUnauthorized, OnForbidden, OnInfrastructureError}` | error presentation callbacks (default envelope when nil) |

### Casbin (`devops-infra/security/authz`)

| API | Description |
| --- | --- |
| `New(gormDB) (*CasbinAuthorizer, error)` | initialization (gorm-adapter persistence) |
| `Enforce / EnforcePlatform / IsPlatformAdmin` | authorization decisions |
| `RebuildRolePolicies / RemoveRolePolicies` | role permission snapshot rebuild / clear |
| `RebuildUserGrouping / RemoveUserGrouping / RemoveRoleGrouping` | user-role binding maintenance |
| `AddPolicyIfAbsent("p"/"g"/"g2", values...)` | idempotently add policy (seed data) |

### Multi-Tenant (`devops-infra/tenant`)

| API | Description |
| --- | --- |
| `HumaTenantContext(check, onUnauthorized, onForbidden)` | huma tenant injection middleware (claims → check → ctx) |
| `With / WithTenant / From / TenantID` | context injection and reading |
| `RequiredTenantID / RequiredTenantCode` | strict reading (error when missing) |
| `RegisterPlugin(gormDB)` | GORM auto-fill + row-level filter callbacks |
| `Scope(db, ctx, column)` | manually append tenant conditions |
| `WithSkip / ExecuteIgnore / IsSkip` | controlled exemptions |

### Error Codes (`devops-infra/errs`)

| API | Description |
| --- | --- |
| `CodeOK/Param/Unauthorized/Forbidden/NotFound/MethodNotAllowed/Internal` | aligned with HTTP status codes |
| `errs.New(code, msg) / Wrap(err, code, msg)` | business error construction (retains underlying cause) |
| `errs.ErrParam / ErrUnauthorized / ErrForbidden / ...` | predefined errors |

### Data Permissions (`devops-infra/datascope`)

| API | Description |
| --- | --- |
| `ScopeAll/Custom/Dept/DeptBelow/Self` | data scope semantic constants (aligned with yudao) |
| `Merge(roles, ownDeptID, selfID, children) Scope` | multi-role merge (most permissive wins) |
| `Apply(ctx, db, Columns)` | inject filter into list queries (fail-closed) |
| `(Scope).Visible(deptID, rowID)` | single-point visibility (privilege escalation returns 404) |
| `ParseDeptIDs / NormalizeDeptIDs` | storage form of custom department sets |
| `ChildrenMap / ExpandSubtree` | department tree expansion |
| `WithScope / From / CtxKey` | context injection and reading |

### Middleware Suite (`devops-infra/security/humakit`)

| API | Description |
| --- | --- |
| `NewKit(authn, Deps) *Kit` | assemble the middleware suite (business deps injected via Deps) |
| `JWT() / Tenant() / Scope() / Perm(p) / Plat(p)` | single middleware |
| `Logged() / TenantBiz() / Biz(p) / ScopedBiz(p) / PlatChain(p)` | chain composition |
| `InvalidateDataScope()` | clear the data permission resolution cache |
| `Deps{Envelope, TenantUsable, UserRoles, ...}` | text and business data source injection (nil is always fail-closed) |

### Role Name Config (`devops-infra/security/authz`)

| API | Description |
| --- | --- |
| `RoleConfig{PlatformAdmin, SuperAdmin}` | role name config (zero values backfill defaults) |
| `New(gdb, WithRoles(cfg))` | initialization with role names |
| `SeedBootstrap(subject, domain)` | platform + tenant super admin initial policies (idempotent) |
| `SeedTenantAdmin(subject, domain)` | tenant super admin policy (idempotent) |
| `(CasbinAuthorizer).Roles()` | the currently effective role name config |

---

*This document corresponds to the current devops-infra implementation; code examples are subject to the actual exported APIs in the packages. Reference implementations are in `devops-system/internal/{auth,authz,middleware,router,tenantdb,dao/scope_source.go}`.*
