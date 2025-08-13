package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const musicBrainzAPIBaseURL = "https://musicbrainz.org/ws/2"

type MusicBrainzClient struct {
	client *http.Client
	db     *Database
}

func setupMusicBrainzClient(db *Database) *MusicBrainzClient {
	return &MusicBrainzClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		db: db,
	}
}

func (c *MusicBrainzClient) GetArtistID(artistName string) (string, error) {
	// Handle disambiguation for specific artists
	if artistName == "wargasm" {
		artistName = "WARGASM (UK)"
	}

	cacheKey := fmt.Sprintf("musicbrainz:artist:%s", artistName)
	if cached, found := c.db.GetCache(cacheKey); found {
		return cached, nil
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/artist?query=%s&fmt=json", musicBrainzAPIBaseURL, url.QueryEscape(artistName)), nil)
	if err != nil {
		return "", err
	}

	resp, err := c.doRequestWithRateLimit(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var artists struct {
		Artists []MusicBrainzArtist `json:"artists"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&artists); err != nil {
		return "", err
	}

	if len(artists.Artists) > 0 {
		artistID := artists.Artists[0].ID
		c.db.SetCache(cacheKey, artistID, 3600*24*30) // Cache for 30 days
		return artistID, nil
	}

	return "", fmt.Errorf("artist not found: %s", artistName)
}

func (c *MusicBrainzClient) GetLatestReleases(artistID string) ([]Release, error) {
	cacheKey := fmt.Sprintf("musicbrainz:releases:%s", artistID)
	if cached, found := c.db.GetCache(cacheKey); found {
		var releases []Release
		if err := json.Unmarshal([]byte(cached), &releases); err == nil {
			return releases, nil
		}
	}

	// Get all releases
	releaseGroups, err := c.getAllReleases(artistID)
	if err != nil {
		return nil, err
	}

	// Filter for latest studio album and subsequent singles
	latestAlbum, subsequentSingles := c.filterReleases(releaseGroups)

	var releasesToFetch []Release
	if latestAlbum != nil {
		releasesToFetch = append(releasesToFetch, *latestAlbum)
	}
	releasesToFetch = append(releasesToFetch, subsequentSingles...)

	// Use a map to track all added tracks across different releases
	allAddedTracks := make(map[string]bool)

	// Get track titles for each release
	for i := range releasesToFetch {
		trackTitles, err := c.getTrackTitles(releasesToFetch[i].ID, allAddedTracks)
		if err != nil {
			log.Printf("Error getting track titles for release %s: %v", releasesToFetch[i].Title, err)
			continue
		}
		releasesToFetch[i].TrackTitles = trackTitles
	}

	return releasesToFetch, nil
}

func (c *MusicBrainzClient) getAllReleases(artistID string) ([]MusicBrainzReleaseGroup, error) {
	var allReleases []MusicBrainzReleaseGroup
	offset := 0
	limit := 100

	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/release-group?artist=%s&fmt=json&limit=%d&offset=%d", musicBrainzAPIBaseURL, artistID, limit, offset), nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.doRequestWithRateLimit(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var releases struct {
			ReleaseGroups []MusicBrainzReleaseGroup `json:"release-groups"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
			return nil, err
		}

		allReleases = append(allReleases, releases.ReleaseGroups...)

		if len(releases.ReleaseGroups) < limit {
			break
		}
		offset += limit
	}

	return allReleases, nil
}

func (c *MusicBrainzClient) filterReleases(releases []MusicBrainzReleaseGroup) (*Release, []Release) {
	var studioAlbums []MusicBrainzReleaseGroup
	var singles []MusicBrainzReleaseGroup

	for _, r := range releases {
		if r.PrimaryType == "Album" && !isLiveOrRemix(r.Title) {
			studioAlbums = append(studioAlbums, r)
		} else if r.PrimaryType == "Single" && !isLiveOrRemix(r.Title) {
			singles = append(singles, r)
		}
	}

	// Sort albums by date
	sort.Slice(studioAlbums, func(i, j int) bool {
		return studioAlbums[i].FirstReleaseDate > studioAlbums[j].FirstReleaseDate
	})

	var latestAlbum *Release
	if len(studioAlbums) > 0 {
		albumDate, _ := time.Parse("2006-01-02", studioAlbums[0].FirstReleaseDate)
		latestAlbum = &Release{ID: studioAlbums[0].ID, Title: studioAlbums[0].Title, Date: albumDate}
	}

	var subsequentSingles []Release
	if latestAlbum != nil {
		for _, s := range singles {
			singleDate, _ := time.Parse("2006-01-02", s.FirstReleaseDate)
			if singleDate.After(latestAlbum.Date) {
				subsequentSingles = append(subsequentSingles, Release{ID: s.ID, Title: s.Title, Date: singleDate})
			}
		}
	}

	return latestAlbum, subsequentSingles
}

func isLiveOrRemix(title string) bool {
	lowerTitle := strings.ToLower(title)
	return strings.Contains(lowerTitle, "live") ||
		strings.Contains(lowerTitle, "remix") ||
		strings.Contains(lowerTitle, "acoustic") ||
		strings.Contains(lowerTitle, "version")
}

func (c *MusicBrainzClient) getTrackTitles(releaseGroupID string, allAddedTracks map[string]bool) ([]string, error) {
	cacheKey := fmt.Sprintf("musicbrainz:tracks:%s", releaseGroupID)
	if cached, found := c.db.GetCache(cacheKey); found {
		var trackTitles []string
		if err := json.Unmarshal([]byte(cached), &trackTitles); err == nil {
			return trackTitles, nil
		}
	}

	// Get releases in the release group
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/release?release-group=%s&fmt=json", musicBrainzAPIBaseURL, releaseGroupID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequestWithRateLimit(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var releaseGroupReleases struct {
		Releases []MusicBrainzRelease `json:"releases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&releaseGroupReleases); err != nil {
		return nil, err
	}

	// Sort releases by date (newest first)
	sort.Slice(releaseGroupReleases.Releases, func(i, j int) bool {
		return releaseGroupReleases.Releases[i].Date > releaseGroupReleases.Releases[j].Date
	})

	var currentReleaseTrackTitles []string

	for _, release := range releaseGroupReleases.Releases {
		// Get tracks for each release
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/release/%s?inc=recordings&fmt=json", musicBrainzAPIBaseURL, release.ID), nil)
		if err != nil {
			log.Printf("Error creating request for release %s: %v", release.ID, err)
			continue
		}

		resp, err := c.doRequestWithRateLimit(req)
		if err != nil {
			log.Printf("Error getting tracks for release %s: %v", release.ID, err)
			continue
		}
		defer resp.Body.Close()

		var releaseWithTracks struct {
			Media []struct {
				Tracks []MusicBrainzTrack `json:"tracks"`
			} `json:"media"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&releaseWithTracks); err != nil {
			log.Printf("Error decoding tracks for release %s: %v", release.ID, err)
			continue
		}

		for _, media := range releaseWithTracks.Media {
			for _, track := range media.Tracks {
				if _, ok := allAddedTracks[track.Title]; !ok {
					currentReleaseTrackTitles = append(currentReleaseTrackTitles, track.Title)
					allAddedTracks[track.Title] = true
				}
			}
		}
	}

	jsonData, err := json.Marshal(currentReleaseTrackTitles)
	if err == nil {
		c.db.SetCache(cacheKey, string(jsonData), 3600*24*7) // Cache for 1 week
	}

	return currentReleaseTrackTitles, nil
}

func (c *MusicBrainzClient) doRequestWithRateLimit(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	backoff := 1 * time.Second

	for i := 0; i < 5; i++ {
		req.Header.Set("User-Agent", "SpotifyPlaylistCreator/1.0 ( your-email@example.com )")
		resp, err = c.client.Do(req)
		if err == nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound) {
			return resp, nil
		}

		if resp != nil && (resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests) {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		return nil, fmt.Errorf("MusicBrainz API request failed with status %d", resp.StatusCode)
	}

	return nil, fmt.Errorf("MusicBrainz API request failed after multiple retries")
}
