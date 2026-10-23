# devops task runner (Makefile replacement)
#
# Syntax notes (key differences between just and make):
#   - Indentation uses [spaces]; no tabs between commands
#   - Variable interpolation uses {{var}}; shell commands use backticks
#   - Each recipe gets its own shell by default; cross-line state must be declared explicitly (see set shell below)
#   - Use `just --list` to see all commands

# Version number: derived from the git tag, falling back to dev when there is no tag.
#
# Note: git describe in a multi-tag repo returns values like "devops-common/v0.1.1-dirty"
# that contain slashes, and docker tags do not allow slashes (it reports "invalid reference format").
# So slashes and illegal characters are uniformly replaced with hyphens.
version := `git describe --tags --always --dirty 2>/dev/null | tr '/' '-' | tr -cd '[:alnum:]._-' || echo dev`

# Local Go toolchain (module cache at /d/Go/pkg); override with `just GO=/path/to/go test`
go := env_var_or_default("GO", "/d/GoToolchain/go/bin/go.exe")

compose := "docker compose -f deploy/docker-compose.yml"
dev_compose := "docker compose -f deploy/docker-compose-dev.yml"

# go.work is a multi-module workspace; `go build ./...` in the root directory reports
# "directory prefix . does not contain modules listed in go.work",
# so modules must be specified one by one.
modules := "devops-common devops-id devops-infra devops-system devops-user"

# List all available commands
default:
    @just --list --unsorted

# ---------- Container Build ----------

# Build the devops-id image
build-id:
    docker build -f devops-id/Dockerfile --build-arg VERSION={{version}} -t devops-id:{{version}} .

# Build the devops-system image
build-system:
    docker build -f devops-system/Dockerfile --build-arg VERSION={{version}} -t devops-system:{{version}} .

# Build all images
build: build-id build-system

# ---------- Production orchestration (deploy/docker-compose.yml) ----------

# Create business tables and seed data (idempotent; skipped if tables exist)
migrate:
    {{compose}} run --rm devops-system migrate

# Start the production orchestration
up:
    {{compose}} up -d

# Stop the production orchestration
down:
    {{compose}} down

# Follow production orchestration logs
logs:
    {{compose}} logs -f

# Show production orchestration status
ps:
    {{compose}} ps

# ---------- Dev orchestration (deploy/docker-compose-dev.yml) ----------
# Starts only middleware + gateway + observability stack; business services are started via local go run.

# Start dev environment dependencies
dev-up:
    {{dev_compose}} up -d mysql redis nacos etcd higress
    {{dev_compose}} up -d jaeger prometheus alertmanager grafana

# Stop dev environment dependencies
dev-down:
    {{dev_compose}} down

# Follow dev environment logs
dev-logs:
    {{dev_compose}} logs -f

# Show dev environment status
dev-ps:
    {{dev_compose}} ps

# Run table creation and seed initialization in a one-shot container
dev-migrate:
    {{dev_compose}} build devops-system
    {{dev_compose}} --profile app run --rm devops-system migrate

# ---------- Local checks (no containers) ----------

# Build the whole workspace (module by module)
build-local:
    #!/usr/bin/env bash
    set -euo pipefail
    for m in {{modules}}; do
        echo "==> building $m"
        {{go}} build "./$m/..."
    done

# Vet the whole workspace (module by module)
vet:
    #!/usr/bin/env bash
    set -euo pipefail
    for m in {{modules}}; do
        echo "==> vetting $m"
        {{go}} vet "./$m/..."
    done

# Run the whole workspace's tests (module by module)
test:
    #!/usr/bin/env bash
    set -euo pipefail
    for m in {{modules}}; do
        echo "==> testing $m"
        {{go}} test "./$m/..."
    done

# Test devops-system only
test-system:
    {{go}} test ./devops-system/...

# Test only the new domain and application layers
test-domain:
    {{go}} test ./devops-system/internal/domain/... ./devops-system/internal/application/... ./devops-system/internal/arch/...

# Format (gofmt)
#
# It cannot be written as `{{go}} fmt ./devops-system/... ...`: go fmt does not enable workspace mode
# (cmd/go's fmtcmd does not call modload InitWorkfile; see golang/go source
# internal/fmtcmd/fmt.go), so in the workspace root all module prefixes are judged as
# "does not contain main module or its selected dependencies".
# Hence enter each module's directory and run it there; go fmt itself works fine within that module.
fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    for m in {{modules}}; do
        echo "==> formatting $m"
        (cd "$m" && {{go}} fmt ./...)
    done

# ---------- Gateway exposure endpoint policy ----------
#
# The anonymous-endpoint allowlist (exempt from jwt-auth) is maintained in only one place:
# the gateway.expose section of each service's own config/bootstrap.yaml.
# The Higress public ingress is rendered from it, and each service's router test compares
# it with the actually registered endpoints — so the gateway side and the application
# layer can no longer drift apart (the classic symptom: the gateway lets it through
# while the application layer answers 401).
#
# To change the allowlist: edit the service's bootstrap.yaml, then run gen-ingress.

# Render the Higress public ingress from each service's bootstrap.yaml (gateway.expose section)
gen-ingress:
    {{go}} run ./devops-infra/cmd/gwpolicy -root . -out deploy/higress/ingresses

# Verify the exposure endpoint allowlist matches the actually registered endpoints (consistency test in each service's router package)
check-policy:
    {{go}} test ./devops-system/internal/router/ ./devops-user/internal/router/

# Build + vet + test: run this before committing
check: build-local vet test

# Clear the Go build cache
clean:
    {{go}} clean -cache -testcache
