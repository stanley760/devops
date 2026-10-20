# Higress Gateway Configuration and Deployment

This directory is the **single source of truth for Higress gateway config**: `mcpbridges/`, `ingresses/`, `wasmplugins/`
are the resource directories recognized by the Higress file config backend, and these yaml files also serve as the
manifest reference for migrating to Kubernetes. This document describes how to configure each capability, plus the three
deployment forms: dev / docker production / K8s.

```
deploy/higress/
├── mcpbridges/default.yaml          # Service source: Nacos registry integration (devops-nacos)
├── ingresses/devops-system.yaml     # Route: api.devops.local -> devops-system
├── ingresses/devops-system-canary.yaml  # Canary route (10% traffic to the gray instance)
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
| docker production | controller + gateway + console split + Nacos config backend | single-host/small-scale production | Section 8 below |
| K8s | Helm deployment (see `deploy/k8s/devops.yaml`) | recommended for formal production | Section 9 below |

Regardless of the form, the business traffic path is the same:

```
Client ──▶ Higress Gateway(:8080/:8443)
              │  jwt-auth / cors / cluster-key-rate-limit / geo-ip
              ▼
           Nacos service discovery ──▶ devops-system / devops-system-gray (registered under DEVOPS_GROUP)
```

**Security boundary**: `devops-system:9999` is not published to host ports or public load balancers; ops endpoints
(`/metrics` `/apidoc` `/healthz` `/readyz`) are not exposed via the public gateway; the Ingress routes only
`/system` and `/.well-known/jwks.json`.

> ⚠️ "Not published to the host" is only the **first** line of defense and is not enough to guarantee unified traffic ingress.
> Any container on the same network (such as the MySQL, Redis, Nacos containers) can reach `devops-system:9999`
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
sh deploy/dev.sh sync       # or .\deploy\dev.ps1 -Command sync
# Force-rerun higress-init seeding, and remove the auto-regenerated
# inclusionRegexps: [.*] in higress-config (see the pitfall table in 2.3), then restart the gateway
```

> init overwrites these three directories every time. **Convention: always edit the yaml in this directory and then sync; the console
> is for viewing only.** Move any route/plugin changes made in the console back to yaml first, otherwise the next `up` will overwrite and lose them.
> Other state the console writes to `/data` (configmaps/secrets, etc.) is unaffected.

### 2.3 Known Pitfalls (Troubleshooting Manual)

| Symptom | Cause | Fix |
| --- | --- | --- |
| Console "service sources" shows `higress-console` instead of `devops-nacos` | The volume holds the default McpBridge generated on the all-in-one's first start; the git config was never seeded | Run `sync`; confirm `/data/mcpbridges/default.yaml` contains `devops-nacos` |
| Route 404 / Ingress not effective | Same as above; `/data/ingresses/` is empty | Same as above |
| Docker engine 500 / abnormally high CPU | ① Windows bind mount (already switched to a named volume); ② the all-in-one's built-in prometheus/loki/promtail/grafana run in parallel with the external stack; ③ `proxyStatsMatcher.inclusionRegexps: [.*]` in `higress-config` causes Envoy full high-cardinality metrics | Use the named volume; disable the built-in observability components (see "8. Observability"); `sync` already includes this fix (see the row below) |
| envoy disappears ~1 minute after startup; 8080 TCP connects but returns empty (8001 is fine) | `inclusionRegexps: [.*]` in `higress-config` (**auto-regenerated on every first start of a new volume**, and the file has CRLF line endings) → envoy's stats tables blow up memory and it gets OOM-killed by cgroup; supervisord treats its exit 0 as expected and does not auto-restart it | `sh deploy/dev.sh sync` (stop, delete the config, then start); the dev compose memory limit has been raised to 2g as a backstop |
| Processes inside all-in-one batch exited | The supervisor shutdown sequence of container stop/restart; logs look like `exited: prometheus (exit status 0; expected)` | First check `docker compose ps` to see whether the container is in a restart loop |

---

## 3. Service Source: McpBridge (Nacos Integration)

`mcpbridges/default.yaml` defines how Higress discovers backend services from the registry:

```yaml
apiVersion: networking.higress.io/v1
kind: McpBridge
metadata:
  name: default
  namespace: higress-system
spec:
  registries:
    - name: devops-nacos        # Service source name, shown on the console's "Service sources" page
      type: nacos2              # nacos2 = Nacos 2.x gRPC protocol, service change perception in seconds
      domain: nacos             # service name within the compose network; change to the actual Service address on K8s
      port: 8848
      nacosNamespaceId: public   # the namespace business services register to
      nacosGroups:
        - DEVOPS_GROUP          # the group business services register to
```

Key points:

