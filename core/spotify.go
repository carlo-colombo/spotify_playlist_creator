package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/spotify"
)

const SpotifyAPIBaseURL = "https://api.spotify.com/v1"

// SpotifyClient wraps Spotify API interactions
type SpotifyClient struct {
	client        *http.Client
	db            *Database
	sessionID     string
	authenticated bool
}

// NewSpotifyClient creates a new Spotify client for a session
func NewSpotifyClient(db *Database, sessionID string) *SpotifyClient {
	return &SpotifyClient{
		db:        db,
		sessionID: sessionID,
	}
}

// IsAuthenticated returns whether the client has a valid token
func (c *SpotifyClient) IsAuthenticated() bool {
	return c.authenticated && c.client != nil
}

// EnsureAuthenticated checks and loads token if needed
func (c *SpotifyClient) EnsureAuthenticated(ctx context.Context) error {
	if c.client != nil && c.authenticated {
		return nil
	}

	// Load token from database
	tokenJSON, found := c.db.GetCache(fmt.Sprintf("spotify:token:%s", c.sessionID))
	if !found {
		return fmt.Errorf("not authenticated: no token found for session")
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokenJSON), &token); err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}

	// Check if token is expired and refresh if needed
	if !token.Valid() && token.RefreshToken != "" {
		var err error
		newToken, err := c.refreshToken(ctx, &token)
		if err != nil {
			return fmt.Errorf("failed to refresh token: %w", err)
		}
		token = *newToken
		// Store refreshed token
		tokenJSON, _ := json.Marshal(token)
		c.db.SetCache(fmt.Sprintf("spotify:token:%s", c.sessionID), string(tokenJSON), int64(token.Expiry.Sub(time.Now()).Seconds()))
	}

	config := &oauth2.Config{
		ClientID:     os.Getenv("SPOTIFY_ID"),
		ClientSecret: os.Getenv("SPOTIFY_SECRET"),
		Endpoint:     spotify.Endpoint,
	}

	c.client = config.Client(ctx, &token)
	c.authenticated = true
	return nil
}

func (c *SpotifyClient) refreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error) {
	config := &oauth2.Config{
		ClientID:     os.Getenv("SPOTIFY_ID"),
		ClientSecret: os.Getenv("SPOTIFY_SECRET"),
		Endpoint:     spotify.Endpoint,
	}

	source := config.TokenSource(ctx, token)
	newToken, err := source.Token()
	if err != nil {
		return nil, err
	}
	return newToken, nil
}

