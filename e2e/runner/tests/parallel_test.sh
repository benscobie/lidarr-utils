#!/usr/bin/env bash
set -euo pipefail

runner_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=../suite.sh
source "$runner_dir/suite.sh"

test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
export E2E_ARTIFACT_DIR="$test_dir/artifacts"
export E2E_RUN_DIR="$test_dir/run"
export E2E_SCENARIO_READY_TIMEOUT=5

assert_all_released_together() {
  local scenario
  for scenario in probe-one probe-two probe-fail; do
    [[ -f "$E2E_RUN_DIR/$scenario.ready" ]]
  done
  [[ -f "$E2E_RUN_DIR/start" ]]
}

probe_one() {
  assert_all_released_together
  touch "$E2E_RUN_DIR/probe-one.complete"
}

probe_two() {
  assert_all_released_together
  touch "$E2E_RUN_DIR/probe-two.complete"
}

probe_fail() {
  assert_all_released_together
  touch "$E2E_RUN_DIR/probe-fail.complete"
  return 23
}

set +e
run_scenarios probe-one probe-two probe-fail
status=$?
set -e

[[ $status -ne 0 ]] || fail "parallel runner did not aggregate the failing scenario"
for scenario in probe-one probe-two probe-fail; do
  [[ -f "$E2E_RUN_DIR/$scenario.ready" ]] || fail "$scenario never reached the start barrier"
  [[ -f "$E2E_RUN_DIR/$scenario.complete" ]] || fail "$scenario was cancelled before completion"
done
assert_eq 0 "$(<"$E2E_ARTIFACT_DIR/probe-one.status")" "probe-one status"
assert_eq 0 "$(<"$E2E_ARTIFACT_DIR/probe-two.status")" "probe-two status"
assert_eq 23 "$(<"$E2E_ARTIFACT_DIR/probe-fail.status")" "probe-fail status"

printf 'parallel aggregation tests passed\n'
