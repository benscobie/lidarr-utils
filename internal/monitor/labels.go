package monitor

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
)

type LabelOptions struct {
	IDs            []string
	MissingArtists config.MissingArtistsConfig
	Policy         config.ReleaseSelectionPolicy
	DryRun         bool
}

type PlannedLabelAlbum struct {
	Album        common.Album
	Lookup       *lidarr.Album
	ArtistExists bool
}

type LabelPlan struct {
	Selected []PlannedLabelAlbum
	Stats    LabelStats
}

type LabelStats struct {
	LabelsProcessed       int
	ReleasesDiscovered    int
	GroupsDiscovered      int
	GroupsFiltered        int
	GroupsCoverageSkipped int
	AlreadyMonitored      int
	MissingArtistSkipped  int
	ArtistsAdded          int
	AlbumsAdded           int
	AlbumsMonitored       int
	SearchesSubmitted     int
	Failures              int
}

type resolvedLabelCandidate struct {
	album          common.Album
	lookup         *lidarr.Album
	artistExists   bool
	ownerForeignID string
}

func (m *Monitor) PlanLabels(opts LabelOptions) (LabelPlan, error) {
	var plan LabelPlan
	if m.opts.MBClient == nil {
		return plan, fmt.Errorf("MusicBrainz client is required for label monitoring")
	}

	groups, groupOrder := m.discoverLabelGroups(opts.IDs, &plan.Stats)
	plan.Stats.GroupsDiscovered = len(groupOrder)
	if len(groupOrder) == 0 {
		return plan, nil
	}

	cache := NewCatalogCache(m.opts.Client)
	artists, err := cache.Artists()
	if err != nil {
		return plan, fmt.Errorf("failed to get artists: %w", err)
	}
	artistsByForeignID := make(map[string]lidarr.Artist, len(artists))
	artistsByID := make(map[int]lidarr.Artist, len(artists))
	for _, artist := range artists {
		artistsByID[artist.ID] = artist
		if artist.ForeignID != "" {
			artistsByForeignID[artist.ForeignID] = artist
		}
	}

	resolved := make(map[string]resolvedLabelCandidate)
	buckets := make(map[string][]string)
	var bucketOrder []string
	for _, groupID := range groupOrder {
		group := groups[groupID]
		groupAlbum := albumFromLabelGroup(group)
		groupAlbum.IsVariousArtists = hasDirectVACredit(groupAlbum)
		if reason := releasePolicyExclusionReason(groupAlbum, opts.Policy); reason != "" {
			plan.Stats.GroupsFiltered++
			logExcludedAlbum(ExcludedAlbum{Album: groupAlbum, Reason: reason}, true)
			continue
		}

		creditIDs, completeCredits := usableArtistCredits(group)
		var creditedExisting []lidarr.Artist
		for _, artistID := range creditIDs {
			if artist, ok := artistsByForeignID[artistID]; ok {
				creditedExisting = append(creditedExisting, artist)
			}
		}
		if !opts.MissingArtists.Enabled &&
			completeCredits &&
			len(creditIDs) > 0 &&
			len(creditedExisting) == 0 {
			plan.Stats.MissingArtistSkipped++
			log.Printf(
				"  Skip: %s - %s (%s) — artist is not in Lidarr and add_missing_artists is false",
				groupAlbum.ArtistName,
				groupAlbum.Title,
				groupAlbum.AlbumType,
			)
			continue
		}

		var (
			existingAlbum *lidarr.Album
			owner         lidarr.Artist
		)
		for _, artist := range creditedExisting {
			albums, err := cache.AlbumsForArtist(artist.ID)
			if err != nil {
				plan.Stats.Failures++
				log.Printf("ERROR: Failed to get albums for %s: %v", artist.ArtistName, err)
				continue
			}
			if album, ok := albumByForeignID(albums, group.ID); ok {
				found := album
				existingAlbum = &found
				owner = ownerArtist(found, artist, artistsByID)
				break
			}
		}

		candidate := resolvedLabelCandidate{album: groupAlbum}
		if existingAlbum != nil {
			candidate.album = mergeLookup(groupAlbum, *existingAlbum)
			candidate.artistExists = true
			candidate.ownerForeignID = owner.ForeignID
		} else {
			lookups, err := cache.LookupAlbum(group.ID)
			if err != nil {
				plan.Stats.Failures++
				log.Printf("ERROR: Failed to look up release group %s: %v", group.ID, err)
				continue
			}
			exact, ok := exactLookup(lookups, group.ID)
			if !ok {
				plan.Stats.Failures++
				log.Printf("ERROR: Lidarr lookup was not exact for release group %s", group.ID)
				continue
			}

			owner, candidate.ownerForeignID = lookupOwner(exact, creditIDs, artistsByID)
			if owner.ID != 0 {
				candidate.artistExists = true
				candidate.ownerForeignID = owner.ForeignID
			} else if _, ok := artistsByForeignID[candidate.ownerForeignID]; ok {
				candidate.artistExists = true
			}
			if candidate.ownerForeignID == "" {
				plan.Stats.Failures++
				log.Printf("ERROR: Lidarr lookup did not identify an artist for %s", group.ID)
				continue
			}
			if !candidate.artistExists && !opts.MissingArtists.Enabled {
				plan.Stats.MissingArtistSkipped++
				log.Printf(
					"  Skip: %s - %s (%s) — artist is not in Lidarr and add_missing_artists is false",
					candidate.album.ArtistName,
					candidate.album.Title,
					candidate.album.AlbumType,
				)
				continue
			}

			candidate.album = mergeLookup(groupAlbum, exact)
			lookupCopy := exact
			if candidate.artistExists {
				existingArtist := artistsByForeignID[candidate.ownerForeignID]
				lookupCopy.Artist = &existingArtist
				lookupCopy.ArtistID = existingArtist.ID
			}
			candidate.lookup = &lookupCopy
		}

		if candidate.ownerForeignID == "" {
			plan.Stats.Failures++
			log.Printf("ERROR: Could not determine catalogue owner for %s", group.ID)
			continue
		}
		candidate.album.IsVariousArtists = candidate.ownerForeignID == musicbrainz.VariousArtistsID
		if _, exists := buckets[candidate.ownerForeignID]; !exists {
			bucketOrder = append(bucketOrder, candidate.ownerForeignID)
		}
		buckets[candidate.ownerForeignID] = append(
			buckets[candidate.ownerForeignID],
			group.ID,
		)
		resolved[group.ID] = candidate
	}

	var classifier *CompilationSingleClassifier
	if opts.Policy.CompilationSingles == config.CompilationSinglesExclude {
		classifier = NewCompilationSingleClassifier(m.opts.MBClient)
	}
	for ownerIndex, ownerID := range bucketOrder {
		candidateIDs := buckets[ownerID]
		candidateSet := make(map[string]struct{}, len(candidateIDs))
		for _, groupID := range candidateIDs {
			candidateSet[groupID] = struct{}{}
		}

		artistName := ownerID
		if owner, ok := artistsByForeignID[ownerID]; ok && owner.ArtistName != "" {
			artistName = owner.ArtistName
		} else {
			for _, groupID := range candidateIDs {
				if name := resolved[groupID].album.ArtistName; name != "" {
					artistName = name
					break
				}
			}
		}
		log.Printf(
			"Processing label artist %d/%d: %s",
			ownerIndex+1,
			len(bucketOrder),
			artistName,
		)

		var catalogue []common.Album
		var catalogueWarnings []string
		if owner, ok := artistsByForeignID[ownerID]; ok {
			raw, err := cache.AlbumsForArtist(owner.ID)
			if err != nil {
				plan.Stats.Failures += len(candidateIDs)
				log.Printf("ERROR: Failed to get albums for %s: %v", owner.ArtistName, err)
				continue
			}
			catalogue, catalogueWarnings = prepareLidarrCatalogue(
				owner,
				raw,
				cache,
				opts.Policy,
				classifier,
			)
		}
		logSelectionWarnings(catalogueWarnings)

		var synthetic []common.Album
		for _, groupID := range candidateIDs {
			candidate := resolved[groupID]
			if !containsForeignAlbum(catalogue, groupID) {
				synthetic = append(synthetic, candidate.album)
			}
		}
		for i, album := range synthetic {
			prepared, warnings := prepareAlbumForSelection(
				album,
				opts.Policy,
				classifier,
				func() ([]common.Track, error) { return album.Tracks, nil },
			)
			synthetic[i] = prepared
			logSelectionWarnings(warnings)
		}
		catalogue = dedupeCatalogue(append(catalogue, synthetic...))

		for _, album := range catalogue {
			if _, candidate := candidateSet[album.ForeignAlbumID]; candidate && album.Monitored {
				plan.Stats.AlreadyMonitored++
				log.Printf("  Already monitored: %s (%s)", album.Title, album.AlbumType)
			}
		}

		result := SelectAlbumsToMonitor(catalogue, SelectionOptions{
			Policy:                 opts.Policy,
			CandidateReleaseGroups: candidateSet,
		})
		plan.Stats.GroupsFiltered += len(result.Excluded)
		plan.Stats.GroupsCoverageSkipped += len(result.Skipped)
		for _, skipped := range result.Skipped {
			logSkippedAlbum(skipped)
		}
		for _, excluded := range result.Excluded {
			logExcludedAlbum(excluded, false)
		}
		logSelectionWarnings(result.Warnings)
		for _, album := range result.ToMonitor {
			candidate := resolved[album.ForeignAlbumID]
			switch {
			case candidate.lookup == nil:
				log.Printf(
					"  Selected existing album: %s (%s)",
					album.Title,
					album.AlbumType,
				)
			case !candidate.artistExists:
				log.Printf(
					"  Selected album to add: %s (%s) — missing artist will be added",
					album.Title,
					album.AlbumType,
				)
			default:
				log.Printf(
					"  Selected album to add: %s (%s)",
					album.Title,
					album.AlbumType,
				)
			}
			plan.Selected = append(plan.Selected, PlannedLabelAlbum{
				Album:        album,
				Lookup:       candidate.lookup,
				ArtistExists: candidate.artistExists,
			})
		}
	}

	return plan, nil
}

