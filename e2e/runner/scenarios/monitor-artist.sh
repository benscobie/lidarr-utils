#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=../lib.sh
source /e2e/runner/lib.sh
# shellcheck source=../ids.sh
source /e2e/runner/ids.sh

scenario_dir=/work/monitor-artist
lidarr-utils --config "$scenario_dir/config.yaml" monitor artist "$MONITOR_ARTIST_MBID"

albums=$(lidarr_api GET /api/v1/album)
preferred_id=$(jq -r --arg id "$MONITOR_PREFERRED_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .id' <<<"$albums")
covered_id=$(jq -r --arg id "$MONITOR_COVERED_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .id' <<<"$albums")

[[ "$preferred_id" =~ ^[0-9]+$ ]] || fail "Preferred Album has no numeric Lidarr ID"
[[ "$covered_id" =~ ^[0-9]+$ ]] || fail "Covered Single has no numeric Lidarr ID"

assert_eq true \
  "$(jq -r --arg id "$MONITOR_PREFERRED_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .monitored' <<<"$albums")" \
  "Preferred Album monitored state"
assert_eq false \
  "$(jq -r --arg id "$MONITOR_COVERED_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .monitored' <<<"$albums")" \
  "Covered Single monitored state"

assert_eq 1 \
  "$(jq --arg id "$preferred_id" '[.monitored_albums | keys[] | select(. == $id)] | length' "$scenario_dir/state.json")" \
  "Preferred Album state entry"

commands=$(lidarr_api GET /api/v1/command)
assert_eq 1 \
  "$(jq --argjson id "$preferred_id" '[.[] | select(.name == "AlbumSearch" and (.body.albumIds | index($id)))] | length' <<<"$commands")" \
  "Preferred Album search command"

log "monitor-artist passed"
