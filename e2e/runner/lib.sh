#!/usr/bin/env bash

log() {
  printf '[e2e] %s\n' "$*" >&2
}

fail() {
  log "ERROR: $*"
  return 1
}

assert_eq() {
  local expected=$1
  local actual=$2
  local message=$3

  if [[ "$expected" != "$actual" ]]; then
    fail "$message (expected: $expected, actual: $actual)"
    return 1
  fi
}

wait_until() {
  local description=$1
  local timeout_seconds=$2
  local probe=$3
  shift 3

  local deadline=$((SECONDS + timeout_seconds))
  local last_output='probe has not run'

  while :; do
    if last_output=$("$probe" "$@" 2>&1); then
      [[ -z "$last_output" ]] || printf '%s\n' "$last_output"
      return 0
    fi

    if ((SECONDS >= deadline)); then
      fail "timed out waiting for $description after ${timeout_seconds}s; last observed state: $last_output"
      return 1
    fi

    sleep 0.2
  done
}

lidarr_api() {
  local method=$1
  local path=$2
  local body=${3-}
  local -a curl_args=(
    --fail-with-body
    --silent
    --show-error
    --request "$method"
    --header "X-Api-Key: $LIDARR_API_KEY"
  )

  if [[ -n "$body" ]]; then
    curl_args+=(
      --header 'Content-Type: application/json'
      --data "$body"
    )
  fi

  curl "${curl_args[@]}" "${LIDARR_URL%/}/${path#/}"
}
