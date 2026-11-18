# Deployment Guide

The `deploy` directory provides container orchestration configs for the local dev environment and production.

## Dev Environment

The dev environment starts **only middleware and the observability stack**; business services (`devops-system` / `devops-id`) are started on the host via `go run`, for easy IDE debugging and hot reload.

### Starting Middleware

```bash
sh scripts/dev.sh
```

Run this from the repository root in a shell with `sh` available. The script starts MySQL, Redis, Nacos, etcd,
the Higress gateway and the observability stack (Tempo/Loki/Prometheus/Grafana and exporters), then automatically
brings up the local business services as background processes. Logs and PIDs are recorded under `deploy/.dev-logs/`
(`tail -f deploy/.dev-logs/devops-system.log` to view output).

On first run, or after schema changes, first create tables and seed data:

```bash
sh scripts/dev.sh migrate
```

> **NACOS_REGISTER_IP**: the IP under which the local process registers to Nacos must be the host's
> LAN IPv4 (not `127.0.0.1`), otherwise the Higress container cannot call back to the business service.
> ```powershell
> $env:NACOS_REGISTER_IP = "192.168.1.20"
> ```

### Common Commands

```bash
sh scripts/dev.sh ps       # show container status
sh scripts/dev.sh logs     # follow logs
sh scripts/dev.sh migrate  # create tables and seed data (skipped if tables exist)
sh scripts/dev.sh restart  # restart middleware
sh scripts/dev.sh app      # start only the local business services (containers untouched)
sh scripts/dev.sh stop     # stop the local business services (containers untouched)
sh scripts/dev.sh sync     # sync deploy/higress config to the named volume and restart the gateway
sh scripts/dev.sh down     # stop and clean up containers
```

### Higress Config and the Named Volume

Higress's `/data` uses the named volume `higress-data` (avoiding the high CPU caused by Windows bind mounts crossing filesystems). `deploy/higress/` remains the single source of truth for config: the compose `higress-init` one-shot service
automatically seeds its `mcpbridges/`, `ingresses/`, `wasmplugins/` into the volume before the gateway starts;
the first `up` / after `down` it is seeded automatically; after editing yaml, run `sync` to take effect immediately (re-seed + restart gateway).
init overwrites these three directories every time — move any route/plugin changes made in the console back to yaml.

### Access URLs

| Service | URL |
| --- | --- |
| Frontend dev server | `http://127.0.0.1:5173/` (admin) · `http://127.0.0.1:5173/portal.html` (C-end portal) |
| devops-system | `http://localhost:9999/healthz` (local process) |
| devops-user | `http://localhost:10000/healthz` (local process) |
| Higress gateway | `http://localhost:8080/healthz` (Host: `api.devops.local`) |
| Higress console | `http://localhost:8001` |
| Nacos console | `http://localhost:8848/nacos` |
| Tempo UI (Grafana Traces Drilldown) | `http://localhost:3000/a/grafana-exploretraces-app` |
| Tempo API | `http://localhost:3200` |
| Loki API | `http://localhost:3100` |
| Prometheus | `http://localhost:9090` |
| Alertmanager | `http://localhost:9093` (dev alerts are UI-only; production wires a webhook) |
| Grafana | `http://localhost:3000` (admin/admin) |
| node-exporter (host metrics) | `node-exporter:9100` (Compose network only) |
| mysqld-exporter (MySQL metrics) | `mysqld-exporter:9104` (Compose network only) |
| redis_exporter (Redis metrics) | `redis-exporter:9121` (Compose network only) |

### Prometheus scrape target (`HOST_IP`)

The three business services run on the host via `go run`, so Prometheus reaches
them through the host's LAN address. `host.docker.internal` resolves to an
address blocked by the Windows Firewall on the Public network profile.

Compose pins the hostname `devops-host` to the LAN address using
`extra_hosts`; Prometheus scrapes that literal hostname because it does not
expand environment variables in `scrape_configs`. Copy
[`deploy/.env.example`](./.env.example) to `deploy/.env` and set `HOST_IP` to
the host's reachable LAN IPv4. Update it when the machine changes networks;
it should match the address used for `NACOS_REGISTER_IP`.

### Signal Exploration (Drilldown)

Grafana 12.x ships three queryless "Drilldown" apps, all sharing the global time
range and label filters — no PromQL/LogQL required:

