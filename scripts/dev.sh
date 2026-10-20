#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/docker-compose-dev.yml"
COMMAND="${1:-up}"
LOG_DIR="$ROOT/deploy/.dev-logs"

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

# Start a business service in the background via `go run`, with logs and PID recorded under deploy/.dev-logs/.
start_service() {
  name="$1"; dir="$2"
  mkdir -p "$LOG_DIR"
  pidfile="$LOG_DIR/$name.pid"
  if [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    echo "  $name is already running (PID $(cat "$pidfile")), skipping"
    return
  fi
  (
    cd "$ROOT/$dir" || exit 1
    exec go run . server
  ) >"$LOG_DIR/$name.log" 2>&1 &
  echo $! >"$pidfile"
  echo "  $name started in the background (PID $!): tail -f deploy/.dev-logs/$name.log"
}

stop_service() {
  name="$1"
  pidfile="$LOG_DIR/$name.pid"
  if [ -f "$pidfile" ]; then
    pid="$(cat "$pidfile")"
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      echo "  Stopped $name (PID $pid)"
    else
      echo "  $name is not running"
    fi
    rm -f "$pidfile"
  else
    echo "  $name is not running"
  fi
}

start_services() {
  echo ""
  echo "Starting local business services..."
  start_service devops-id devops-id
  # devops-system depends on devops-id for ID generation; wait for it to be ready first
  sleep 3
  start_service devops-system devops-system
  start_service devops-user devops-user
  echo ""
  echo "On first run, or after schema changes, first create tables and seed data: sh deploy/dev.sh migrate"
  echo "Before first use, copy .env.example to .env and fill in the LAN IP (NACOS_REGISTER_IP)."
}

# —— watch: monitor local business services (liveness+readiness). Liveness failures auto-restart (backoff+circuit breaker); readiness failures only alert ——
# Endpoints are hardcoded from the dev config: devops-id's liveness probe uses metrics :9991 (if metrics are disabled it would be
# falsely judged dead, with the circuit breaker as the backstop); devops-system's /readyz contains three dependency checks:
# mysql/redis/devops-id; devops-user's liveness uses /healthz, and its /readyz contains three dependency checks: mysql/redis/devops-id.

WATCH_INTERVAL=10  # poll interval (seconds)
WATCH_BACKOFF=60   # per-service restart backoff (seconds)
WATCH_BREAKER=5    # consecutive-restart circuit-breaker threshold (count)
ID_ALIVE_URL="http://127.0.0.1:9991/metrics"
SYS_ALIVE_URL="http://127.0.0.1:9999/healthz"
SYS_READY_URL="http://127.0.0.1:9999/readyz"
USR_ALIVE_URL="http://127.0.0.1:10000/healthz"
USR_READY_URL="http://127.0.0.1:10000/readyz"

# HTTP probe: outputs the status code (000 or empty on connection failure); when $2 is provided, writes the response body to that file.
# curl failures have their exit codes swallowed via || true to avoid set -e killing the monitor loop.
probe_code() {
  if [ "$#" -ge 2 ]; then
    curl -s -m 3 -o "$2" -w '%{http_code}' "$1" 2>/dev/null || true
  else
    curl -s -m 3 -o /dev/null -w '%{http_code}' "$1" 2>/dev/null || true
  fi
}

# Probe "liveness + readiness" style services: $1=liveness URL $2=readiness URL $3=readiness response output file.
# Results are written to probe_state (UP/DOWN/UNREADY) and probe_detail (not-ready details), taken by the caller immediately.
probe_http() {
  probe_state="DOWN"; probe_detail=""
  if [ "$(probe_code "$1")" = "200" ]; then
    probe_state="UP"
    if [ "$(probe_code "$2" "$3")" != "200" ]; then
      probe_state="UNREADY"
      probe_detail=$(grep -o '"unready":\[[^]]*\]' "$3" 2>/dev/null || true)
      if [ -z "$probe_detail" ]; then probe_detail='"unready":[?]'; fi
    fi
  fi
}

# Before restarting a service that depends on devops-id, wait for it to recover first (up to 30s): when devops-id is not ready,
# restarting a consumer would immediately be unhealthy too, needlessly burning backoff and circuit-breaker counts.
wait_for_id() {
  if [ "$(probe_code "$ID_ALIVE_URL")" = "200" ]; then
    return
  fi
  echo "  Waiting for devops-id to recover before restarting $1 (up to 30s)..."
  waited=0
  while [ "$waited" -lt 30 ]; do
    sleep 3
    waited=$((waited + 3))
    if [ "$(probe_code "$ID_ALIVE_URL")" = "200" ]; then break; fi
  done
}

# Read a single service's last restart timestamp (persisted in .dev-logs so the backoff still applies after a watch restart); invalid content is treated as 0
read_last_restart() {
  value=0
  if [ -f "$LOG_DIR/$1.last-restart" ]; then
    value=$(cat "$LOG_DIR/$1.last-restart" 2>/dev/null || echo 0)
  fi
  case "$value" in
    ''|*[!0-9]*) value=0 ;;
  esac
  echo "$value"
}

