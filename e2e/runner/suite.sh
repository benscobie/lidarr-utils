#!/usr/bin/env bash
set -euo pipefail

runner_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=lib.sh
source "$runner_dir/lib.sh"

scenario_command() {
  local scenario=$1
  local function_name=${scenario//-/_}

  if declare -F "$function_name" >/dev/null; then
    "$function_name"
  else
    "$runner_dir/scenarios/$scenario.sh"
  fi
}

start_released() {
  [[ -f "$E2E_RUN_DIR/start" ]] || {
    printf 'start barrier not released'
    return 1
  }
}

run_one_scenario() {
  local scenario=$1
  local status

  touch "$E2E_RUN_DIR/$scenario.ready"
  if wait_until "start barrier for $scenario" "$E2E_SCENARIO_READY_TIMEOUT" start_released >/dev/null; then
    if scenario_command "$scenario"; then
      status=0
    else
      status=$?
    fi
  else
    status=1
  fi

  printf '%s\n' "$status" >"$E2E_ARTIFACT_DIR/$scenario.status"
  exit "$status"
}

all_scenarios_ready() {
  local scenario
  local -a missing=()

  for scenario in "$@"; do
    [[ -f "$E2E_RUN_DIR/$scenario.ready" ]] || missing+=("$scenario")
  done

  ((${#missing[@]} == 0)) || {
    printf 'missing ready markers: %s' "${missing[*]}"
    return 1
  }
}

run_scenarios() {
  local scenario pid status
  local aggregate_status=0
  local orchestration_status=0
  local -a scenarios=("$@")
  local -a pids=()

  : "${E2E_ARTIFACT_DIR:?E2E_ARTIFACT_DIR must be set}"
  export E2E_RUN_DIR=${E2E_RUN_DIR:-/work/run}
  export E2E_SCENARIO_READY_TIMEOUT=${E2E_SCENARIO_READY_TIMEOUT:-30}
  umask 022
  mkdir -p "$E2E_ARTIFACT_DIR" "$E2E_RUN_DIR"
  rm -f "$E2E_RUN_DIR/start"

  for scenario in "${scenarios[@]}"; do
    rm -f "$E2E_RUN_DIR/$scenario.ready"
    log "starting $scenario"
    run_one_scenario "$scenario" \
      >"$E2E_ARTIFACT_DIR/$scenario.stdout" \
      2>"$E2E_ARTIFACT_DIR/$scenario.stderr" &
    pids+=("$!")
  done

  if ! wait_until "all scenario ready markers" "$E2E_SCENARIO_READY_TIMEOUT" all_scenarios_ready "${scenarios[@]}" \
    >"$E2E_ARTIFACT_DIR/orchestration.stdout" \
    2>"$E2E_ARTIFACT_DIR/orchestration.stderr"; then
    orchestration_status=1
    aggregate_status=1
  fi
  printf '%s\n' "$orchestration_status" >"$E2E_ARTIFACT_DIR/orchestration.status"

  log "releasing ${#scenarios[@]} scenario(s) from the shared start barrier"
  touch "$E2E_RUN_DIR/start"

  for pid in "${pids[@]}"; do
    if ! wait "$pid"; then
      aggregate_status=1
    fi
  done

  if ((orchestration_status != 0)); then
    cat "$E2E_ARTIFACT_DIR/orchestration.stderr" >&2
  fi

  for scenario in "${scenarios[@]}"; do
    if [[ -r "$E2E_ARTIFACT_DIR/$scenario.status" ]]; then
      status=$(<"$E2E_ARTIFACT_DIR/$scenario.status")
    else
      status=1
      log "$scenario exited without writing a status artifact"
    fi
    printf '\n=== %s stdout (status %s) ===\n' "$scenario" "$status"
    cat "$E2E_ARTIFACT_DIR/$scenario.stdout"
    if [[ -s "$E2E_ARTIFACT_DIR/$scenario.stderr" ]]; then
      printf '\n=== %s stderr ===\n' "$scenario"
      cat "$E2E_ARTIFACT_DIR/$scenario.stderr" >&2
    fi
    ((status == 0)) || aggregate_status=1
  done

  return "$aggregate_status"
}

scenario_selected() {
  local wanted=$1
  shift
  local scenario
  for scenario in "$@"; do
    [[ "$scenario" == "$wanted" ]] && return 0
  done
  return 1
}

assert_shared_postconditions() {
  local albums
  albums=$(lidarr_api GET /api/v1/album)

  assert_eq true \
    "$(jq -r --arg id "$MONITOR_PREFERRED_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .monitored' <<<"$albums")" \
    "Preferred Album post-suite monitored state"
  assert_eq true \
    "$(jq -r --arg id "$LABEL_ALBUM_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .monitored' <<<"$albums")" \
    "Label Album post-suite monitored state"
  assert_eq true \
    "$(jq -r --arg id "$DEDUPE_ALBUM_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .monitored' <<<"$albums")" \
    "Dedupe Album post-suite monitored state"
  # shellcheck source=/dev/null
  source /work/dedupe/audio.env
  [[ -f "$DEDUPE_ALBUM_FILE" ]] || fail "Dedupe Album file missing after shared suite"
  log "shared-stack postconditions passed"
}

main() {
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

  if !(($# == 1)) || [[ $1 != smoke ]]; then
    # shellcheck source=setup.sh
    source "$runner_dir/setup.sh"
    setup_suite
  fi

  run_scenarios "$@"

  if scenario_selected monitor-artist "$@" &&
    scenario_selected monitor-labels "$@" &&
    scenario_selected dedupe "$@"; then
    # shellcheck source=ids.sh
    source "$runner_dir/ids.sh"
    assert_shared_postconditions
  fi
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
  main "$@"
fi