func (m *Monitor) RunLabels(opts LabelOptions) (*LabelStats, error) {
	dryRun := opts.DryRun || m.opts.DryRun
	var root lidarr.RootFolder
	if opts.MissingArtists.Enabled {
		roots, err := m.opts.Client.GetRootFolders()
		if err != nil {
			return nil, fmt.Errorf("failed to get Lidarr root folders: %w", err)
		}
		selected, err := selectRootFolder(roots, opts.MissingArtists.RootFolder)
		if err != nil {
			return nil, err
		}
		root = selected
	}

	plan, err := m.PlanLabels(opts)
	if err != nil {
		return nil, err
	}
	stats := plan.Stats
	var albumsToApply []common.Album
	createdArtists := make(map[string]lidarr.Artist)
	plannedArtists := make(map[string]struct{})
	plannedAdds := 0

	for _, planned := range plan.Selected {
		if planned.Lookup == nil {
			if planned.Album.ID > 0 {
				albumsToApply = append(albumsToApply, planned.Album)
			}
			continue
		}

		ownerID := plannedOwnerForeignID(planned)
		if !planned.ArtistExists && !opts.MissingArtists.Enabled {
			stats.Failures++
			log.Printf(
				"ERROR: Refusing to add %s because its artist is missing",
				planned.Album.Title,
			)
			continue
		}

		if dryRun {
			stats.AlbumsAdded++
			plannedAdds++
			if !planned.ArtistExists {
				if _, counted := plannedArtists[ownerID]; !counted {
					plannedArtists[ownerID] = struct{}{}
					stats.ArtistsAdded++
				}
			}
			continue
		}

		request := cloneAlbum(*planned.Lookup)
		request.Monitored = false
		request.AddOptions.SearchForNewAlbum = false
		if planned.ArtistExists {
			if request.Artist == nil {
				stats.Failures++
				log.Printf(
					"ERROR: Existing artist payload is missing for %s",
					planned.Album.Title,
				)
				continue
			}
			request.Artist.AddOptions = nil
		} else if created, ok := createdArtists[ownerID]; ok {
			created.AddOptions = nil
			request.Artist = &created
		} else {
			if request.Artist == nil {
				request.Artist = &lidarr.Artist{
					ArtistName: planned.Album.ArtistName,
					ForeignID:  ownerID,
				}
			}
			shapeMissingArtist(request.Artist, root)
		}

		created, err := m.opts.Client.AddAlbum(request)
		if err != nil {
			stats.Failures++
			log.Printf(
				"ERROR: Failed to add %s; Lidarr may have created its artist: %v",
				planned.Album.Title,
				err,
			)
			continue
		}
		if created == nil || created.ID <= 0 {
			stats.Failures++
			log.Printf("ERROR: Lidarr returned no album ID after adding %s", planned.Album.Title)
			continue
		}

		applied := planned.Album
		applied.ID = created.ID
		applied.ArtistID = created.ArtistID
		applied.Monitored = created.Monitored
		albumsToApply = append(albumsToApply, applied)
		stats.AlbumsAdded++

		if !planned.ArtistExists {
			if _, known := createdArtists[ownerID]; !known {
				artist := *request.Artist
				if created.Artist != nil {
					artist = *created.Artist
				}
				artist.AddOptions = nil
				createdArtists[ownerID] = artist
				stats.ArtistsAdded++
			}
		}
	}

	applyStats, err := applyAlbums(m.opts.Client, m.opts.State, dryRun, albumsToApply)
	if err != nil {
		return &stats, err
	}
	stats.AlbumsMonitored = applyStats.AlbumsMonitored
	stats.SearchesSubmitted = applyStats.SearchesSubmitted
	if dryRun {
		stats.AlbumsMonitored += plannedAdds
		stats.SearchesSubmitted += plannedAdds
	}
	return &stats, nil
}

