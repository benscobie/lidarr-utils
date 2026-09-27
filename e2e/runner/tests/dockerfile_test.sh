#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
go_version=$(awk '$1 == "go" { print $2; exit }' "$repo_root/go.mod")
builder_version=$(sed -n 's/^FROM golang:\([^@]*\)@.*/\1/p' "$repo_root/e2e/Dockerfile" | head -n 1)

[[ -n "$go_version" ]] || {
  printf 'go.mod does not declare a Go version\n' >&2
  exit 1
}
[[ -n "$builder_version" ]] || {
  printf 'e2e/Dockerfile does not use a digest-pinned golang builder image\n' >&2
  exit 1
}
[[ "$builder_version" == "$go_version" ]] || {
  printf 'E2E builder Go version %s does not match go.mod version %s\n' "$builder_version" "$go_version" >&2
  exit 1
}

printf 'E2E Dockerfile version tests passed\n'
