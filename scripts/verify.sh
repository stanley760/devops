#!/usr/bin/env bash
# Local verification script: build + static checks + tests.
# Usage:
#   scripts/verify.sh build   build the whole workspace only
#   scripts/verify.sh test    build + test the whole workspace
#   scripts/verify.sh test-system   build + test devops-system
#   scripts/verify.sh test ./devops-system/internal/...   test a specific package
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO="${GO_BIN:-/d/GoToolchain/go/bin/go.exe}"
cd "$ROOT"

if [ ! -x "$GO" ]; then
  echo "Go toolchain not found: $GO (override with the GO_BIN environment variable)" >&2
  exit 127
fi

TARGET="${2:-}"
MODULES="devops-common devops-id devops-infra devops-system devops-user"

has_go_files() {
  find "$1" -type f -name '*.go' -print -quit | grep -q .
}

build_all() {
  for m in $MODULES; do
    if ! has_go_files "$m"; then
      echo "  - $m (no Go source files, skipping)"
      continue
    fi
    echo "  - $m"
    "$GO" build ./"$m"/...
  done
}

test_all() {
  for m in $MODULES; do
    if ! has_go_files "$m"; then
      echo "  - $m (no Go source files, skipping)"
      continue
    fi
    echo "  - $m"
    "$GO" test ./"$m"/...
  done
}

vet_all() {
  for m in $MODULES; do
    if ! has_go_files "$m"; then
      echo "  - $m (no Go source files, skipping)"
      continue
    fi
    echo "  - $m"
    "$GO" vet ./"$m"/...
  done
}

echo "==> go build (all modules)"
build_all

case "${1:-test}" in
  build)
    echo "==> build only, passed"
    ;;
  vet)
    if [ -n "$TARGET" ]; then
      echo "==> go vet $TARGET"
      "$GO" vet "$TARGET"
    else
      echo "==> go vet (all modules)"
      vet_all
    fi
    echo "==> vet passed"
    ;;
  test)
    if [ -n "$TARGET" ]; then
      echo "==> go test $TARGET"
      "$GO" test "$TARGET"
    else
      echo "==> go test (all modules)"
      test_all
    fi
    ;;
  test-system)
    echo "==> go test ./devops-system/..."
    "$GO" test ./devops-system/...
    ;;
  *)
    echo "Unknown subcommand: $1 (supported: build|vet|test|test-system)" >&2
    exit 2
    ;;
esac