// SearchTrack searches for a track on Spotify
func (c *SpotifyClient) SearchTrack(ctx context.Context, title, artist, album string) (string, error) {
	cacheKey := fmt.Sprintf("spotify:track:%s:%s:%s", title, artist, album)
	if cached, found := c.db.GetCache(cacheKey); found {
		return cached, nil
	}

	query := fmt.Sprintf("track:%s artist:%s album:%s", cleanTrackTitle(title), artist, album)

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/search?q=%s&type=track", SpotifyAPIBaseURL, url.QueryEscape(query)), nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("spotify search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResult struct {
		Tracks struct {
			Items []SpotifyTrack `json:"items"`
		} `json:"tracks"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return "", err
	}

	if len(searchResult.Tracks.Items) > 0 {
		trackURI := searchResult.Tracks.Items[0].URI
		c.db.SetCache(cacheKey, trackURI, 3600*24*7) // Cache for 1 week
		return trackURI, nil
	}

	return "", nil
}

// GetOrCreatePlaylist gets or creates a playlist
func (c *SpotifyClient) GetOrCreatePlaylist(ctx context.Context, name string, skipCreation bool) (*SpotifyPlaylist, error) {
	userID, err := c.getCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}

	// Check if playlist already exists
	var allPlaylists []SpotifyPlaylist
	nextURL := fmt.Sprintf("%s/users/%s/playlists", SpotifyAPIBaseURL, userID)

	var req *http.Request
	var resp *http.Response
	for nextURL != "" {
		req, err = http.NewRequestWithContext(ctx, "GET", nextURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err = c.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var playlists struct {
			Items []SpotifyPlaylist `json:"items"`
			Next  string            `json:"next"`
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("spotify get playlists failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		}

		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&playlists); err != nil {
			return nil, err
		}

		allPlaylists = append(allPlaylists, playlists.Items...)
		nextURL = playlists.Next
	}

	for _, p := range allPlaylists {
		if p.Name == name {
			return &p, nil
		}
	}

	if skipCreation {
		return nil, fmt.Errorf("playlist '%s' not found and creation was skipped", name)
	}

	// Create playlist
	createReqBody, _ := json.Marshal(map[string]interface{}{
		"name":        name,
		"public":      false,
		"description": "Playlist created by Spotify Playlist Creator",
	})

	req, err = http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/users/%s/playlists", SpotifyAPIBaseURL, userID), bytes.NewBuffer(createReqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	creationResp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer creationResp.Body.Close()

	var newPlaylist SpotifyPlaylist
	if err := json.NewDecoder(creationResp.Body).Decode(&newPlaylist); err != nil {
		return nil, err
	}

	return &newPlaylist, nil
}

// AddTracksToPlaylist adds tracks to a playlist
func (c *SpotifyClient) AddTracksToPlaylist(ctx context.Context, playlistID string, trackURIs []string) error {
	existingTracks, err := c.getPlaylistTracks(ctx, playlistID)
	if err != nil {
		return fmt.Errorf("failed to get existing playlist tracks: %w", err)
	}

	tracksToAdd := []string{}
	for _, newTrack := range trackURIs {
		found := false
		for _, existingTrack := range existingTracks {
			if newTrack == existingTrack {
				found = true
				break
			}
		}
		if !found {
			tracksToAdd = append(tracksToAdd, newTrack)
		}
	}

	if len(tracksToAdd) == 0 {
		return nil
	}

	// Batch track additions (100 tracks per request)
	for i := 0; i < len(tracksToAdd); i += 100 {
		end := i + 100
		if end > len(tracksToAdd) {
			end = len(tracksToAdd)
		}
		batch := tracksToAdd[i:end]

		reqBody, _ := json.Marshal(map[string]interface{}{
			"uris": batch,
		})

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/playlists/%s/tracks", SpotifyAPIBaseURL, playlistID), bytes.NewBuffer(reqBody))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("failed to add tracks to playlist, status code: %d", resp.StatusCode)
		}
	}
	return nil
}

func (c *SpotifyClient) getPlaylistTracks(ctx context.Context, playlistID string) ([]string, error) {
	var allTrackURIs []string
	offset := 0
	limit := 100

	for {
		req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/playlists/%s/tracks?limit=%d&offset=%d", SpotifyAPIBaseURL, playlistID, limit, offset), nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var playlistTracks struct {
			Items []struct {
				Track SpotifyTrack `json:"track"`
			} `json:"items"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&playlistTracks); err != nil {
			return nil, err
		}

		for _, item := range playlistTracks.Items {
			allTrackURIs = append(allTrackURIs, item.Track.URI)
		}

		if len(playlistTracks.Items) < limit {
			break
		}
		offset += limit
	}

	return allTrackURIs, nil
}

func (c *SpotifyClient) getCurrentUserID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/me", SpotifyAPIBaseURL), nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var user struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}

	return user.ID, nil
}

// GetTrackDetails retrieves full track metadata
func (c *SpotifyClient) GetTrackDetails(ctx context.Context, trackURI string) (*SpotifyTrack, error) {
	trackID := strings.TrimPrefix(trackURI, "spotify:track:")
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/tracks/%s", SpotifyAPIBaseURL, trackID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("spotify get track details failed with status %d: %s", resp.StatusCode, string(body))
	}

	var track SpotifyTrack
	if err := json.NewDecoder(resp.Body).Decode(&track); err != nil {
		return nil, err
	}

	return &track, nil
}

func cleanTrackTitle(title string) string {
	// Remove featuring artists
	if idx := strings.Index(title, " feat."); idx != -1 {
		title = title[:idx]
	}
	if idx := strings.Index(title, " ft."); idx != -1 {
		title = title[:idx]
	}
	if idx := strings.Index(title, " featuring"); idx != -1 {
		title = title[:idx]
	}

	// Remove remix indicators
	title = strings.ReplaceAll(title, " (Remix)", "")
	title = strings.ReplaceAll(title, " (Radio Edit)", "")
	title = strings.ReplaceAll(title, " - Remastered", "")
	title = strings.ReplaceAll(title, " (Live)", "")

	// Normalize various delimiters to spaces
	title = strings.ReplaceAll(title, "/", " ")
	title = strings.ReplaceAll(title, "-", " ")

	return strings.TrimSpace(title)
}
