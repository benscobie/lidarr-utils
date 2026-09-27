#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=../lib.sh
source /e2e/runner/lib.sh
# shellcheck source=../ids.sh
source /e2e/runner/ids.sh

scenario_dir=/work/monitor-labels
lidarr-utils --config "$scenario_dir/config.yaml" monitor labels "$LABEL_MBID"

albums=$(lidarr_api GET /api/v1/album)
label_album_id=$(jq -r --arg id "$LABEL_ALBUM_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .id' <<<"$albums")
unrelated_album_id=$(jq -r --arg id "$LABEL_UNRELATED_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .id' <<<"$albums")

[[ "$label_album_id" =~ ^[0-9]+$ ]] || fail "Label Album has no numeric Lidarr ID"
[[ "$unrelated_album_id" =~ ^[0-9]+$ ]] || fail "Unrelated Album has no numeric Lidarr ID"

assert_eq true \
  "$(jq -r --arg id "$LABEL_ALBUM_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .monitored' <<<"$albums")" \
  "Label Album monitored state"
assert_eq false \
  "$(jq -r --arg id "$LABEL_UNRELATED_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .monitored' <<<"$albums")" \
  "Unrelated Album monitored state"

assert_eq 1 "$(jq '.monitored_albums | length' "$scenario_dir/state.json")" "label state entry count"
assert_eq 1 \
  "$(jq --arg id "$label_album_id" '[.monitored_albums | keys[] | select(. == $id)] | length' "$scenario_dir/state.json")" \
  "Label Album state entry"

commands=$(lidarr_api GET /api/v1/command)
assert_eq 1 \
  "$(jq --argjson id "$label_album_id" '[.[] | select(.name == "AlbumSearch" and (.body.albumIds | index($id)))] | length' <<<"$commands")" \
  "Label Album search command"

grep -q 'GET /ws/2/release?' /work/fixtures/access.log || fail "MusicBrainz fixture route was not requested"

log "monitor-labels passed"