func shapeMissingArtist(artist *lidarr.Artist, root lidarr.RootFolder) {
	artist.RootFolderPath = root.Path
	artist.QualityProfileID = root.DefaultQualityProfileID
	artist.MetadataProfileID = root.DefaultMetadataProfileID
	artist.Tags = append([]int(nil), root.DefaultTags...)
	artist.Monitored = false
	artist.MonitorNewItems = "none"
	artist.AddOptions = &lidarr.AddArtistOptions{
		Monitor:                "none",
		Monitored:              false,
		SearchForMissingAlbums: false,
	}
}

func plannedOwnerForeignID(planned PlannedLabelAlbum) string {
	if planned.Lookup != nil &&
		planned.Lookup.Artist != nil &&
		planned.Lookup.Artist.ForeignID != "" {
		return planned.Lookup.Artist.ForeignID
	}
	if len(planned.Album.ForeignArtistIDs) > 0 {
		return planned.Album.ForeignArtistIDs[0]
	}
	return ""
}

func cloneAlbum(album lidarr.Album) lidarr.Album {
	clone := album
	clone.SecondaryTypes = append([]string(nil), album.SecondaryTypes...)
	clone.Releases = append([]lidarr.Release(nil), album.Releases...)
	clone.Tracks = append([]lidarr.Track(nil), album.Tracks...)
	if album.Artist != nil {
		artist := *album.Artist
		artist.Tags = append([]int(nil), album.Artist.Tags...)
		if album.Artist.AddOptions != nil {
			addOptions := *album.Artist.AddOptions
			artist.AddOptions = &addOptions
		}
		clone.Artist = &artist
	}
	return clone
}

