#!/usr/bin/env bash
# warm-module-proxy.sh — trigger proxy.golang.org (and thus pkg.go.dev) to
# resolve+index the current @latest revision of every workspace module.
#
# This does NOT fix "unknown revision"/"invalid version" errors from `go mod
# download` for a *pinned* pseudo-version — that's a resolvability problem
# for that exact version, fixed by scripts/bump-workspace-deps.sh. This
# script is a diagnostic + automation of the pkg.go.dev "Request" button:
# it makes the same @latest proxy call pkg.go.dev makes, for every module,
# in one shot, and prints the resolved version or the error.
#
# Usage: scripts/warm-module-proxy.sh

set -euo pipefail

PROXY_BASE="https://proxy.golang.org"
MODULES=(
  "github.com/DirIO-S3/dirio"
  "github.com/DirIO-S3/dirio/api"
  "github.com/DirIO-S3/dirio/common"
  "github.com/DirIO-S3/dirio/console"
  "github.com/DirIO-S3/dirio/sdk"
)

# Module proxy protocol escapes uppercase letters as "!" + lowercase,
# to avoid case-insensitive filesystem collisions (e.g. DirIO -> !dir!i!o).
escape_path() {
  local path="$1" out="" c
  for ((i = 0; i < ${#path}; i++)); do
    c="${path:$i:1}"
    if [[ "$c" =~ [A-Z] ]]; then
      out+="!${c,}"
    else
      out+="$c"
    fi
  done
  echo "$out"
}

any_failed=false

for mod in "${MODULES[@]}"; do
  escaped=$(escape_path "$mod")
  url="${PROXY_BASE}/${escaped}/@latest"
  echo "==> $mod"
  resp=$(curl -sS -w '\n%{http_code}' "$url")
  body=$(echo "$resp" | head -n -1)
  code=$(echo "$resp" | tail -n1)

  if [[ "$code" == "200" ]]; then
    version=$(echo "$body" | grep '"Version"' | sed -E 's/.*"Version": *"([^"]+)".*/\1/')
    echo "    OK  -> $version"
  else
    echo "    FAIL ($code) -> $body"
    any_failed=true
  fi
done

echo ""
if $any_failed; then
  echo "One or more modules failed to resolve. pkg.go.dev will show the same error." >&2
  exit 1
fi
echo "All modules resolved. pkg.go.dev will pick these up within a few minutes."