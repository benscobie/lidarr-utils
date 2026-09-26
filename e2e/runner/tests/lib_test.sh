#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)

# shellcheck source=../lib.sh
source "$repo_root/e2e/runner/lib.sh"

failure_output=$(mktemp)
trap 'rm -f "$failure_output"' EXIT

never_ready() {
  printf 'pending\n'
  return 1
}

if wait_until "catalogue" 1 never_ready 2>"$failure_output"; then
  printf 'wait_until unexpectedly succeeded\n' >&2
  exit 1
fi

wait_error=$(<"$failure_output")
[[ "$wait_error" == *"catalogue"* ]] || {
  printf 'wait_until error did not identify catalogue: %s\n' "$wait_error" >&2
  exit 1
}
[[ "$wait_error" == *"pending"* ]] || {
  printf 'wait_until error did not include last probe output: %s\n' "$wait_error" >&2
  exit 1
}

: >"$failure_output"
if assert_eq expected actual message 2>"$failure_output"; then
  printf 'assert_eq unexpectedly succeeded\n' >&2
  exit 1
fi

assert_error=$(<"$failure_output")
[[ "$assert_error" == *"message"* ]] || {
  printf 'assert_eq error did not include message: %s\n' "$assert_error" >&2
  exit 1
}
[[ "$assert_error" == *"expected"* && "$assert_error" == *"actual"* ]] || {
  printf 'assert_eq error did not include supplied values: %s\n' "$assert_error" >&2
  exit 1
}

printf 'PASS: runner helper behavior\n'