func (m *Monitor) PrintLabelSummary(stats *LabelStats, duration time.Duration) {
	fmt.Printf("\n=== LABEL MONITOR SUMMARY ===\n")
	fmt.Printf("Completed in %v\n", duration)
	fmt.Printf("Labels processed: %d\n", stats.LabelsProcessed)
	fmt.Printf("Releases discovered: %d\n", stats.ReleasesDiscovered)
	fmt.Printf("Release groups discovered: %d\n", stats.GroupsDiscovered)
	fmt.Printf("Release groups filtered: %d\n", stats.GroupsFiltered)
	fmt.Printf("Release groups skipped by coverage: %d\n", stats.GroupsCoverageSkipped)
	fmt.Printf("Already monitored: %d\n", stats.AlreadyMonitored)
	fmt.Printf("Missing artists skipped: %d\n", stats.MissingArtistSkipped)
	if m.opts.DryRun {
		fmt.Printf("Artists that would be added: %d\n", stats.ArtistsAdded)
		fmt.Printf("Albums that would be added: %d\n", stats.AlbumsAdded)
		fmt.Printf("Albums that would be monitored: %d\n", stats.AlbumsMonitored)
		fmt.Printf("Searches that would be submitted: %d\n", stats.SearchesSubmitted)
	} else {
		fmt.Printf("Artists added: %d\n", stats.ArtistsAdded)
		fmt.Printf("Albums added: %d\n", stats.AlbumsAdded)
		fmt.Printf("Albums monitored: %d\n", stats.AlbumsMonitored)
		fmt.Printf("Searches submitted: %d\n", stats.SearchesSubmitted)
	}
	fmt.Printf("Failures: %d\n\n", stats.Failures)
}

