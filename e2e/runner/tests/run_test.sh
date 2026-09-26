#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
test_dir=$(mktemp -d)
outside_dir="$repo_root/e2e/run-test-outside"
artifact_link="$repo_root/e2e/artifacts/run-test-symlink"
trap 'rm -rf "$test_dir" "$outside_dir" "$artifact_link"' EXIT

mkdir -p "$test_dir/bin" "$outside_dir"
printf 'must survive\n' >"$outside_dir/sentinel"

cat >"$test_dir/bin/docker" <<'EOF'
#!/usr/bin/env sh
: >"$FAKE_DOCKER_MARKER"
EOF
chmod +x "$test_dir/bin/docker"

export FAKE_DOCKER_MARKER="$test_dir/docker-invoked"
set +e
output=$(PATH="$test_dir/bin:$PATH" E2E_PROJECT_NAME=../run-test-outside "$repo_root/e2e/run.sh" smoke 2>&1)
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

mkdir -p "$test_dir/symlink-target"
printf 'must also survive\n' >"$test_dir/symlink-target/sentinel"
ln -s "$test_dir/symlink-target" "$artifact_link"

set +e
output=$(PATH="$test_dir/bin:$PATH" E2E_PROJECT_NAME=run-test-symlink "$repo_root/e2e/run.sh" smoke 2>&1)
status=$?
set -e

[[ $status == 2 ]] || {
  printf 'expected escaping artifact symlink to exit 2, got %s\n%s\n' "$status" "$output" >&2
  exit 1
}
grep -q 'artifact path escapes artifact root' <<<"$output"
[[ -f "$test_dir/symlink-target/sentinel" ]] || {
  printf 'artifact symlink escaped the artifact root and deleted the sentinel\n' >&2
  exit 1
}
[[ ! -e "$FAKE_DOCKER_MARKER" ]] || {
  printf 'Docker was invoked before artifact-path validation\n' >&2
  exit 1
}

printf 'host runner validation tests passed\n'
