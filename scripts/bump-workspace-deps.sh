#!/usr/bin/env bash
# bump-workspace-deps.sh — re-pin cross-module requires to a given commit.
#
# Workspace modules may require sibling workspace modules by pseudo-version
# (e.g. console requires github.com/DirIO-S3/dirio/api). That pseudo-version
# is only valid if, AT THAT COMMIT, the target module's own go.mod already
# declares the current module path — otherwise `go mod download` fails with
# "unknown revision" or "invalid version" (this bit us right after renaming
# github.com/mallardduck/dirio -> github.com/DirIO-S3/dirio, since old
# pseudo-versions pointed at pre-rename commits).
#
# This script re-pins every such cross-module require to a given commit
# (default: HEAD) so CI, which builds without go.work, can resolve them.
#
# Usage: scripts/bump-workspace-deps.sh [commit-ish]
#   commit-ish   Commit to pin to (default: HEAD). Must already be pushed to
#                origin — go get resolves it over the network, not locally.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

REF="${1:-HEAD}"
SHA=$(git rev-parse "$REF")

if ! git branch -r --contains "$SHA" 2>/dev/null | grep -q .; then
  echo "WARN: $SHA doesn't appear to be on any remote branch yet." >&2
  echo "      Push it first — go get needs to fetch it over the network." >&2
fi

MODULE_PREFIX="github.com/DirIO-S3/dirio"
MODULES=(api common console sdk)

# "tidy" = safe to run go get + go mod tidy
# "edit" = workspace-only imports make full resolution unsafe; edit go.mod directly
declare -A STRATEGY=(
  [api]="tidy"
  [common]="tidy"
  [sdk]="tidy"
  [console]="edit"
)

export GOWORK=off

any_changed=false

for mod in "${MODULES[@]}"; do
  modfile="$mod/go.mod"
  [[ -f "$modfile" ]] || continue

  # Sibling workspace modules this module requires (excludes itself)
  deps=$(grep -E "^\s+${MODULE_PREFIX}/[a-z]+ v" "$modfile" \
    | awk '{print $1}' \
    | grep -v "^${MODULE_PREFIX}/${mod}\$" || true)

  for dep in $deps; do
    # Skip deps already satisfied by a local-path replace (e.g. console -> ../api) —
    # those never go stale and re-pinning a version here would regress that fix.
    if grep -qE "${dep//\//\\/}\s*=>\s*\.\./" "$modfile"; then
      echo "Skipping $mod: $dep (has local replace)"
      continue
    fi

    echo "Bumping $mod: $dep -> @$SHA"
    any_changed=true
    strategy="${STRATEGY[$mod]}"

    if [[ "$strategy" == "tidy" ]]; then
      go get -C "$mod" "${dep}@${SHA}"
      go mod tidy -C "$mod"
    else
      # Resolve the pseudo-version from a neutral module (common has no
      # cross-deps of its own) so go mod edit can pin it exactly.
      if ! list_out=$(go list -C common -m -json "${dep}@${SHA}" 2>&1); then
        echo "ERROR: failed to resolve ${dep}@${SHA}:" >&2
        echo "$list_out" >&2
        exit 1
      fi
      pseudo=$(echo "$list_out" | grep '"Version"' | head -1 | sed -E 's/.*"Version": "(.*)".*/\1/')
      if [[ -z "$pseudo" ]]; then
        echo "ERROR: could not parse version from go list output for ${dep}@${SHA}:" >&2
        echo "$list_out" >&2
        exit 1
      fi
      go mod edit -C "$mod" -require "${dep}@${pseudo}"
      go mod download -C "$mod" "${dep}@${pseudo}"
    fi
  done
done

if ! $any_changed; then
  echo "No cross-module requires found to bump."
fi

echo ""
echo "Done. Review go.mod/go.sum diffs, then commit."