func (m *Monitor) discoverLabelGroups(
	labelIDs []string,
	stats *LabelStats,
) (map[string]musicbrainz.LabelReleaseGroup, []string) {
	groups := make(map[string]musicbrainz.LabelReleaseGroup)
	var groupOrder []string
	seenLabels := make(map[string]struct{}, len(labelIDs))
	for _, rawID := range labelIDs {
		labelID := strings.ToLower(strings.TrimSpace(rawID))
		if labelID == "" {
			continue
		}
		if _, duplicate := seenLabels[labelID]; duplicate {
			continue
		}
		seenLabels[labelID] = struct{}{}

		result, err := m.opts.MBClient.LabelReleaseGroups(labelID)
		if err != nil {
			stats.Failures++
			if errors.Is(err, musicbrainz.ErrLabelNotFound) {
				log.Printf("ERROR: MusicBrainz label %s was not found", labelID)
			} else {
				log.Printf("ERROR: Failed to browse MusicBrainz label %s: %v", labelID, err)
			}
			continue
		}
		stats.LabelsProcessed++
		stats.ReleasesDiscovered += result.ReleasesSeen
		for _, group := range result.Groups {
			if group.ID == "" {
				continue
			}
			if existing, ok := groups[group.ID]; ok {
				groups[group.ID] = mergeDiscoveredGroup(existing, group)
				continue
			}
			groups[group.ID] = group
			groupOrder = append(groupOrder, group.ID)
		}
	}
	return groups, groupOrder
}

func albumFromLabelGroup(group musicbrainz.LabelReleaseGroup) common.Album {
	album := common.Album{
		Title:          group.Title,
		AlbumType:      group.PrimaryType,
		SecondaryTypes: append([]string(nil), group.SecondaryTypes...),
		ForeignAlbumID: group.ID,
	}
	for _, credit := range group.ArtistCredits {
		if credit.ArtistID != "" {
			album.ForeignArtistIDs = append(album.ForeignArtistIDs, credit.ArtistID)
		}
		if album.ArtistName == "" {
			album.ArtistName = credit.Name
		}
	}
	for _, format := range group.Formats {
		album.Releases = append(album.Releases, common.Release{Format: format})
	}
	for _, track := range group.Tracks {
		album.Tracks = append(album.Tracks, common.Track{
			Title:              track.Title,
			ForeignTrackID:     track.ID,
			ForeignRecordingID: track.RecordingID,
		})
	}
	return album
}

func mergeLookup(album common.Album, lookup lidarr.Album) common.Album {
	album.ID = lookup.ID
	if lookup.Title != "" {
		album.Title = lookup.Title
	}
	if lookup.AlbumType != "" {
		album.AlbumType = lookup.AlbumType
	}
	if lookup.SecondaryTypes != nil {
		album.SecondaryTypes = append([]string(nil), lookup.SecondaryTypes...)
	}
	album.ArtistID = lookup.ArtistID
	album.Releases = lidarr.ConvertReleases(lookup.Releases)
	album.HasFiles = lookup.Statistics != nil && lookup.Statistics.TrackFileCount > 0
	album.Monitored = lookup.Monitored
	if lookup.Artist != nil {
		if lookup.Artist.ArtistName != "" {
			album.ArtistName = lookup.Artist.ArtistName
		}
		if lookup.Artist.ForeignID != "" &&
			!containsString(album.ForeignArtistIDs, lookup.Artist.ForeignID) {
			album.ForeignArtistIDs = append(album.ForeignArtistIDs, lookup.Artist.ForeignID)
		}
		album.IsVariousArtists = lookup.Artist.ForeignID == musicbrainz.VariousArtistsID
	}
	return album
}

func usableArtistCredits(group musicbrainz.LabelReleaseGroup) ([]string, bool) {
	seen := make(map[string]struct{})
	var ids []string
	complete := len(group.ArtistCredits) > 0
	for _, credit := range group.ArtistCredits {
		if credit.ArtistID == "" {
			complete = false
			continue
		}
		if _, duplicate := seen[credit.ArtistID]; duplicate {
			continue
		}
		seen[credit.ArtistID] = struct{}{}
		ids = append(ids, credit.ArtistID)
	}
	return ids, complete
}

