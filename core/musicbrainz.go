package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const musicBrainzAPIBaseURL = "https://musicbrainz.org/ws/2"

// MusicBrainzClient wraps MusicBrainz API interactions
type MusicBrainzClient struct {
	client *http.Client
	db     *Database
}

// NewMusicBrainzClient creates a new MusicBrainz client
func NewMusicBrainzClient(db *Database) *MusicBrainzClient {
	return &MusicBrainzClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		db: db,
	}
}

// GetArtistID retrieves the MusicBrainz artist ID
func (c *MusicBrainzClient) GetArtistID(artistName string) (string, error) {
	// Handle disambiguation for specific artists
	if strings.ToLower(artistName) == "wargasm" {
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

// GetArtistIDAndName retrieves the MusicBrainz artist ID and canonical name
func (c *MusicBrainzClient) GetArtistIDAndName(artistName string) (string, string, error) {
	// Handle disambiguation for specific artists
	if strings.ToLower(artistName) == "wargasm" {
		artistName = "WARGASM (UK)"
	}

	cacheKey := fmt.Sprintf("musicbrainz:artist:%s", artistName)
	if cached, found := c.db.GetCache(cacheKey); found {
		return cached, "", nil
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/artist?query=%s&fmt=json", musicBrainzAPIBaseURL, url.QueryEscape(artistName)), nil)
	if err != nil {
		return "", "", err
	}

	resp, err := c.doRequestWithRateLimit(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var artists struct {
		Artists []MusicBrainzArtist `json:"artists"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&artists); err != nil {
		return "", "", err
	}

	if len(artists.Artists) > 0 {
		artistID := artists.Artists[0].ID
		canonicalName := artists.Artists[0].Name
		c.db.SetCache(cacheKey, artistID, 3600*24*30) // Cache for 30 days
		return artistID, canonicalName, nil
	}

	return "", "", fmt.Errorf("artist not found: %s", artistName)
}

// GetLatestReleases retrieves the latest releases for an artist
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

	// Cache the releases
	releasesJSON, _ := json.Marshal(releasesToFetch)
	c.db.SetCache(cacheKey, string(releasesJSON), 3600*24*7)

	return releasesToFetch, nil
}

func (c *MusicBrainzClient) getAllReleases(artistID string) ([]MusicBrainzReleaseGroup, error) {
	var allReleases []MusicBrainzReleaseGroup
	offset := 0
	limit := 100

	for {
		trackUrl := fmt.Sprintf("%s/release-group?artist=%s&fmt=json&limit=%d&offset=%d", musicBrainzAPIBaseURL, artistID, limit, offset)
		req, err := http.NewRequest("GET", trackUrl, nil)
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
	now := time.Now()

	var studioAlbums []MusicBrainzReleaseGroup
	var singles []MusicBrainzReleaseGroup

	for _, r := range releases {
		releaseDate, err := time.Parse("2006-01-02", r.FirstReleaseDate)
		if err != nil {
			if len(r.FirstReleaseDate) == 4 {
				releaseDate, err = time.Parse("2006", r.FirstReleaseDate)
			}
			if err != nil {
				continue
			}
		}
		if releaseDate.After(now) {
			continue
		}

		isStudioAlbum := r.PrimaryType == "Album" && !contains(r.SecondaryTypes, "Live") && !contains(r.SecondaryTypes, "Compilation")
		isSingle := r.PrimaryType == "Single"

		if isStudioAlbum {
			studioAlbums = append(studioAlbums, r)
		} else if isSingle && !isLiveOrRemix(r.Title) {
			singles = append(singles, r)
		}
	}

	// Sort by date descending
	sortByDateDesc(studioAlbums)
	sortByDateDesc(singles)

	if len(studioAlbums) == 0 {
		return nil, nil
	}

	latestAlbum := studioAlbums[0]
	latestAlbumDate, _ := time.Parse("2006-01-02", latestAlbum.FirstReleaseDate)

	var subsequentSingles []Release
	for _, s := range singles {
		singleDate, err := time.Parse("2006-01-02", s.FirstReleaseDate)
		if err != nil {
			continue
		}
		if singleDate.After(latestAlbumDate) {
			subsequentSingles = append(subsequentSingles, Release{
				ID:          s.ID,
				Title:       s.Title,
				Date:        singleDate,
				TrackTitles: nil,
			})
		}
	}

	return &Release{
		ID:          latestAlbum.ID,
		Title:       latestAlbum.Title,
		Date:        latestAlbumDate,
		TrackTitles: nil,
	}, subsequentSingles
}

func (c *MusicBrainzClient) getTrackTitles(releaseID string, addedTracks map[string]bool) ([]string, error) {
	cacheKey := fmt.Sprintf("musicbrainz:tracks:%s", releaseID)
	if cached, found := c.db.GetCache(cacheKey); found {
		var titles []string
		if err := json.Unmarshal([]byte(cached), &titles); err == nil {
			return titles, nil
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/release/%s?fmt=json&inc=tracks", musicBrainzAPIBaseURL, releaseID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequestWithRateLimit(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var release struct {
		Tracks []MusicBrainzTrack `json:"tracks"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	var titles []string
	for _, track := range release.Tracks {
		title := strings.TrimSpace(track.Title)
		if title != "" && !addedTracks[strings.ToLower(title)] {
			addedTracks[strings.ToLower(title)] = true
			titles = append(titles, title)
		}
	}

	// Cache the track titles
	titlesJSON, _ := json.Marshal(titles)
	c.db.SetCache(cacheKey, string(titlesJSON), 3600*24*7)

	return titles, nil
}

func (c *MusicBrainzClient) doRequestWithRateLimit(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "SpotifyPlaylistCreator/1.0 (contact@example.com)")

	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 503 || resp.StatusCode == 429 {
			resp.Body.Close()
			// Wait before retrying
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func isLiveOrRemix(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, "live") || strings.Contains(lower, "remix") || strings.Contains(lower, "remaster")
}

func sortByDateDesc(releases []MusicBrainzReleaseGroup) {
	for i := 0; i < len(releases); i++ {
		for j := i + 1; j < len(releases); j++ {
			dateI, _ := time.Parse("2006-01-02", releases[i].FirstReleaseDate)
			dateJ, _ := time.Parse("2006-01-02", releases[j].FirstReleaseDate)
			if dateJ.After(dateI) {
				releases[i], releases[j] = releases[j], releases[i]
			}
		}
	}
}
