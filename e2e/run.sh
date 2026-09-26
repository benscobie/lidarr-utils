#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
compose_file="$repo_root/e2e/compose.yaml"

valid_scenario() {
  case "$1" in
    smoke | catalog-smoke | monitor-artist | monitor-labels | dedupe) return 0 ;;
    *) return 1 ;;
  esac
}

if (($# == 0)); then
  scenarios=(monitor-artist monitor-labels dedupe)
else
  scenarios=("$@")
fi

for scenario in "${scenarios[@]}"; do
  if ! valid_scenario "$scenario"; then
    printf 'unknown scenario: %s\n' "$scenario" >&2
    exit 2
  fi
  if [[ ! -x "$repo_root/e2e/runner/scenarios/$scenario.sh" ]]; then
    printf 'scenario not implemented: %s\n' "$scenario" >&2
    exit 2
  fi
done

project_name=${E2E_PROJECT_NAME:-lidarr-utils-e2e-$(date +%s)-$$}
if [[ ! "$project_name" =~ ^[a-z0-9][a-z0-9_-]*$ ]]; then
  printf 'invalid E2E_PROJECT_NAME: %s\n' "$project_name" >&2
  exit 2
fi

artifact_root="$repo_root/e2e/artifacts"
mkdir -p "$artifact_root"
artifact_root=$(cd "$artifact_root" && pwd -P)
artifact_dir=$(realpath -m -- "$artifact_root/$project_name")
if [[ "$artifact_dir" != "$artifact_root/"* ]]; then
  printf 'artifact path escapes artifact root: %s\n' "$artifact_dir" >&2
  exit 2
fi
if [[ -e "$artifact_dir" ]]; then
  rm -rf "$artifact_dir"
fi
mkdir -p "$artifact_dir"
export E2E_ARTIFACT_DIR=$artifact_dir

compose=(docker compose --project-name "$project_name" --file "$compose_file")

cleanup() {
  local status=$?
  trap - EXIT

  if ((status != 0)); then
    "${compose[@]}" logs --no-color >"$artifact_dir/compose.log" 2>&1 || true
    chmod -R a+rX "$artifact_dir" || true
    printf 'E2E artifacts retained at %s\n' "$artifact_dir" >&2
  fi

  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true

  if ((status == 0)); then
    rm -rf "$artifact_dir"
  fi

  exit "$status"
}
trap cleanup EXIT

"${compose[@]}" pull lidarr fixtures
"${compose[@]}" build runner
"${compose[@]}" up --detach lidarr fixtures
"${compose[@]}" run --rm runner "${scenarios[@]}"