| App | URL | Backend | Data |
| --- | --- | --- | --- |
| Metrics | `/a/grafana-metricsdrilldown-app` | Prometheus | all scraped metrics, grouped by metric-name prefix |
| Logs | `/a/grafana-lokiexplore-app` | Loki | the services' slog output, grouped by `service.name` (index label `service_name`), with `trace_id`/`span_id` as structured metadata |
| Traces | `/a/grafana-exploretraces-app` | Tempo | Rate / Errors / Duration charts fed by Tempo's span metrics (`traces_spanmetrics_*`, remote-written into Prometheus) |

### Industrial ops dashboards

Three dashboards are provisioned from `grafana/provisioning/dashboards/`:

| Dashboard | uid | Covers |
| --- | --- | --- |
| System resources + application performance | `devops-system-app` | Host CPU/memory/disk/network, per-service runtime, HTTP/gRPC RED, gateway, dependency readiness |
| Database + cache/middleware | `devops-data-middleware` | MySQL server metrics, SQL pool, application cache hit rate, Redis server metrics, Redis pool |
| Logs + security | `devops-logs-security` | Log volume/level/status, slow requests, error stream, login failures, 4xx/5xx, rate limiting |

The application cache hit rate (`devops_cache_operations_total`) measures
lookups served from the cache. Redis connection-pool reuse
(`db_client_connections_hits` / `misses`) measures a different behavior and is
shown separately. Host resource metrics come from node-exporter inside the
WSL2/Docker VM, not the Windows host.

Dashboard JSON is rendered from `devops-infra/cmd/dashboards`. After changing a
panel, re-render the files:

```bash
go run ./devops-infra/cmd/dashboards -out deploy/grafana/provisioning/dashboards
```

Data flow:

- **Traces** — services export OTLP traces to Tempo (`:4318`, same port Jaeger
  used, so `observability.otlp_endpoint` is unchanged).
- **Logs** — services export OTLP logs to Loki's OTLP endpoint
  (`http://loki:3100/otlp` → `/otlp/v1/logs`; note this is *not* the standard
  `/v1/logs` path). Configured via `observability.otlp_log_endpoint`.
- **Span RED metrics** — Tempo's metrics-generator remote-writes into Prometheus
  (requires `--web.enable-remote-write-receiver`), which is what feeds the Traces
  Drilldown Rate/Errors/Duration charts.
- **Backend metrics** — Prometheus separately scrapes Tempo and Loki's `/metrics`
  endpoints on the Compose network; these are their own service metrics, not span RED data.

Cross-navigation is wired in `grafana/provisioning/datasources/observability.yml`:
Prometheus exemplars → Tempo trace, Tempo → its logs (by `trace_id`), Tempo ↔ Loki.

> The frontend no longer connects to the business services directly — every API request goes
> through the Higress gateway (see [higress/UNIFIED-ENTRY.md](./higress/UNIFIED-ENTRY.md)).
> Add the following to `C:\Windows\System32\drivers\etc\hosts`:
> ```
> 127.0.0.1 api.devops.local
> 127.0.0.1 user-api.devops.local
> 127.0.0.1 devops.local
> ```
> The gateway can only reach the locally running business services via the **LAN IPv4** of this
> machine, not `127.0.0.1`; set `NACOS_REGISTER_IP` to that address when starting the services
> locally, otherwise the gateway resolves to the loopback and every request returns 503.

Alerts: rules live in [prometheus/alerts.yml](./prometheus/alerts.yml) (target down / HTTP 5xx /
P99 / gRPC error rate / SQL·Redis pool waits, plus host resources, MySQL, cache and security groups);
fired alerts are routed via Alertmanager. The dev receiver is empty (UI-only); in production,
replace dev-sink in `alertmanager/alertmanager.yml` with a DingTalk/email webhook.

Dev environment data is stored in Docker volumes with a `dev-` prefix, isolated from the production orchestration's data volumes.
Higress-specific routes, rate limiting, CORS and Kubernetes notes are in
[higress/README.md](./higress/README.md).

### Config File Mapping

| Startup mode | Loaded config | Notes |
| --- | --- | --- |
| Local `go run` | `devops-system/config/devops-system-dev.yaml` | address `127.0.0.1`, connects to host ports |
| Local `go run` | `devops-id/config/devops-id-dev.yaml` | etcd `127.0.0.1:2379` |
| `migrate` container | `deploy/config/system-dev.yaml` | addresses use container service names (mysql/redis/nacos) |

## Production / General Container Orchestration

The production/general orchestration file is [docker-compose.yml](./docker-compose.yml). Higress starts in it
via the `gateway` profile:

```bash
docker compose -f deploy/docker-compose.yml --profile gateway up -d
```