# Initiate a restart (the caller has already passed the backoff check): update the timestamp and reuse start_service
watch_restart() {
  echo "[$(date '+%H:%M:%S')] Auto-restarting $1 ..."
  date +%s >"$LOG_DIR/$1.last-restart"
  start_service "$1" "$2"
}

print_state() {
  # $1=service name $2=state $3=readiness detail
  case "$2" in
    UP)      echo "[$(date '+%H:%M:%S')] $1 UP" ;;
    DOWN)    echo "[$(date '+%H:%M:%S')] $1 DOWN" ;;
    UNREADY) echo "[$(date '+%H:%M:%S')] $1 UNREADY (dependency anomaly, alert only, no restart): $3" ;;
  esac
}

watch_services() {
  mkdir -p "$LOG_DIR"
  if ! command -v go >/dev/null 2>&1; then
    echo "Warning: go is not found in the current shell; detected failures cannot be auto-restarted (under Git Bash, export PATH first before running)"
  fi
  echo "Monitoring devops-id ($ID_ALIVE_URL), devops-system ($SYS_ALIVE_URL + $SYS_READY_URL)"
  echo "and devops-user ($USR_ALIVE_URL + $USR_READY_URL)"
  echo "Polling every ${WATCH_INTERVAL}s; liveness failures auto-restart (backoff ${WATCH_BACKOFF}s, circuit breaker after ${WATCH_BREAKER} consecutive times); readiness failures only alert. Ctrl-C to exit."
  echo ""
  trap 'echo ""; echo "Monitoring exited (services are unaffected)"; exit 0' INT TERM

  id_state="INIT"; id_breaker=0; id_tripped=0; id_last=$(read_last_restart devops-id)
  sys_state="INIT"; sys_breaker=0; sys_tripped=0; sys_last=$(read_last_restart devops-system)
  usr_state="INIT"; usr_breaker=0; usr_tripped=0; usr_last=$(read_last_restart devops-user)
  cycle=0

  while :; do
    cycle=$((cycle + 1))

    # Probe: devops-id liveness only; devops-system / devops-user liveness + readiness
    if [ "$(probe_code "$ID_ALIVE_URL")" = "200" ]; then id_new="UP"; else id_new="DOWN"; fi
    probe_http "$SYS_ALIVE_URL" "$SYS_READY_URL" "$LOG_DIR/watch-readyz-sys.body"
    sys_new="$probe_state"; sys_detail="$probe_detail"
    probe_http "$USR_ALIVE_URL" "$USR_READY_URL" "$LOG_DIR/watch-readyz-usr.body"
    usr_new="$probe_state"; usr_detail="$probe_detail"

    # Print state changes immediately; recovery to UP resets the breaker count and releases the circuit breaker
    if [ "$id_new" != "$id_state" ]; then
      print_state devops-id "$id_new" ""
      id_state="$id_new"
    fi
    if [ "$id_new" = "UP" ]; then id_breaker=0; id_tripped=0; fi
    if [ "$sys_new" != "$sys_state" ]; then
      print_state devops-system "$sys_new" "$sys_detail"
      sys_state="$sys_new"
    fi
    if [ "$sys_new" = "UP" ]; then sys_breaker=0; sys_tripped=0; fi
    if [ "$usr_new" != "$usr_state" ]; then
      print_state devops-user "$usr_new" "$usr_detail"
      usr_state="$usr_new"
    fi
    if [ "$usr_new" = "UP" ]; then usr_breaker=0; usr_tripped=0; fi

    # Heartbeat: one line every 6 cycles (~60s)
    if [ $((cycle % 6)) -eq 0 ]; then
      echo "[$(date '+%H:%M:%S')] heartbeat: devops-id=$id_state devops-system=$sys_state devops-user=$usr_state"
    fi

    # Auto-restart (triggered by liveness failure only; readiness failure is a container/dependency problem, restarting the Go service is pointless)
    if [ "$id_new" = "DOWN" ] && [ "$id_tripped" -ne 1 ]; then
      now=$(date +%s)
      if [ $((now - id_last)) -ge "$WATCH_BACKOFF" ]; then
        watch_restart devops-id devops-id
        id_last=$now
        id_breaker=$((id_breaker + 1))
        if [ "$id_breaker" -ge "$WATCH_BREAKER" ]; then
          id_tripped=1
          echo "[$(date '+%H:%M:%S')] devops-id still unhealthy after ${WATCH_BREAKER} consecutive restarts; auto-restart stopped, please investigate manually"
        fi
      fi
    fi
    if [ "$sys_new" = "DOWN" ] && [ "$sys_tripped" -ne 1 ]; then
      now=$(date +%s)
      if [ $((now - sys_last)) -ge "$WATCH_BACKOFF" ]; then
        # Both services dead at once: wait for devops-id to recover before restarting devops-system (consumer readiness depends on ID generation)
        wait_for_id devops-system
        watch_restart devops-system devops-system
        sys_last=$(date +%s)
        sys_breaker=$((sys_breaker + 1))
        if [ "$sys_breaker" -ge "$WATCH_BREAKER" ]; then
          sys_tripped=1
          echo "[$(date '+%H:%M:%S')] devops-system still unhealthy after ${WATCH_BREAKER} consecutive restarts; auto-restart stopped, please investigate manually"
        fi
      fi
    fi
    if [ "$usr_new" = "DOWN" ] && [ "$usr_tripped" -ne 1 ]; then
      now=$(date +%s)
      if [ $((now - usr_last)) -ge "$WATCH_BACKOFF" ]; then
        # Likewise: when devops-id is dead, do not rush to restart devops-user; wait for the ID-generation service to come back first
        wait_for_id devops-user
        watch_restart devops-user devops-user
        usr_last=$(date +%s)
        usr_breaker=$((usr_breaker + 1))
        if [ "$usr_breaker" -ge "$WATCH_BREAKER" ]; then
          usr_tripped=1
          echo "[$(date '+%H:%M:%S')] devops-user still unhealthy after ${WATCH_BREAKER} consecutive restarts; auto-restart stopped, please investigate manually"
        fi
      fi
    fi

    sleep "$WATCH_INTERVAL"
  done
}

