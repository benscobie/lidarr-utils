#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=lib.sh
source /e2e/runner/lib.sh

export LIDARR_URL=${LIDARR_URL:-http://lidarr:8686}

config_ready() {
  [[ -s /config/config.xml ]] || {
    printf 'config.xml not ready'
    return 1
  }
}

wait_until "Lidarr config.xml" 120 config_ready >/dev/null
export LIDARR_API_KEY
LIDARR_API_KEY=$(xmlstarlet sel -t -v '/Config/ApiKey' /config/config.xml)

lidarr_ready() {
  local status
  status=$(lidarr_api GET /api/v1/system/status) || return 1
  jq -e '.version != null' <<<"$status" >/dev/null || {
    printf '%s' "$status"
    return 1
  }
  printf '%s' "$status"
}

wait_until "Lidarr API" 180 lidarr_ready >/dev/null

for scenario in "$@"; do
  log "running $scenario"
  "/e2e/runner/scenarios/$scenario.sh"
done
