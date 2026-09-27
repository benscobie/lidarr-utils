#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=../lib.sh
source /e2e/runner/lib.sh

fixture_ready() {
  local response
  response=$(curl --fail-with-body --silent --show-error http://fixtures/health.json) || return 1
  jq -e '.status == "ok"' <<<"$response" >/dev/null || {
    printf '%s' "$response"
    return 1
  }
  printf '%s' "$response"
}

lidarr_version_ready() {
  local status version
  status=$(lidarr_api GET /api/v1/system/status) || return 1
  version=$(jq -r '.version // empty' <<<"$status")
  [[ "$version" == 3.1.0.4875 ]] || {
    printf 'version=%s' "$version"
    return 1
  }
  printf 'version=%s' "$version"
}

wait_until "fixture server" 30 fixture_ready >/dev/null
wait_until "Lidarr version 3.1.0.4875" 30 lidarr_version_ready >/dev/null
log "smoke passed"
