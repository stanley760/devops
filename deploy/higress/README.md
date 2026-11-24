# Higress Gateway Configuration and Deployment

This directory is the **single source of truth for Higress gateway config**: `mcpbridges/`, `ingresses/`, `wasmplugins/`
are the resource directories recognized by the Higress file config backend, and these yaml files also serve as the
manifest reference for migrating to Kubernetes. This document describes how to configure each capability, plus the three
deployment forms: dev / docker production / K8s.

```
deploy/higress/
├── mcpbridges/default.yaml          # Service source: DNS service sources (devops-system / devops-user / devops-system-gray)
├── ingresses/devops-system.yaml     # Route: api.devops.local -> devops-system
├── ingresses/devops-system-canary.yaml  # Canary route (gray release, triggered by the x-canary tag)
├── ingresses/devops-system-gray-route.yaml  # Dedicated gray domain: gray.devops.local goes 100% direct
├── services/higress-gateway.yaml    # all-in-one built-in Service declaration (K8s migration reference)
├── wasmplugins/cors-1.0.0.yaml      # CORS: handled centrally at the gateway (console export format)
└── wasmplugins/geo-ip-1.0.0.yaml    # GeoIP: injects X-Higress-Geo-* headers (consumed by login audit)
```

> `.gitignore` uses a whitelist: only yaml under `mcpbridges/`, `ingresses/`, `wasmplugins/`
> is tracked. State files the console writes at runtime to `/data` (configmaps/secrets/envoyfilters, etc.)
> are not committed — see "Config Management Conventions".

---

## 1. Architecture and Deployment Forms

| Form | Image/method | Use case | Doc section |
|---|---|---|---|
| dev | all-in-one single container + named volume | local development | Section 2 below |
| docker production | controller + gateway + console split + file config backend | single-host/small-scale production | Section 8 below |
| K8s | Helm deployment (see `deploy/k8s/devops.yaml`) | recommended for formal production | Section 9 below |

Regardless of the form, the business traffic path is the same:

```
Client ──▶ Higress Gateway(:8080/:8443)
              │  jwt-auth / cors / cluster-key-rate-limit / geo-ip
              ▼
           DNS service source (K8s-style FQDN) ──▶ devops-system / devops-user / devops-system-gray
```

**Security boundary**: `devops-system:9999` is not published to host ports or public load balancers; ops endpoints
(`/metrics` `/apidoc` `/healthz` `/readyz`) are not exposed via the public gateway; the Ingress routes only
`/system` and `/.well-known/jwks.json`.

> ⚠️ "Not published to the host" is only the **first** line of defense and is not enough to guarantee unified traffic ingress.
> Any container on the same network (such as the MySQL, Redis containers) can reach `devops-system:9999`
> directly, bypassing the gateway, at which point all gateway-layer capabilities fail: rate limiting is void, GeoIP
> header injection disappears (all four `X-Higress-Geo-*` fields empty in the login audit), gray release is void, CORS is void.
> Hence this project adopts a **three-layer containment**, detailed in section 7.1.

---

## 2. Dev Environment (all-in-one + Named Volume)

### 2.1 Why a Named Volume Instead of a Bind Mount

The all-in-one container continuously writes state to `/data` (pilot, console, built-in observability components). Under
Windows, a bind mount crosses the NTFS→WSL2 filesystem boundary, and high-frequency small writes can peg CPU at 100% until
the Docker engine becomes completely unresponsive (500 errors). Hence the dev compose uses the named volume `higress-data`.

### 2.2 Config Seeding and Sync

The named volume starts empty; config seeding is done by the compose `higress-init` one-shot service: before higress starts it
copies the three git-managed directories `mcpbridges/`, `ingresses/`, `wasmplugins/` from this directory into
the named volume (defined in `docker-compose-dev.yml`, busybox image, exits when done). Therefore:

- `up` only re-runs init seeding when the volume is empty / after `down` (compose does not re-run completed containers);
- explicit sync after editing yaml in this directory (stop gateway → re-seed → fix the toxic config → start gateway):

```bash
sh scripts/dev.sh sync
# Force-rerun higress-init seeding, and remove the auto-regenerated
# inclusionRegexps: [.*] in higress-config (see the pitfall table in 2.3), then restart the gateway
```

> init overwrites these three directories every time. **Convention: always edit the yaml in this directory and then sync; the console
> is for viewing only.** Move any route/plugin changes made in the console back to yaml first, otherwise the next `up` will overwrite and lose them.
> Other state the console writes to `/data` (configmaps/secrets, etc.) is unaffected.

### 2.3 Known Pitfalls (Troubleshooting Manual)