- **Underscore escaping rule**: the Nacos group `DEVOPS_GROUP` is written as
  `DEVOPS-GROUP` in Higress target addresses (Higress's escaping rule for underscores); the Ingress's
  `higress.io/destination` must use the escaped form.
- `type: nacos2` requires Nacos to also expose the gRPC port (8848 plus offset 1000, i.e. 9848);
  the nacos service in compose needs `expose` 9848/9849.
- The gray instance `devops-system-gray` registers to the **same** group, distinguished by service name — no extra source needed.
- With multiple registries, list several entries under `registries` (e.g. another DNS-type static source).

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
    # Target address format: <service-name>.<NACOS-group-escaped>.<namespace>.nacos
    higress.io/destination: devops-system.DEVOPS-GROUP.public.nacos
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

- `higress.io/destination` points to the service discovered via McpBridge; service instance up/down events are perceived
  automatically by Nacos, with no route changes needed.
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
    higress.io/canary-weight: "10"        # Weight-based gray release: ~10% of traffic
    higress.io/destination: devops-system-gray.DEVOPS-GROUP.public.nacos
```

Three splitting methods, with priority **Header > Cookie > Weight**:

```yaml
higress.io/canary-by-header: "x-canary"           # Header-based gray release
higress.io/canary-by-header-value: "true"          # Gray release only on exact value match
higress.io/canary-by-cookie: "canary"              # Cookie-based gray release
higress.io/canary-weight: "10"                     # Fallback weight-based gray release
```

Procedure: the `gray` instance registers to the same Nacos group under the service name `devops-system-gray` →
edit the canary yaml weight → `sync` (dev) or apply the manifest (production) → watch Grafana/logs to confirm the
gray traffic ratio → when fully rolled out, publish the new version under the name `devops-system` and delete the canary route.

**Gray instance targeted access** (not weight-random): `ingresses/devops-system-gray-route.yaml` provides
a dedicated `gray.devops.local` route, 100% direct to the gray instance, for gray version verification. Starting the gray instance locally:

```bash
cd devops-system && go run . server -f config/bootstrap-gray.yaml   # :9998, registers as devops-system-gray
curl -i -H 'Host: gray.devops.local' http://localhost:8080/api/v1/auth/info   # 401 means it reached the backend
```

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
bypass is already impossible; on top of that the `edge`/`infra` network isolation keeps MySQL/Redis/Nacos/etcd containers off the business network.
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

### 7.4 Third Layer: Application Guard

The middleware lives in `devops-infra/security/gateway` and requires requests to carry a **independently verifiable**
Bearer JWT (signature + Redis session state), failing uniformly with 401 and not distinguishing "missing credentials" from
"invalid credentials" (anti-probing). It is mounted in `devops-infra/http/server.go`: after the ops endpoints and
before the business routes, otherwise `/healthz` `/readyz` `/metrics` would be blocked.

**Why the application layer must verify again even though the gateway already did**:

- jwt-auth mounts at Ingress-resource granularity and cannot express the fine-grained policy "only these paths need authentication";
  this layer provides the fine-grained containment;
- the compose form has no NetworkPolicy, so bypassing the gateway goes through no plugin at all;
- in the K8s form this layer degrades to defense-in-depth, still valuable.

Config item (`deploy/config/system-prod.yaml`):

```yaml
gateway:
  require_jwt: true
  exempt_paths: []   # /healthz /readyz /metrics, login/refresh/jwks, apidoc are already allowed in code
```

**When `require_jwt: false` the guard passes through directly**, zero changes for unenabled environments. Deliberately disabled in dev: business services run
via `go run` on the host at `127.0.0.1:9999`, and local curl debugging would be significantly slowed by 401s.

**The fail-closed invariant**: when `require_jwt: true` but no JWT verifier is assembled, **deny** rather than allow,
otherwise it amounts to site-wide access with ineffective containment and no alerting.

The anonymous endpoint paths in the exemption list come from the same source as huma registration (the `AuthLoginPath` / `AuthRefreshPath` / `JWKSPath` exported by
`devops-system/internal/router/operations.go`), avoiding a separate copy of the strings in
`devops-system-public.yaml` that could silently drift — the consequence of drift is login being locked out.

### 7.5 Verification Commands

```bash
# 1) Confirm the plugin is loaded (output should be 1, not 0)
docker compose -f deploy/docker-compose-dev.yml exec higress \
  sh -c 'grep -c "fetching image plugins/jwt-auth" /var/log/higress/gateway.log'

# 2) Anonymous endpoint accessible (expect 200)
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: api.devops.local' \
  http://localhost:8080/system/api/v1/auth/login

# 3) Business endpoint with no token: should be 401 (double containment at plugin and application layers)
curl -s -H 'Host: api.devops.local' \
  http://localhost:8080/system/api/v1/users

# 4) Log in to get a token
TOKEN=$(curl -s -X POST -H 'Host: api.devops.local' -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<pwd>"}' \
  http://localhost:8080/system/api/v1/auth/login | jq -r .data.access_token)

# 5) With token: should be 200
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: api.devops.local' \
  -H "Authorization: Bearer $TOKEN" http://localhost:8080/system/api/v1/users

# 6) Direct container connection bypassing the gateway: should be 401 (Layer-3 guard in effect)
docker compose -f deploy/docker-compose.yml exec devops-system \
  wget -qO- --header='Host: x' http://127.0.0.1:9999/system/api/v1/users

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

External stack (applies to both dev compose and production): **Jaeger (traces) + Prometheus (metrics) +
Grafana (dashboards)**, configured in `deploy/prometheus/` and `deploy/grafana/`.

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
system. Production (docker form) should use **component separation + Nacos as the config backend**:

```
┌────────────────────────────────────────────────┐
│ docker compose (single host)                    │
│                                                  │
│  higress-gateway ×N  ── stateless, scalable,     │
│        ▲ xDS                minimal config       │
│  higress-controller ×1 ── connects to Nacos to read/write routes/plugins  │
│        ▲ REST                config (config backend)       │
│  higress-console ×1    ── admin UI, intranet-only     │
│                                                  │
│  nacos(cluster/standalone)  redis  mysql  business services           │
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
      # ⚠ the connection params below follow the official higress-standalone compose
      NACOS_ADDRESS: nacos:8848          # config backend: routes/plugins/McpBridge stored in Nacos
      NACOS_NAMESPACE: public
      NACOS_GROUP: higress-config         # Higress's own config group (not the business group)
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
- Production uses **Nacos as the Higress config backend** (the controller connects to Nacos directly), no longer the file
  backend; the yaml in this directory is the declarative source of truth, imported or CI-synced to Nacos
  via the console/API, and the controller pushes changes to the gateway in seconds.
- Change McpBridge's `domain` to the production Nacos address; change the rate-limit plugin's `redis.service_name`
  to the production Redis address.

**Security**
- The console port binds only to `127.0.0.1` or the intranet, accessed via a bastion/reverse proxy; change the
  default admin password immediately after first initialization.
- Enable Nacos authentication in production (`NACOS_AUTH_ENABLE=true` + secrets); the current dev disabled state
  is local-only.
- Configure `jwt-auth` at the gateway layer (see 6.4); mount the TLS certificate to 8443, and have HTTP 80 do a 301 redirect only.
- Passwords for Redis/MySQL etc. are always injected via `.env` (not committed) or secrets management, referenced as `${VAR}` in compose.

**Resources and stability**
- Set `deploy.resources.limits` per component (reference values in the template) to prevent a single component from taking down the host.
- All `restart: unless-stopped` + healthcheck; rotate logs with `json-file` to avoid filling the disk.
- Pin image tags to specific versions, forbidding `latest`; before upgrading, `docker compose pull` +
  rolling restart per component with `up -d <svc>`, gateway first, then controller.

**Backup and recovery**
- Back up the Nacos data volume daily (the config backend = all route/plugin config lives there).
- Every config change in this directory is recorded in git commits, serving as the rollback basis.
- The gateway data volume holds no business state and can be rebuilt; after rebuilding, just re-pull config from Nacos.

**Monitoring**
- Reuse the existing Prometheus/Grafana stack: scrape `higress-gateway:15020/stats/prometheus`
  (metrics) and controller metrics; import Envoy/Istio dashboards in Grafana to observe 4xx/5xx, P99
  latency and connection counts, plus Jaeger traces to locate slow requests.
- Alert items: gateway health check failure, 5xx ratio, rate-limit 429 spikes, controller-to-Nacos
  disconnection.

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
- Change McpBridge's `domain` in `mcpbridges/default.yaml` to the actual Nacos Service
  (e.g. `nacos.middleware.svc.cluster.local`), confirming 8848/9848 are reachable;
- Change the rate-limit plugin's `redis.service_name` to the actual Redis Service, updating the password and thresholds;
- Replace the Ingress host with the real domain, with TLS certificates via cert-manager or a manual Secret.

---

## Appendix: Common Commands Cheat Sheet

```bash
# dev: start / sync config / stop local services
sh deploy/dev.sh          # containers + auto config seeding + bring up local services
sh deploy/dev.sh sync     # sync to the named volume after editing higress yaml
sh deploy/dev.sh app      # start only the local business services

# Route verification
curl -i -H 'Host: api.devops.local' http://localhost:8080/healthz

# In-container troubleshooting
docker compose -f deploy/docker-compose-dev.yml exec higress supervisorctl status
docker compose -f deploy/docker-compose-dev.yml exec higress cat /data/mcpbridges/default.yaml

# Console
# http://localhost:8001  (dev)
```