case "$COMMAND" in
  up)
    # Start middleware + gateway + observability stack, then automatically bring up local business services
    # higress-init automatically seeds deploy/higress config before higress (on first run / after down)
    compose up -d mysql redis nacos etcd higress
    compose up -d jaeger prometheus alertmanager grafana
    echo ""
    echo "Middleware and observability stack started."
    start_services
    ;;
  app)
    # Start/supplement local business services only; containers untouched
    start_services
    ;;
  watch)
    # Monitor local business services: liveness failures auto-restart, readiness failures only alert (containers and gateway are not monitored)
    watch_services
    ;;
  stop)
    # Stop local business services (containers unaffected)
    stop_service devops-user
    stop_service devops-system
    stop_service devops-id
    ;;
  migrate)
    # The dev compose stack only runs infrastructure; business services run locally, so migration also executes with local config.
    (cd "$ROOT/devops-system" && go run . migrate)
    ;;
  down) compose down ;;
  restart) compose restart mysql redis nacos etcd higress ;;
  sync)
    # Explicit sync after editing yaml under deploy/higress; also cures the envoy OOM toxic config:
    #   1. Stop the gateway (with the apiserver down, editing config files will not be overwritten by the in-memory copy)
    #   2. Re-run higress-init (only --force-recreate re-runs a completed one-shot container) to seed git config
    #   3. Remove the auto-generated inclusionRegexps: [.*] in higress-config
    #      (full metrics → envoy stats tables blow up memory and get OOM-killed by cgroup; supervisord treats exit 0 as
    #      expected and does not auto-restart; the file has CRLF line endings, so it must be normalized with tr before sed)
    #   4. Start the gateway. Recommended to run this command once after the first up of a new volume
    compose stop higress
    compose up -d --force-recreate higress-init
    compose run --rm --entrypoint sh higress-init -c 'if [ -f /data/configmaps/higress-config.yaml ]; then tr -d "\r" < /data/configmaps/higress-config.yaml > /tmp/hc && mv /tmp/hc /data/configmaps/higress-config.yaml && sed -i "/^      proxyStatsMatcher:$/,/^        - \.\*$/d" /data/configmaps/higress-config.yaml; fi'
    compose start higress
    ;;
  logs) compose logs -f ;;
  ps) compose ps ;;
  *)
    echo "Usage: $0 [up|app|stop|down|restart|sync|logs|ps|migrate|watch]" >&2
    exit 2
    ;;
esac
