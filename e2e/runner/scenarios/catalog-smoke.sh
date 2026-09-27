#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=../lib.sh
source /e2e/runner/lib.sh
# shellcheck source=../ids.sh
source /e2e/runner/ids.sh

albums=$(lidarr_api GET /api/v1/album)

assert_unmonitored_album() {
  local release_group_id=$1
  local title=$2
  local matches monitored

  matches=$(jq --arg id "$release_group_id" '[.[] | select(.foreignAlbumId == $id)] | length' <<<"$albums")
  assert_eq 1 "$matches" "$title catalogue membership"

  monitored=$(jq -r --arg id "$release_group_id" '.[] | select(.foreignAlbumId == $id) | .monitored' <<<"$albums")
  assert_eq false "$monitored" "$title initial monitored state"
}

assert_unmonitored_album "$MONITOR_PREFERRED_RG_MBID" "Preferred Album"
assert_unmonitored_album "$MONITOR_COVERED_RG_MBID" "Covered Single"
assert_unmonitored_album "$LABEL_ALBUM_RG_MBID" "Label Album"
assert_unmonitored_album "$LABEL_UNRELATED_RG_MBID" "Unrelated Album"
assert_unmonitored_album "$DEDUPE_ALBUM_RG_MBID" "Dedupe Album"
assert_unmonitored_album "$DEDUPE_SINGLE_RG_MBID" "Duplicate Single"

label_response=$(curl --get --fail-with-body --silent --show-error \
  --data-urlencode "label=$LABEL_MBID" \
  --data-urlencode 'fmt=json' \
  http://fixtures/ws/2/release)

assert_eq 1 "$(jq '.releases | length' <<<"$label_response")" "fixture label release count"
assert_eq "$LABEL_ALBUM_RG_MBID" \
  "$(jq -r '.releases[0]["release-group"].id' <<<"$label_response")" \
  "fixture label release group"

log "catalog smoke passed"
