#!/usr/bin/env bash

# shellcheck source=lib.sh
source /e2e/runner/lib.sh
# shellcheck source=ids.sh
source /e2e/runner/ids.sh

configure_metadata_source() {
  local config updated actual
  config=$(lidarr_api GET /api/v1/config/metadataprovider)
  updated=$(jq '.metadataSource = "http://fixtures/api/v0.4"' <<<"$config")
  lidarr_api PUT /api/v1/config/metadataprovider "$updated" >/dev/null
  actual=$(lidarr_api GET /api/v1/config/metadataprovider | jq -r '.metadataSource')
  assert_eq "http://fixtures/api/v0.4" "$actual" "Lidarr metadata source"
}

configure_profiles_and_root() {
  local metadata_profiles metadata_profile metadata_profile_id quality_profile_id root_payload

  metadata_profiles=$(lidarr_api GET /api/v1/metadataprofile)
  metadata_profile=$(jq '.[0]
    | .primaryAlbumTypes |= map(
        if (.albumType.name == "Album" or .albumType.name == "Single")
        then .allowed = true
        else .
        end
      )' <<<"$metadata_profiles")
  metadata_profile_id=$(jq -r '.id' <<<"$metadata_profile")
  lidarr_api PUT "/api/v1/metadataprofile/$metadata_profile_id" "$metadata_profile" >/dev/null

  quality_profile_id=$(lidarr_api GET /api/v1/qualityprofile | jq -r '.[0].id')
  root_payload=$(jq -n \
    --arg path /music \
    --arg name 'E2E Music' \
    --argjson metadata "$metadata_profile_id" \
    --argjson quality "$quality_profile_id" \
    '{
      name: $name,
      path: $path,
      defaultMetadataProfileId: $metadata,
      defaultQualityProfileId: $quality,
      defaultMonitorOption: "none",
      defaultNewItemMonitorOption: "none",
      defaultTags: []
    }')
  lidarr_api POST /api/v1/rootfolder "$root_payload" >/dev/null

  export E2E_METADATA_PROFILE_ID=$metadata_profile_id
  export E2E_QUALITY_PROFILE_ID=$quality_profile_id
}

add_artist() {
  local artist_mbid=$1
  local lookup payload

  lookup=$(lidarr_api GET "/api/v1/artist/lookup?term=lidarr:$artist_mbid")
  assert_eq 1 "$(jq 'length' <<<"$lookup")" "artist lookup $artist_mbid"

  payload=$(jq \
    --arg root /music \
    --argjson quality "$E2E_QUALITY_PROFILE_ID" \
    --argjson metadata "$E2E_METADATA_PROFILE_ID" \
    '.[0] + {
      rootFolderPath: $root,
      qualityProfileId: $quality,
      metadataProfileId: $metadata,
      monitored: false,
      monitorNewItems: "none",
      tags: [],
      addOptions: {
        monitor: "none",
        monitored: false,
        searchForMissingAlbums: false,
        albumsToMonitor: []
      }
    }' <<<"$lookup")

  lidarr_api POST /api/v1/artist "$payload" >/dev/null
}

catalogue_ready() {
  local albums count
  albums=$(lidarr_api GET /api/v1/album) || return 1
  count=$(jq \
    --arg one "$MONITOR_PREFERRED_RG_MBID" \
    --arg two "$MONITOR_COVERED_RG_MBID" \
    --arg three "$LABEL_ALBUM_RG_MBID" \
    --arg four "$LABEL_UNRELATED_RG_MBID" \
    --arg five "$DEDUPE_ALBUM_RG_MBID" \
    --arg six "$DEDUPE_SINGLE_RG_MBID" \
    '[.[] | select(.foreignAlbumId == $one or .foreignAlbumId == $two or
                   .foreignAlbumId == $three or .foreignAlbumId == $four or
                   .foreignAlbumId == $five or .foreignAlbumId == $six)] | length' \
    <<<"$albums")
  [[ "$count" == 6 ]] || {
    jq -c '[.[] | {title, foreignAlbumId}]' <<<"$albums"
    return 1
  }
}

write_cli_config() {
  local scenario=$1
  local scenario_dir="/work/$scenario"
  mkdir -p "$scenario_dir"

  cat >"$scenario_dir/config.yaml" <<EOF
lidarr:
  url: "$LIDARR_URL"
  api_key: "$LIDARR_API_KEY"
musicbrainz:
  url: "http://fixtures/ws/2"
app:
  dry_run: false
  log_level: debug
  log_file: "$scenario_dir/lidarr-utils.log"
  state_file: "$scenario_dir/state.json"
dedupe:
  add_import_exclusion: false
monitor:
  artists:
    selection:
      include_secondary_types: true
      exclude_secondary_types: []
      exclude_formats: []
      various_artists: include
      compilation_singles: include
      skip_fully_covered_releases: true
  labels:
    ids: []
    missing_artists:
      enabled: false
      root_folder: ""
    selection:
      include_secondary_types: true
      exclude_secondary_types: []
      exclude_formats: []
      various_artists: include
      compilation_singles: include
      skip_fully_covered_releases: true
EOF
}

setup_suite() {
  log "configuring deterministic Lidarr catalogue"
  chown 1001:1001 /music
  configure_metadata_source
  configure_profiles_and_root
  add_artist "$MONITOR_ARTIST_MBID"
  add_artist "$LABEL_ARTIST_MBID"
  add_artist "$DEDUPE_ARTIST_MBID"
  wait_until "six fixture albums" 180 catalogue_ready >/dev/null

  write_cli_config monitor-artist
  write_cli_config monitor-labels
  write_cli_config dedupe
  log "catalogue setup passed"
}
