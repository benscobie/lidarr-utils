#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
# shellcheck source=../host-lib.sh
source "$repo_root/e2e/runner/host-lib.sh"
test_dir=$(mktemp -d)
outside_dir=$(mktemp -d "$repo_root/e2e/run-test-outside.XXXXXX")
cleanup() {
  rm -rf "$test_dir" "$outside_dir"
}
trap cleanup EXIT

mkdir -p "$test_dir/bin"
printf 'must survive\n' >"$outside_dir/sentinel"

cat >"$test_dir/bin/docker" <<'EOF'
#!/usr/bin/env sh
: >"$FAKE_DOCKER_MARKER"
EOF
chmod +x "$test_dir/bin/docker"

export FAKE_DOCKER_MARKER="$test_dir/docker-invoked"
set +e
output=$(PATH="$test_dir/bin:$PATH" E2E_PROJECT_NAME="../$(basename "$outside_dir")" "$repo_root/e2e/run.sh" smoke 2>&1)
status=$?
set -e

[[ $status == 2 ]] || {
  printf 'expected invalid project name to exit 2, got %s\n%s\n' "$status" "$output" >&2
  exit 1
}
grep -q 'invalid E2E_PROJECT_NAME' <<<"$output"
[[ -f "$outside_dir/sentinel" ]] || {
  printf 'invalid project name escaped the artifact root and deleted the sentinel\n' >&2
  exit 1
}
[[ ! -e "$FAKE_DOCKER_MARKER" ]] || {
  printf 'Docker was invoked before project-name validation\n' >&2
  exit 1
}

mkdir -p "$test_dir/root-target/internal"
printf 'root target must survive\n' >"$test_dir/root-target/internal/sentinel"
ln -s "$test_dir/root-target" "$test_dir/root-link"

set +e
output=$(resolve_artifact_dir "$test_dir/root-link" internal 2>&1)
status=$?
set -e

[[ $status == 2 ]] || {
  printf 'expected artifact-root symlink to exit 2, got %s\n%s\n' "$status" "$output" >&2
  exit 1
}
grep -q 'artifact root must not be a symlink' <<<"$output"
[[ -f "$test_dir/root-target/internal/sentinel" ]] || {
  printf 'artifact-root symlink target was modified\n' >&2
  exit 1
}

mkdir -p "$test_dir/child-root/owned"
printf 'child target must survive\n' >"$test_dir/child-root/owned/sentinel"
ln -s "$test_dir/child-root/owned" "$test_dir/child-root/internal"

set +e
output=$(resolve_artifact_dir "$test_dir/child-root" internal 2>&1)
status=$?
set -e

[[ $status == 2 ]] || {
  printf 'expected artifact-directory symlink to exit 2, got %s\n%s\n' "$status" "$output" >&2
  exit 1
}
grep -q 'artifact directory must not be a symlink' <<<"$output"
[[ -f "$test_dir/child-root/owned/sentinel" ]] || {
  printf 'artifact-directory symlink target was modified\n' >&2
  exit 1
}

printf 'host runner validation tests passed\n'
