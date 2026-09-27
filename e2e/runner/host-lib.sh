#!/usr/bin/env bash

validate_project_name() {
  local project_name=$1
  [[ "$project_name" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || {
    printf 'invalid E2E_PROJECT_NAME: %s\n' "$project_name" >&2
    return 2
  }
}

resolve_artifact_dir() {
  local requested_root=$1
  local project_name=$2
  local root_parent artifact_root artifact_dir

  validate_project_name "$project_name" || return

  root_parent=$(cd "$(dirname "$requested_root")" && pwd -P)
  artifact_root="$root_parent/$(basename "$requested_root")"
  if [[ -L "$artifact_root" ]]; then
    printf 'artifact root must not be a symlink: %s\n' "$artifact_root" >&2
    return 2
  fi

  mkdir -p "$artifact_root"
  artifact_dir="$artifact_root/$project_name"
  if [[ -L "$artifact_dir" ]]; then
    printf 'artifact directory must not be a symlink: %s\n' "$artifact_dir" >&2
    return 2
  fi

  printf '%s\n' "$artifact_dir"
}
