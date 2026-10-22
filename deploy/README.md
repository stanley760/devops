# Deployment Guide

The `deploy` directory provides container orchestration configs for the local dev environment and production.

## Dev Environment

The dev environment starts **only middleware and the observability stack**; business services (`devops-system` / `devops-id`) are started on the host via `go run`, for easy IDE debugging and hot reload.

### Starting Middleware

```powershell
.\deploy\dev.ps1
```

```bash
sh deploy/dev.sh
```

The script starts MySQL, Redis, Nacos, etcd, the Higress gateway and the observability stack (Jaeger/Prometheus/Grafana),
then automatically brings up the local business services:

- **PowerShell**: runs `go run` for `devops-id` and `devops-system` in two separate cmd windows; closing a window stops that service.
- **sh**: starts them as background processes, with logs and PIDs recorded under `deploy/.dev-logs/`
  (`tail -f deploy/.dev-logs/devops-system.log` to view output).

On first run, or after schema changes, first create tables and seed data:

```powershell
.\deploy\dev.ps1 -Command migrate
```

```bash
sh deploy/dev.sh migrate
```

> **NACOS_REGISTER_IP**: the IP under which the local process registers to Nacos must be the host's
> LAN IPv4 (not `127.0.0.1`), otherwise the Higress container cannot call back to the business service.
> ```powershell
> $env:NACOS_REGISTER_IP = "192.168.1.20"
> ```

### Common Commands

```powershell
.\deploy\dev.ps1 -Command ps       # show container status
.\deploy\dev.ps1 -Command logs     # follow logs
.\deploy\dev.ps1 -Command migrate  # create tables and seed data (skipped if tables exist)
.\deploy\dev.ps1 -Command restart  # restart middleware
.\deploy\dev.ps1 -Command app      # start only the local business services (containers untouched)
.\deploy\dev.ps1 -Command sync     # sync deploy/higress config to the named volume and restart the gateway
.\deploy\dev.ps1 -Command down     # stop and clean up containers
```

```bash
sh deploy/dev.sh ps       # show container status
sh deploy/dev.sh logs     # follow logs
sh deploy/dev.sh migrate  # create tables and seed data (skipped if tables exist)
sh deploy/dev.sh restart  # restart middleware
sh deploy/dev.sh app      # start only the local business services (containers untouched)
sh deploy/dev.sh stop    # stop the local business services (containers untouched)
sh deploy/dev.sh sync    # sync deploy/higress config to the named volume and restart the gateway
sh deploy/dev.sh down     # stop and clean up containers
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
| Jaeger UI | `http://localhost:16686` |
| Prometheus | `http://localhost:9090` |
| Alertmanager | `http://localhost:9093` (dev alerts are UI-only; production wires a webhook) |
| Grafana | `http://localhost:3000` (admin/admin) |

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
P99 / gRPC error rate / SQL·Redis pool waits, etc.); fired alerts are routed via Alertmanager. The dev receiver
is empty (UI-only); in production, replace dev-sink in `alertmanager/alertmanager.yml` with a DingTalk/email webhook.

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
