#!/usr/bin/env bash

# shellcheck source=lib.sh
source /e2e/runner/lib.sh
# shellcheck source=ids.sh
source /e2e/runner/ids.sh

write_silent_flac() {
  local output=$1
  local album=$2
  local release_id=$3
  local release_group_id=$4
  local release_track_id=$5

  ffmpeg \
    -hide_banner \
    -loglevel error \
    -f lavfi \
    -i 'anullsrc=r=44100:cl=stereo' \
    -t 1 \
    -c:a flac \
    -metadata 'ARTIST=Dedupe Artist' \
    -metadata 'ALBUMARTIST=Dedupe Artist' \
    -metadata "ALBUM=$album" \
    -metadata 'TITLE=Duplicate Song' \
    -metadata 'TRACKNUMBER=1' \
    -metadata 'DISCNUMBER=1' \
    -metadata 'DATE=2020' \
    -metadata "MUSICBRAINZ_ARTISTID=$DEDUPE_ARTIST_MBID" \
    -metadata "MUSICBRAINZ_ALBUMARTISTID=$DEDUPE_ARTIST_MBID" \
    -metadata "MUSICBRAINZ_ALBUMID=$release_id" \
    -metadata "MUSICBRAINZ_RELEASEGROUPID=$release_group_id" \
    -metadata "MUSICBRAINZ_TRACKID=$DEDUPE_SHARED_RECORDING_MBID" \
    -metadata "MUSICBRAINZ_RELEASETRACKID=$release_track_id" \
    "$output"
}

command_completed() {
  local command_id=$1
  local command status message

  command=$(lidarr_api GET "/api/v1/command/$command_id") || return 1
  status=$(jq -r '.status' <<<"$command")
  if [[ "$status" == failed ]]; then
    message=$(jq -r '.message // "no failure message"' <<<"$command")
    fail "RescanFolders command failed: $message"
    return 2
  fi

  [[ "$status" == completed ]] || {
    printf 'status=%s' "$status"
    return 1
  }
}

audio_imported() {
  local album_id=$1
  local tracks
  tracks=$(lidarr_api GET "/api/v1/track?albumId=$album_id") || return 1
  [[ $(jq '[.[] | select(.hasFile)] | length' <<<"$tracks") == 1 ]] || {
    jq -c '[.[] | {title, hasFile}]' <<<"$tracks"
    return 1
  }
}

seed_audio() {
  local artist artist_id artist_path albums album_id single_id album_dir single_dir
  local album_file single_file monitor_payload response response_body response_status command_id

  log "seeding tagged audio for dedupe"
  artist=$(lidarr_api GET "/api/v1/artist?mbId=$DEDUPE_ARTIST_MBID")
  assert_eq 1 "$(jq 'length' <<<"$artist")" "dedupe artist lookup"
  artist_id=$(jq -r '.[0].id' <<<"$artist")
  artist_path=$(jq -r '.[0].path' <<<"$artist")

  albums=$(lidarr_api GET /api/v1/album)
  album_id=$(jq -r --arg id "$DEDUPE_ALBUM_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .id' <<<"$albums")
  single_id=$(jq -r --arg id "$DEDUPE_SINGLE_RG_MBID" '.[] | select(.foreignAlbumId == $id) | .id' <<<"$albums")
  [[ -n "$album_id" && -n "$single_id" ]] || fail "dedupe albums missing from Lidarr catalogue"

  album_dir="$artist_path/Dedupe Album (2020)"
  single_dir="$artist_path/Duplicate Single (2021)"
  album_file="$album_dir/01 - Duplicate Song.flac"
  single_file="$single_dir/01 - Duplicate Song.flac"
  mkdir -p "$album_dir" "$single_dir"

  write_silent_flac "$album_file" "Dedupe Album" "$DEDUPE_ALBUM_RELEASE_MBID" "$DEDUPE_ALBUM_RG_MBID" "$DEDUPE_ALBUM_TRACK_MBID"
  write_silent_flac "$single_file" "Duplicate Single" "$DEDUPE_SINGLE_RELEASE_MBID" "$DEDUPE_SINGLE_RG_MBID" "$DEDUPE_SINGLE_TRACK_MBID"

  chown -R 1001:1001 "$artist_path"
  su-exec 1001:1001 test -w "$album_dir"
  su-exec 1001:1001 test -w "$single_dir"

  mkdir -p /work/dedupe
  printf 'DEDUPE_ALBUM_FILE=%q\nDEDUPE_SINGLE_FILE=%q\n' "$album_file" "$single_file" > /work/dedupe/audio.env

  monitor_payload=$(jq -n --argjson album "$album_id" --argjson single "$single_id" \
    '{albumIds: [$album, $single], monitored: true}')
  lidarr_api PUT /api/v1/album/monitor "$monitor_payload" >/dev/null

  response=$(curl \
    --silent \
    --show-error \
    --header "X-Api-Key: $LIDARR_API_KEY" \
    --header 'Content-Type: application/json' \
    --data "$(jq -n --arg folder "$artist_path" --argjson artist "$artist_id" \
      '{name: "RescanFolders", folders: [$folder], filter: "none", addNewArtists: false, artistIds: [$artist]}')" \
    --write-out $'\n%{http_code}' \
    "${LIDARR_URL%/}/api/v1/command")
  response_status=${response##*$'\n'}
  response_body=${response%$'\n'*}
  assert_eq 201 "$response_status" "RescanFolders response status"
  command_id=$(jq -r '.id' <<<"$response_body")
  [[ "$command_id" =~ ^[0-9]+$ ]] || fail "RescanFolders returned invalid command id: $command_id"

  wait_until "RescanFolders command" 120 command_completed "$command_id" >/dev/null
  wait_until "Dedupe Album audio import" 60 audio_imported "$album_id" >/dev/null
  wait_until "Duplicate Single audio import" 60 audio_imported "$single_id" >/dev/null
  log "tagged audio seed passed"
}