| Symptom | Cause | Fix |
| --- | --- | --- |
| Console "service sources" shows the default source instead of the git config | The volume holds the default McpBridge generated on the all-in-one's first start; the git config was never seeded | Run `sync`; confirm `/data/mcpbridges/default.yaml` lists the three dns sources |
| Route 404 / Ingress not effective | Same as above; `/data/ingresses/` is empty | Same as above |
| Docker engine 500 / abnormally high CPU | ① Windows bind mount (already switched to a named volume); ② the all-in-one's built-in prometheus/loki/promtail/grafana run in parallel with the external stack; ③ `proxyStatsMatcher.inclusionRegexps: [.*]` in `higress-config` causes Envoy full high-cardinality metrics | Use the named volume; disable the built-in observability components (see "8. Observability"); `sync` already includes this fix (see the row below) |
| envoy disappears ~1 minute after startup; 8080 TCP connects but returns empty (8001 is fine) | `inclusionRegexps: [.*]` in `higress-config` (**auto-regenerated on every first start of a new volume**, and the file has CRLF line endings) → envoy's stats tables blow up memory and it gets OOM-killed by cgroup; supervisord treats its exit 0 as expected and does not auto-restart it | `sh scripts/dev.sh sync` (stop, delete the config, then start); the dev compose memory limit has been raised to 2g as a backstop |
| Processes inside all-in-one batch exited | The supervisor shutdown sequence of container stop/restart; logs look like `exited: prometheus (exit status 0; expected)` | First check `docker compose ps` to see whether the container is in a restart loop |
| Pages intermittently return **503 Service Unavailable** (roughly a fixed percentage of requests) | The canary Ingress uses `higress.io/canary-weight`, and the gray instance is not running → that cluster has no healthy endpoints in CDS → Envoy picks no upstream → `response_flags: NC`. `weighted_clusters` has **no** fallback to the baseline cluster, so the hit requests fail directly | Change the canary Ingress to tag-based gray release (`canary-by-header` / `canary-by-cookie`, Higress's `cluster_fallback` plugin degrades gracefully) → see section 5; or start the gray instance to make the cluster exist. Confirm via `curl -s localhost:15000/clusters?format=json` (the gray cluster should appear with the correct port) and `curl -s localhost:15000/stats \| grep no_cluster` |
| The gray instance reports `mysql.host must not be empty` at startup | The config file name is derived from the service name; the gray instance runs under the identity `devops-system-gray` but should reuse the main instance's config | Declare `service.config_name: devops-system` in the gray bootstrap (see `devops-system/config/bootstrap-gray.yaml` / `deploy/config/system-gray-bootstrap.yaml`) |
| `higress.io/destination` without a port | Envoy parses the target as port 8080, which is not the port the backend actually listens on → the request cannot reach the backend | Write the port explicitly: `devops-system.dns:9999` / `devops-system-gray.dns:9998` |

---

## 3. Service Source: McpBridge (DNS Service Sources)

`mcpbridges/default.yaml` declares each backend as a DNS service source (Nacos removed; discovery is platform-native — compose network alias / extra_hosts, or the K8s Service DNS):

```yaml
spec:
  registries:
    - name: devops-system                 # B 端管理后台；服务标识 devops-system.dns
      type: dns
      domain: devops-system.devops.svc.cluster.local   # K8s-style FQDN
      port: 9999
    - name: devops-user                   # C 端用户服务；服务标识 devops-user.dns
      type: dns
      domain: devops-user.devops.svc.cluster.local
      port: 10000
    - name: devops-system-gray            # 灰度实例；服务标识 devops-system-gray.dns
      type: dns
      domain: devops-system-gray.devops.svc.cluster.local
      port: 9998
```

Key points:

- The FQDN resolves per deployment platform: dev compose maps it to the host LAN
  IP via the `higress` service's `extra_hosts` (`HOST_IP`, see `deploy/.env`);
  prod compose attaches it as a network alias (`networks.edge.aliases`); on K8s
  it is the Service DNS and resolves natively.
- Destination reference format: `<registry-name>.dns:<port>` — e.g.
  `devops-system.dns:9999`. The port must be written explicitly (see the pitfall table).
- On real K8s the same FQDNs are the Service DNS; only the McpBridge type changes
  from `dns` to the in-cluster K8s source (or is left to Higress's native discovery).

---

## 4. Routing: Ingress

`ingresses/devops-system.yaml`:

```yaml
apiVersion: networking.higress.io/v1  # actually networking.k8s.io/v1, see the file
kind: Ingress
metadata:
  name: devops-system
  namespace: higress-system
  annotations:
    # Target address format: <registry-name>.dns (the port must be explicit)
    higress.io/destination: devops-system.dns:9999
spec:
  ingressClassName: higress
  rules:
    - host: api.devops.local
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              resource:
                apiGroup: networking.higress.io
                kind: McpBridge
                name: default
```

Key points:

- `higress.io/destination` points to the service declared in McpBridge; the FQDN resolves per platform
  (extra_hosts / compose alias / K8s Service DNS), with no route changes needed.
- `host` is the external domain; verify locally with `curl -H 'Host: api.devops.local'
  http://localhost:8080/healthz` or add a hosts mapping.
- Route only the business API and JWKS — **do not** hang ops endpoints such as `/metrics` on the public gateway.
- Higress supports standard Ingress annotations (rewrite, timeout, etc.) and `httproutes` (the newer protocol);
  for new capabilities, check the official Higress annotation docs first before deciding whether to introduce a custom EnvoyFilter.

---

## 5. Gray Release

`ingresses/devops-system-canary.yaml`: an Ingress with the same host and path plus the canary
annotation splits traffic:

```yaml
metadata:
  annotations:
    higress.io/canary: "true"
    # Gray is triggered explicitly by a tag; the default 100% stays on the baseline instance
    higress.io/canary-by-header: "x-canary"           # Header-based gray release
    higress.io/canary-by-header-value: "true"          # Gray release only on exact value match
    higress.io/canary-by-cookie: "canary"              # Cookie-based gray release
    # The port must be written explicitly (the gray instance listens on 9998)
    higress.io/destination: devops-system-gray.dns:9998
```

Three splitting methods, with priority **Header > Cookie > Weight**:

```yaml
higress.io/canary-by-header: "x-canary"           # Header-based gray release
higress.io/canary-by-header-value: "true"          # Gray release only on exact value match
higress.io/canary-by-cookie: "canary"              # Cookie-based gray release
higress.io/canary-weight: "10"                     # Fallback weight-based gray release
```

> ⚠️ **Do not use `higress.io/canary-weight` when the gray instance may not be running.**
> The weight split is unconditional: a request hitting the gray cluster is sent there even if the
> gray instance is not running — that cluster has no healthy endpoints, and Envoy has no
> upstream to pick → `503` + `response_flags: NC` (no cluster). Because the split is random, roughly
> `weight%` of all page requests fail intermittently, and the failure looks unrelated to the gray
> release. Measured on this project: 300 requests → 269×200 + **31×503 (10.3%)**, exactly matching
> `canary-weight: "10"`. Note that Envoy's `weighted_clusters` has **no** fallback to the baseline
> cluster, whereas Higress's header/cookie canary routes carry the `cluster_fallback` plugin
> (routing_cluster = gray, fallback_clusters = baseline) — a request carrying the gray tag but no
> available gray instance transparently falls back to the baseline instead of failing.
>
> So the rule is: **use Header/Cookie tags for gray release** (explicit, and degrades gracefully),
> and only use the weight split when the gray instance is confirmed running.

Procedure: start the `gray` instance (identity `devops-system-gray`, see `deploy/config/system-gray-*.yaml`) →
edit the canary yaml (add the tag annotation) → `sync` (dev) or apply the manifest (production) →
request with the tag header to confirm the gray instance is hit → when fully rolled out, publish the
new version under the name `devops-system` and delete the canary route.

**Gray instance targeted access** (not weight-random): `ingresses/devops-system-gray-route.yaml` provides
a dedicated `gray.devops.local` route, 100% direct to the gray instance, for gray version verification. Starting the gray instance locally:

```bash
cd devops-system && go run . server -f config/bootstrap-gray.yaml   # :9998, registers as devops-system-gray
curl -i -H 'Host: gray.devops.local' http://localhost:8080/api/v1/auth/info   # 401 means it reached the backend
```

> The gray instance registers as `devops-system-gray` but reads the main instance's business config.
> Because the config file name is derived from the config-side service name, the gray bootstrap must
> declare `service.config_name: devops-system` (see `deploy/config/system-gray-bootstrap.yaml`);
> otherwise the name is derived from `devops-system-gray`, no config file is found, and the instance
> fails at startup with `mysql.host must not be empty`.

Note: the console's "route management" does not show canary Ingresses as separate entries (they are merged into the main route's
split rules); whether gray release is in effect is determined by envoy's actual config/traffic:
`docker compose -f deploy/docker-compose-dev.yml exec higress sh -c 'curl -s localhost:15000/config_dump | grep -c devops-system-gray'`.

---

## 6. Wasm Plugins

Plugins bind to specific routes via `matchRules.ingress`; `defaultConfigDisable: true` means
unmatched routes are not affected.

**Route names carry no namespace prefix**: the all-in-one's file backend uses the filename as the resource name to generate routes,
so the route name in envoy is `devops-system` (not the K8s-mode
`higress-system/devops-system`). Writing matchRules as `higress-system/xxx` will never match
and the plugin will **silently do nothing** (no logs at all) — this was confirmed by hands-on testing after hitting this pitfall. To verify whether a plugin
really executes: pre-flight OPTIONS expecting 204 + `access-control-*` headers; for rate limiting, look at the `x-ratelimit-*`
headers in the response.

**The console's "plugin config" page not showing hand-written yaml as "active" is normal**: that page only recognizes resources with
`higress.io/resource-definer: higress`, `higress.io/wasm-plugin-built-in: "true"`,
`higress.io/wasm-plugin-name` and other labels (i.e. those created/exported from the console, such as
geo-ip-1.0.0.yaml / cors-1.0.0.yaml, both console export formats with labels); the hand-written
cluster-key-rate-limit.yaml lacks these labels, so the console does not categorize it, but envoy
loads and executes it as usual. The `config_dump` is authoritative:
`docker compose -f deploy/docker-compose-dev.yml exec higress sh -c 'curl -s localhost:15000/config_dump' | grep higress-system.cors`.

### 6.1 CORS (cors.yaml)

CORS is handled centrally at the gateway and **the application no longer implements CORS**, avoiding duplicate response headers. When adding an origin domain, edit
`allow_origins` and sync to take effect:

```yaml
matchRules:
  - ingress: [devops-system, devops-system-gray-route]   # Route names, no namespace prefix
    config:
      allow_origins: ["http://localhost:5173", "http://localhost:3000"]
      allow_methods: [GET, POST, PUT, PATCH, DELETE, OPTIONS]
      allow_headers: [Content-Type, Authorization, X-Requested-With]
      allow_credentials: true
```

### 6.2 Cluster Rate Limiting (cluster-key-rate-limit.yaml)

Redis-based **cluster-level** rate limiting (counter shared across gateway instances); the current policy is 100 QPS per IP,
returning 429 when exceeded.

Two pitfalls you will certainly hit:

1. **The config after a matchRules hit replaces defaultConfig entirely**, so the `redis` connection block must be
   repeated in every matched rule, otherwise the plugin fails to load with `missing redis in config` and is fail-closed.
2. **`redis.service_name` points to a service in the Higress service source**, not the compose service name.
   In the compose environment, a DNS service source must first be registered in mcpbridges/default.yaml, and 2.2.3's
   McpBridge does not support `protocol: redis` (reports `invalid protocol:redis`) — `tcp` works:

```yaml
# mcpbridges/default.yaml — redis service source
registries:
  - name: devops-redis
    type: dns
    protocol: tcp            # the redis protocol is not recognized in 2.2.3; tcp works
    domain: redis.devops.local   # must be a full dotted domain name; corresponds to the redis network alias in compose
    port: 6379
```

```yaml
# cluster-key-rate-limit.yaml
matchRules:
  - ingress: [devops-system, devops-system-gray-route]
    config:
      redis:
        service_name: devops-redis.dns   # dns service source reference format: <registry-name>.dns
        service_port: 6379
        password: "123456"               # injected from secrets management in production, never committed in plaintext
      rule_items:
        - limit_by_per_ip: from-remote-addr
          limit_keys:
            - key: 0.0.0.0/0
              query_per_second: 100
      rejected_code: 429
```

The `redis.devops.local` alias is declared on the redis service in docker-compose-dev.yml
(`networks.default.aliases`), and Docker's embedded DNS resolves it to the redis
container within the compose network.

Tuning dimensions: for Header/Cookie-based rate limiting change `limit_by_per_header` / `limit_by_per_cookie`;
for different thresholds per key group, add entries under `limit_keys`.

### 6.3 GeoIP (geo-ip-1.0.0.yaml)

Resolves the client IP's location in the AUTHN phase, injecting `X-Higress-Geo-Country/-Province/
-City/-Isp` request headers after URL escaping; for intranet IPs the value is "Intranet IP". The consumer is the devops-system login audit:
the `withClientMeta` middleware in `router/operations.go` unescapes these four headers + gin ClientIP,
and writes them with the login into `t_system_login_log` (geo fields). The IP itself is carried separately by the X-Forwarded-For chain,
independent of this plugin. Source is `ip_source_type: origin-source` (the downstream connection's real source; the client
connects directly to 8080 with no front LB); if another proxy layer is added in front, change to `header` + the corresponding header and make sure that layer writes it.

The audit read side is the admin endpoint `/api/v1/login-logs` (paginated query/delete/clear, permission
`system:loginlog:list|delete|clean`, menu seed in
`internal/dbinit/sql/data/t_system_menu.sql`); before persistence the dao layer truncates UA/Geo headers by
column width, so overlong headers never lose audit rows.

### 6.4 JWT Authentication (implemented: `wasmplugins/jwt-auth-2.0.0.yaml`)

The first gateway-side verification, done by the `jwt-auth` plugin:

1. Reads the JWT from `Authorization: Bearer <token>`, verifying the signature, `issuer=devops`, `exp`, `iat`;
2. `claims_to_headers` forwards the `uid` / `username` / `tid` / `tcode` claims as internal headers,
   with `override=true` overwriting same-named client headers, preventing forged tenant identity;
3. The verification public key is **inlined** in the plugin yaml's `jwks` field (RS256 public key; the plugin file contains only the public key and is safe to commit).
   ⚠️ Higress jwt-auth **has no `jwksFromUrl` field**, so the gateway will **not** fetch from
   `/.well-known/jwks.json` — that endpoint only serves "other services' self-verification" and manual troubleshooting.

Key strategy: system holds the RS256 private key for signing; Higress only configures the public key JWKS.
**The JWT private key must never be written into public config files in this directory.** After rotating keys, the public key in the plugin must be re-exported
(`openssl rsa -noout -modulus` exports n, `e=65537`), otherwise everything returns 401.

After gateway authentication passes, devops-system's JWT middleware must still be kept: system also handles Redis session
revocation checks, tenant status checks and tenant context injection, and must not trust requests based solely on `X-Tenant-ID`.

The plugin config's three pitfalls and debugging criteria are in "7.3 Second Layer: JWT Verification".

---

## 7. Unified Traffic-Ingress Management (Three-Layer Containment)

### 7.1 Why Containment Is Needed

As long as `devops-system:9999` is reachable inside the container network, any compromised or misconfigured container can reach the backend directly.
In that case **JWT authentication is still effective** (it lives inside the application), but **all gateway-layer capabilities fail**:

| Broken capability | Consequence |
| --- | --- |
| `cluster-key-rate-limit` | Rate limiting becomes a rubber stamp; the backend can be hammered directly |
| `geo-ip` header injection | All four `X-Higress-Geo-*` fields empty in the login audit; the audit chain is broken and bypassed traffic cannot be distinguished from normal traffic |
| `canary` gray release | Gray release can be bypassed to hit the production instance directly |
| `cors` | Cross-origin policy is void |
| Access logs | Ingress logs are missing, so post-incident auditing is impossible |

### 7.2 The Three-Layer Model

| Layer | Mechanism | Enforcement | Effective form |
| --- | --- | --- | --- |
| Layer 1 Network | K8s NetworkPolicy | **enforced** | K8s |
| Layer 1 Network | compose `edge`/`infra` dual network + `internal: true` | partially enforced (blocks infrastructure-side bypass) | Docker |
| Layer 2 Gateway | `jwt-auth` plugin verifies the JWT (`jwt-auth-2.0.0.yaml`) | needs Layer 3 to complement | Docker / K8s |
| Layer 3 Application | `devops-infra/security/gateway` middleware independently verifies the JWT, failing with 401 | **enforced** | Docker / K8s |

**Docker form**: in compose, `devops-system` only does `expose: "9999"` with no `ports`, so host-level
bypass is already impossible; on top of that the `edge`/`infra` network isolation keeps MySQL/Redis/etcd containers off the business network.
The truly enforced measure is the Layer-3 guard — because compose cannot do a "gateway-only" ACL on a single port.

**K8s form**: Layer 1 NetworkPolicy truly closes the loop (the application-layer guard degrades to defense-in-depth).
In `deploy/k8s/networkpolicy.yaml`, `allow-gateway-to-system` makes 9999 accept only
Pods with `app=higress` from the `higress-system` namespace.

### 7.3 Second Layer: JWT Verification (`jwt-auth` Plugin)

For plugin config see `wasmplugins/jwt-auth-2.0.0.yaml`. Key points:

- `global_auth: false` (**mandatory**): `true` makes the plugin apply globally to **all** routes in the gateway,
  including the login endpoint → users have no token yet but are required to carry one → **nobody can log in; the system self-locks**.
- `matchRules.ingress` = `[devops-system, devops-system-gray-route]`,
  **deliberately excluding** `devops-system-public`.
- `keep_token: true`: keeps the JWT when forwarding to the backend. The backend still verifies Redis session revocation,
  tenant status and tenant context, and must not trust tenant identity based solely on the gateway-injected `X-Tenant-ID`.
- Authentication config (`consumers` / `jwks`) goes in the **instance-level `defaultConfig`**; rules only carry
  authz `config: { allow: [...] }` — the docs require that authentication and authorization cannot coexist within the same rule.

#### Three Pitfalls of Higress 2.2.3 all-in-one (none of them log errors)

1. **`url` cannot use `oci://`**. Under the file backend + all-in-one the controller does not pull images,
   so the plugin silently fails to load. You must use `http://localhost:8002/plugins/jwt-auth/2.0.0/plugin.wasm`
   (consistent with the other plugins in this directory).
2. **Inlining the `consumers` array in `matchRules.config` → the whole plugin is silently skipped**.
   pilot's parse failure is not reported, and requests pass straight through to the backend. Writing `config: {}` also fails —
   that replaces `defaultConfig` entirely, and even real tokens get `403 Access Denied`.
3. **`metadata.name` cannot be `jwt-auth`**. pilot would take the Istio native branch
   (logs show `--tokenManagerPlugin=""` and `JwtRule: ""`), and all-in-one 2.2.3
   recognizes neither `security.istio.io` resources nor an envoy `jwt_authn` filter, so nothing loads.
   Renaming it to `jwt-auth-2.0.0` works (the wasm file path is unaffected).

#### Why the anonymous endpoints need a separate ingress file

The plugin mounts by **Ingress resource**, and `matchRules` has only `ingress` / `domain` / `service`,
**with no path / prefix**. The main ingress's `/system` is a Prefix prefix that already covers the login paths —
if login were written in the same file, mounting the plugin would lock login out. Hence `login` / `refresh` / `jwks.json`
are split into `devops-system-public.yaml`, and Higress **longest prefix matching** makes these three more precise paths
hit it, thereby bypassing the plugin.

Verified (same host `api.devops.local`): `/system/api/v1/users` with no token →
`Jwt is missing` (blocked); `/system/api/v1/auth/login` → 200 (exemption works).

#### Whether the Plugin Is Really Loaded

**Do not look at config_dump's ECDS** — under all-in-one it is always empty, even for the effective cors. Reliable criteria:

```bash
docker compose -f deploy/docker-compose-dev.yml exec higress \
  sh -c 'grep -c "fetching image plugins/jwt-auth" /var/log/higress/gateway.log'
```

Or look at the response shape of a token-less request: **plain text** `Jwt is missing` = the plugin is in effect;
envelope format `{"code":401,"msg":"Not logged in or session expired"}` = the application layer; the plugin is not in effect.

### 7.4 Where the caller's identity comes from (single verification)

**The JWT is verified exactly once — at the gateway.** The plugin verifies the signature (RS256, `iss`/`exp`/`kid`),
and then passes the **verified** claims down to the backend in headers
(`X-Jwt-Uid` / `X-Jwt-Username` / `X-Jwt-Sid` / `X-Tenant-ID` / `X-Tenant-Code`; `override: true` means a
client-forged header with the same name is overwritten).

The backend takes the identity from those headers and **does not verify the signature again** — that was the old
"two-layer JWT verification", and verifying twice cost a second signature verification plus a Redis session lookup
on every request, while the trust decision was made in the same place anyway.

What the backend still does:

- checks the **session state** (one Redis lookup by the injected `sid`) — so a kicked-out user is rejected
  immediately, not when the access token expires (2h by default). This is why the plugin must inject `sid`;
- the route middlewares still require an identity, so a request without one is 401.

**The premise is network isolation**: the backend must be reachable only from the gateway — K8s NetworkPolicy, or
the dedicated compose network. On top of that, the backend only trusts the identity headers from peers listed in
`server.trusted_proxies` (the direct peer address, not the `X-Forwarded-For` value, which is client-controlled).
A request that did not come through the gateway therefore carries no identity and is rejected with 401.

Config item (`deploy/config/system-prod.yaml`):

```yaml
gateway:
  identity_source: gateway   # the gateway is the only JWT verifier
```

`identity_source: jwt` is the legacy path: the service verifies the Bearer JWT itself (for environments where the
gateway is not in front). It is not the default — a service that is not explicitly told otherwise must not verify
signatures itself, because the point is that the JWT is verified exactly once.

**Local development goes through the gateway too** (same rule as production):

```bash
# login (whitelist path, no token needed)
curl -H "Host: api.devops.local" -H "Content-Type: application/json" \
  -d '{"tenant_code":"default","username":"admin","password":"<pwd>"}' \
  http://127.0.0.1:8080/system/api/v1/auth/login

# business endpoint (the token is verified by the gateway; the identity arrives as headers)
curl -H "Host: api.devops.local" -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8080/system/api/v1/users
```

A direct call to `http://127.0.0.1:9999/system/api/v1/users` is answered with 401 even when a valid token is
attached — the token was never verified by anyone the backend trusts, so there is no identity to act on.

**Where the whitelist lives**: the anonymous endpoints (login / refresh / captcha / jwks.json) are **not** written in
this README, nor in a separate copy inside `devops-system-public.yaml` — each service declares its own list in
**its own `config/bootstrap.yaml`**, under `gateway.expose`:

```yaml
# devops-system/config/bootstrap.yaml
gateway:
  identity_source: gateway
  expose:
    prefix: /system
    host: api.devops.local
    destination: devops-system.dns:9999
    endpoints:
      - path: /api/v1/auth/login
        reason: 登录：签发 access + refresh 令牌
      - path: /.well-known/jwks.json
        absolute: true          # 主机级端点，不拼服务前缀
        reason: 验证公钥（JWKS），供其他服务自验与人工排障
```

That single declaration feeds two consumers:

- `devops-system-public.yaml` / `devops-user-public.yaml` are **rendered from it** (`just gen-ingress`);
- each service's router test compares it against the actually registered endpoints, so the list cannot drift
  away from the API surface.

The list lives in the service's base config (bootstrap.yaml) rather than in a shared file because that is where
the rest of the service's contract lives, and because bootstrap is **environment independent** — the same file
backs local runs and the container (the deployment config only overrides the environment-specific parts, e.g.
`server.trusted_proxies`). Keeping the whitelist in per-environment files would have meant N copies per service,
which is exactly how the two sides drifted before: the symptom was a request the gateway lets through and the
application layer answers 401 (which is how the OAuth callback on devops-user fell out of sync).

The business config deliberately has **no** exemption entry point: the whitelist is not configurable. An exemption
that exists only in the application layer does not expose any new endpoint (the gateway still requires a JWT
there) — it only means "the gateway already verified the token, skip the second verification in the application
layer". That is a performance / defense-in-depth trade, not an access-control switch, and such a path goes naked
whenever a container is reached directly bypassing the gateway (the compose form has no NetworkPolicy). To add an
exemption, add it to the policy file — it then goes through code review like any other change to which endpoints
are anonymous.

### 7.5 Verification Commands

```bash
# 1) Confirm the plugin is loaded (output should be 1, not 0)
docker compose -f deploy/docker-compose-dev.yml exec higress \
  sh -c 'grep -c "fetching image plugins/jwt-auth" /var/log/higress/gateway.log'

# 2) Anonymous endpoint accessible (expect 200)
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: api.devops.local' \
  http://localhost:8080/system/api/v1/auth/login

# 3) Business endpoint with no token: should be 401 (rejected by the gateway plugin)
curl -s -H 'Host: api.devops.local' \
  http://localhost:8080/system/api/v1/users

# 4) Log in to get a token
TOKEN=$(curl -s -X POST -H 'Host: api.devops.local' -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<pwd>"}' \
  http://localhost:8080/system/api/v1/auth/login | jq -r .data.access_token)

# 5) With token: should be 200 (the gateway verified it and passed the identity down as headers)
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: api.devops.local' \
  -H "Authorization: Bearer $TOKEN" http://localhost:8080/system/api/v1/users

# 6) Direct connection to the backend bypassing the gateway: should be 401
#    (no gateway identity → no identity to act on, even with a valid token attached)
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:9999/system/api/v1/users

# 7) K8s: confirm NetworkPolicy is really enforced by the CNI (see the warning at the top of networkpolicy.yaml)
kubectl -n devops get networkpolicy
```

### 7.6 Common Misconceptions

- **Thinking containment is done with just compose network isolation**: Docker has no port-level ACL, and containers on the same edge
  network can still bypass. Layer 3 must be added.
- **Reusing a static shared secret as the traffic-ingress credential** (the deprecated `X-Gateway-Token`): it expresses no identity,
  has no expiry and no revocation, and the plaintext is scattered across ingresses and configs; a leak means unlimited replay with no audit attribution.
- **Only changing the main ingress and forgetting the canary / gray routes**: all gray traffic gets 403.
- **Empty secret**: `Enforce` passes through directly (by design, to avoid false positives), but that means containment silently fails.
  Always grep to confirm the token is non-empty before going to production.
- **Not testing after NetworkPolicy apply**: kind/k3s default to kindnet, which does not support NetworkPolicy;
  apply reports no error but has no effect at all.

---

## 8. Observability

External stack (applies to both dev compose and production): **Tempo (traces) + Loki (logs) + Prometheus (metrics) +
Grafana (dashboards)**, configured in `deploy/prometheus/`, `deploy/tempo/`, `deploy/loki/` and `deploy/grafana/`.

- **Scraping gateway metrics**: Envoy admin port 15020 outputs merged metrics
  (`enablePrometheusMerge: true` in `higress-config`); Prometheus scrape job:

  ```yaml
  - job_name: higress-gateway
    metrics_path: /stats/prometheus
    static_configs:
      - targets: ["higress:15020"]
  ```

- **Do not enable full metrics**: `proxyStatsMatcher.inclusionRegexps: [.*]` in
  `higress-config` makes Envoy generate stats for every operation, and combined with scraping it causes severe CPU consumption
  (measured to be able to peg the whole Docker engine). Unless troubleshooting requires it, keep it removed.
- **all-in-one built-in observability components**: the image ships prometheus/loki/promtail/grafana, which duplicate
  the external stack and are one of the main causes of high dev-environment CPU. When dev uses the external stack they should be disabled (turn off the
  built-in dashboard in the console: `dashboard.builtin: false`; verify with
  `docker exec <higress-container> supervisorctl status`, and if o11y processes are still listed, overwrite the corresponding `/etc/supervisor/`
  program config with an empty file). After disabling, gateway access logs can only be viewed via
  `docker logs`; if Loki log querying is needed, build it into the external stack.

---

## 9. Docker Production-Grade Deployment

### 9.1 Why all-in-one Cannot Go to Production

all-in-one stuffs pilot/controller, gateway, console and built-in observability components into one container,
with supervisord starting all processes: no independent scaling (the gateway is a stateless hot component, the controller is not),
an OOM in any component restarts the whole container, no per-component resource limits or rolling upgrades, and the built-in o11y conflicts with the production monitoring
system. Production (docker form) should use **component separation + the file config backend** (this repo's yaml is the declarative source of truth; Nacos has been removed from the stack):

```
┌────────────────────────────────────────────────┐
│ docker compose (single host)                    │
│                                                  │
│  higress-gateway ×N  ── stateless, scalable,     │
│        ▲ xDS                minimal config       │
│  higress-controller ×1 ── reads/writes routes/plugins from the config files  │
│        ▲ REST                config (config backend)       │
│  higress-console ×1    ── admin UI, intranet-only     │
│                                                  │
│  redis  mysql  business services           │
└────────────────────────────────────────────────┘
```

### 9.2 Production Compose Template

> Component images and startup parameters should follow the official **higress-standalone**
> (github.com/higress-group/higress-standalone) repo's docker-compose — it
> is Higress's officially maintained non-K8s deployment approach, structurally identical to this section; when writing this doc network access was limited
> so each image tag and env var could not be checked one by one — before adopting, calibrate the ⚠-marked parts against that repo.

```yaml
# deploy/docker-compose-gateway.yml —— production gateway stack (business stack: see docker-compose.yml)
name: devops-gateway

services:
  higress-controller:
    image: higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/controller:2.2.3   # ⚠ tag: compare with higress-standalone
    restart: unless-stopped
    environment:
      # ⚠ the params below follow the official higress-standalone compose; the controller
      # loads the declarative config in deploy/higress (file backend) and pushes xDS to the gateway
      HIGRESS_CONSOLE_DB_URL: ""          # leave empty for the file backend; see the official repo for DB mode
    volumes:
      - gateway-controller-data:/data
    deploy:
      resources:
        limits: { cpus: "1", memory: 1G }   # controller handles config pushes; start at 1C1G
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost:15014/ready"]   # ⚠ port per the official docs
      interval: 10s
      timeout: 5s
      retries: 12
    logging: { driver: json-file, options: { max-size: "50m", max-file: "5" } }

  higress-gateway:
    image: higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/gateway:2.2.3      # ⚠ same as above
    restart: unless-stopped
    depends_on:
      higress-controller:
        condition: service_healthy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - gateway-data:/data
    deploy:
      replicas: 1            # single-host compose cannot span nodes; use K8s for multi-node
      resources:
        limits: { cpus: "2", memory: 2G }
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost:15020/ready"]   # ⚠ port per the official docs
      interval: 10s
      timeout: 5s
      retries: 12
    logging: { driver: json-file, options: { max-size: "50m", max-file: "5" } }

  higress-console:
    image: higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/console:2.2.3       # ⚠ same as above
    restart: unless-stopped
    depends_on:
      - higress-controller
    ports:
      - "127.0.0.1:8001:8001"   # bind to loopback only! Access via SSH tunnel/intranet reverse proxy; never public
    logging: { driver: json-file, options: { max-size: "20m", max-file: "3" } }

volumes:
  gateway-controller-data:
  gateway-data:
```

### 9.3 Production Hardening Checklist

**Config management**
- The yaml in this directory is the declarative source of truth (file config backend); changes are applied by
  re-seeding the controller's config directory and rolling the gateway, and every change is recorded in git.
- Change the rate-limit plugin's `redis.service_name` to the production Redis address.

**Security**
- The console port binds only to `127.0.0.1` or the intranet, accessed via a bastion/reverse proxy; change the
  default admin password immediately after first initialization.
- Configure `jwt-auth` at the gateway layer (see 6.4); mount the TLS certificate to 8443, and have HTTP 80 do a 301 redirect only.
- Passwords for Redis/MySQL etc. are always injected via `.env` (not committed) or secrets management, referenced as `${VAR}` in compose.

**Resources and stability**
- Set `deploy.resources.limits` per component (reference values in the template) to prevent a single component from taking down the host.
- All `restart: unless-stopped` + healthcheck; rotate logs with `json-file` to avoid filling the disk.
- Pin image tags to specific versions, forbidding `latest`; before upgrading, `docker compose pull` +
  rolling restart per component with `up -d <svc>`, gateway first, then controller.

**Backup and recovery**
- Back up the controller config directory (all route/plugin config lives there).
- Every config change in this directory is recorded in git commits, serving as the rollback basis.
- The gateway data volume holds no business state and can be rebuilt; after rebuilding, the controller re-pushes config.

**Monitoring**
- Reuse the existing Prometheus/Grafana stack: scrape `higress-gateway:15020/stats/prometheus`
  (metrics) and controller metrics; import Envoy/Istio dashboards in Grafana to observe 4xx/5xx, P99
  latency and connection counts, plus Tempo traces to locate slow requests.
- Alert items: gateway health check failure, 5xx ratio, rate-limit 429 spikes, controller health.

---

## 10. Kubernetes Deployment (Recommended for Formal Production)

Migrate to K8s when scale, high availability and rolling-upgrade needs are strong; see `deploy/k8s/devops.yaml`:

```bash
helm repo add higress.io https://higress.cn/helm-charts
helm install higress -n higress-system higress.io/higress --create-namespace
kubectl apply -f deploy/higress/mcpbridges/default.yaml
kubectl apply -f deploy/higress/ingresses
kubectl apply -f deploy/higress/wasmplugins
kubectl apply -f deploy/k8s/devops.yaml
# Network containment: make devops-system:9999 accept only traffic from higress-system.
# Must first confirm the CNI supports NetworkPolicy (Calico/Cilium); kindnet does not.
kubectl apply -f deploy/k8s/networkpolicy.yaml
```

In the K8s form, Layer-1 containment is **truly enforced**: once `devops-system` is blocked by NetworkPolicy,
any bypass attempt times out (see "7.2 The Three-Layer Model"). Three places must be changed in sync when migrating:
- The inline public key in `deploy/higress/wasmplugins/jwt-auth-2.0.0.yaml` must correspond to
  devops-system's `jwt.signing_key` (`kid` / `iss` must also match).
  The wasm-go implementation may also switch to `remote_jwks` pointing at `/.well-known/jwks.json` to follow rotation automatically,
  but that requires the service to be reachable via McpBridge, and requests during a failed-fetch window are fail-closed;
- Set `gateway.require_jwt` to true in the Secret in `deploy/k8s/devops.yaml`;
- The `ipBlock.cidr` in `allow-observe-and-probe` must be changed to the actual cluster node CIDRs —
  allowing `0.0.0.0/0` is as good as not allowing it; a wrong CIDR causes kubelet probe failures and repeated Pod restarts.

Address-type configs to adjust when migrating:
- On K8s the McpBridge dns-source FQDNs are the Service DNS themselves — create the Services with
  matching names (`devops-system.devops.svc.cluster.local` etc., see `deploy/k8s/devops.yaml`);
- Change the rate-limit plugin's `redis.service_name` to the actual Redis Service, updating the password and thresholds;
- Replace the Ingress host with the real domain, with TLS certificates via cert-manager or a manual Secret.

---

## Appendix: Common Commands Cheat Sheet

```bash
# dev: start / sync config / stop local services
sh scripts/dev.sh          # containers + auto config seeding + bring up local services
sh scripts/dev.sh sync     # sync to the named volume after editing higress yaml
sh scripts/dev.sh app      # start only the local business services

# Route verification
curl -i -H 'Host: api.devops.local' http://localhost:8080/healthz

# In-container troubleshooting
docker compose -f deploy/docker-compose-dev.yml exec higress supervisorctl status
docker compose -f deploy/docker-compose-dev.yml exec higress cat /data/mcpbridges/default.yaml

# Console
# http://localhost:8001  (dev)
```