func albumByForeignID(albums []lidarr.Album, id string) (lidarr.Album, bool) {
	for _, album := range albums {
		if album.ForeignAlbumID == id {
			return album, true
		}
	}
	return lidarr.Album{}, false
}

func ownerArtist(
	album lidarr.Album,
	fallback lidarr.Artist,
	artistsByID map[int]lidarr.Artist,
) lidarr.Artist {
	if artist, ok := artistsByID[album.ArtistID]; ok {
		return artist
	}
	return fallback
}

func exactLookup(albums []lidarr.Album, id string) (lidarr.Album, bool) {
	var exact lidarr.Album
	matches := 0
	for _, album := range albums {
		if album.ForeignAlbumID == id {
			exact = album
			matches++
		}
	}
	return exact, matches == 1
}

func lookupOwner(
	album lidarr.Album,
	creditIDs []string,
	artistsByID map[int]lidarr.Artist,
) (lidarr.Artist, string) {
	if artist, ok := artistsByID[album.ArtistID]; ok {
		return artist, artist.ForeignID
	}
	if album.Artist != nil {
		if artist, ok := artistsByID[album.Artist.ID]; ok {
			return artist, artist.ForeignID
		}
		if album.Artist.ForeignID != "" {
			return lidarr.Artist{}, album.Artist.ForeignID
		}
	}
	if len(creditIDs) == 1 {
		return lidarr.Artist{}, creditIDs[0]
	}
	return lidarr.Artist{}, ""
}

func containsForeignAlbum(albums []common.Album, id string) bool {
	for _, album := range albums {
		if album.ForeignAlbumID == id {
			return true
		}
	}
	return false
}

func dedupeCatalogue(albums []common.Album) []common.Album {
	index := make(map[string]int, len(albums))
	result := make([]common.Album, 0, len(albums))
	for _, album := range albums {
		if album.ForeignAlbumID == "" {
			result = append(result, album)
			continue
		}
		if existingIndex, ok := index[album.ForeignAlbumID]; ok {
			if result[existingIndex].ID == 0 && album.ID != 0 {
				result[existingIndex] = album
			}
			continue
		}
		index[album.ForeignAlbumID] = len(result)
		result = append(result, album)
	}
	return result
}

func hasDirectVACredit(album common.Album) bool {
	for _, artistID := range album.ForeignArtistIDs {
		if artistID == musicbrainz.VariousArtistsID {
			return true
		}
	}
	return false
}

func mergeDiscoveredGroup(
	existing musicbrainz.LabelReleaseGroup,
	incoming musicbrainz.LabelReleaseGroup,
) musicbrainz.LabelReleaseGroup {
	for _, secondaryType := range incoming.SecondaryTypes {
		if !containsString(existing.SecondaryTypes, secondaryType) {
			existing.SecondaryTypes = append(existing.SecondaryTypes, secondaryType)
		}
	}
	artistIDs := make(map[string]struct{}, len(existing.ArtistCredits))
	for _, credit := range existing.ArtistCredits {
		artistIDs[credit.ArtistID] = struct{}{}
	}
	for _, credit := range incoming.ArtistCredits {
		if _, exists := artistIDs[credit.ArtistID]; exists {
			continue
		}
		artistIDs[credit.ArtistID] = struct{}{}
		existing.ArtistCredits = append(existing.ArtistCredits, credit)
	}
	for _, format := range incoming.Formats {
		if !containsString(existing.Formats, format) {
			existing.Formats = append(existing.Formats, format)
		}
	}
	trackKeys := make(map[string]struct{}, len(existing.Tracks))
	for _, track := range existing.Tracks {
		trackKeys[labelTrackKey(track)] = struct{}{}
	}
	for _, track := range incoming.Tracks {
		key := labelTrackKey(track)
		if _, exists := trackKeys[key]; exists {
			continue
		}
		trackKeys[key] = struct{}{}
		existing.Tracks = append(existing.Tracks, track)
	}
	for _, label := range incoming.SourceLabels {
		if !containsString(existing.SourceLabels, label) {
			existing.SourceLabels = append(existing.SourceLabels, label)
		}
	}
	return existing
}

func labelTrackKey(track musicbrainz.Track) string {
	if track.RecordingID != "" {
		return "recording:" + track.RecordingID
	}
	if track.ID != "" {
		return "track:" + track.ID
	}
	return "title:" + strings.ToLower(strings.TrimSpace(track.Title))
}

func containsString(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}
