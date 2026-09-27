#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=../lib.sh
source /e2e/runner/lib.sh
# shellcheck source=../ids.sh
source /e2e/runner/ids.sh
# shellcheck source=/dev/null
source /work/dedupe/audio.env

track_file_count() {
  local album_id=$1
  lidarr_api GET "/api/v1/track?albumId=$album_id" | jq '[.[] | select(.hasFile)] | length'
}

album_monitored() {
  local release_group_id=$1
  lidarr_api GET /api/v1/album | jq -r --arg id "$release_group_id" '.[] | select(.foreignAlbumId == $id) | .monitored'
}

assert_file_exists() {
  local path=$1
  local message=$2
  [[ -f "$path" ]] || fail "$message: $path"
}

albums=$(lidarr_api GET /api/v1/album)
dedupe_album_id=$(jq -r --arg id "$DEDUPE_ALBUM_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .id' <<<"$albums")
dedupe_single_id=$(jq -r --arg id "$DEDUPE_SINGLE_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .id' <<<"$albums")

assert_file_exists "$DEDUPE_ALBUM_FILE" "Dedupe Album seed file missing"
assert_file_exists "$DEDUPE_SINGLE_FILE" "Duplicate Single seed file missing"
assert_eq 1 "$(track_file_count "$dedupe_album_id")" "Dedupe Album initial track-file count"
assert_eq 1 "$(track_file_count "$dedupe_single_id")" "Duplicate Single initial track-file count"
assert_eq true "$(album_monitored "$DEDUPE_ALBUM_RG_MBID")" "Dedupe Album initial monitored state"
assert_eq true "$(album_monitored "$DEDUPE_SINGLE_RG_MBID")" "Duplicate Single initial monitored state"

lidarr-utils --config /work/dedupe/config.yaml dedupe --dry-run

assert_file_exists "$DEDUPE_ALBUM_FILE" "dry-run removed album file"
assert_file_exists "$DEDUPE_SINGLE_FILE" "dry-run removed single file"
assert_eq 1 "$(track_file_count "$dedupe_album_id")" "Dedupe Album dry-run track-file count"
assert_eq 1 "$(track_file_count "$dedupe_single_id")" "Duplicate Single dry-run track-file count"
assert_eq true "$(album_monitored "$DEDUPE_SINGLE_RG_MBID")" "Duplicate Single dry-run monitored state"

lidarr-utils --config /work/dedupe/config.yaml dedupe

single_removed() {
  local monitored files
  monitored=$(album_monitored "$DEDUPE_SINGLE_RG_MBID") || return 1
  files=$(track_file_count "$dedupe_single_id") || return 1
  [[ "$monitored" == false && "$files" == 0 && ! -e "$DEDUPE_SINGLE_FILE" ]] || {
    printf 'monitored=%s files=%s disk=%s' "$monitored" "$files" "$(test -e "$DEDUPE_SINGLE_FILE" && printf present || printf absent)"
    return 1
  }
}

wait_until "duplicate single deletion" 30 single_removed >/dev/null
assert_file_exists "$DEDUPE_ALBUM_FILE" "real dedupe removed album file"
assert_eq 1 "$(track_file_count "$dedupe_album_id")" "Dedupe Album final track-file count"
assert_eq true "$(album_monitored "$DEDUPE_ALBUM_RG_MBID")" "Dedupe Album final monitored state"

log "dedupe passed"
