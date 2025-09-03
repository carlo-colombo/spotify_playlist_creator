package main

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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/spotify"
)

const spotifyAPIBaseURL = "https://api.spotify.com/v1"

type SpotifyClient struct {
	client *http.Client
	db     *Database
}

func setupSpotifyClient(ctx context.Context, db *Database) (*SpotifyClient, error) {
	config := &oauth2.Config{
		ClientID:     os.Getenv("SPOTIFY_ID"),
		ClientSecret: os.Getenv("SPOTIFY_SECRET"),
		RedirectURL:  "http://127.0.0.1:8888/callback",
		Scopes:       []string{"playlist-modify-private", "playlist-modify-public"},
		Endpoint:     spotify.Endpoint,
	}

	// Get token from cache or from web
	token, err := getToken(ctx, config)
	if err != nil {
		return nil, err
	}

	client := config.Client(ctx, token)
	return &SpotifyClient{client: client, db: db}, nil
}

func getToken(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	tokenCacheDir := fmt.Sprintf("%s/spotify_playlist_creator", cacheDir)
	if _, err := os.Stat(tokenCacheDir); os.IsNotExist(err) {
		os.MkdirAll(tokenCacheDir, 0700)
	}
	tokenFile := fmt.Sprintf("%s/token.json", tokenCacheDir)

	// Try to get token from cache
	if _, err := os.Stat(tokenFile); err == nil {
		file, err := os.Open(tokenFile)
		if err == nil {
			defer file.Close()
			var token oauth2.Token
			if err := json.NewDecoder(file).Decode(&token); err == nil {
				// Check if the token is expired
				if token.Valid() {
					return &token, nil
				}
			}
		}
	}

	// If token not in cache or expired, get it from web
	url := config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Printf("Your browser has been opened to visit the following page:\n%s\n", url)

	// Open browser
	// open.Run(url)

	// Wait for authorization code
	code := make(chan string)
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code <- r.URL.Query().Get("code")
		fmt.Fprintf(w, "You can close this window now.")
	})
	go http.ListenAndServe(":8888", nil)

	authCode := <-code

	token, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, err
	}

	// Cache the new token
	file, err := os.Create(tokenFile)
	if err == nil {
		defer file.Close()
		json.NewEncoder(file).Encode(token)
	}

	return token, nil
}

func (c *SpotifyClient) SearchTrack(ctx context.Context, title, artist, album string) (string, error) {
	cacheKey := fmt.Sprintf("spotify:track:%s:%s:%s", title, artist, album)
	if cached, found := c.db.GetCache(cacheKey); found {
		return cached, nil
	}

	query := fmt.Sprintf("track:%s artist:%s album:%s", cleanTrackTitle(title), artist, album)
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/search?q=%s&type=track", spotifyAPIBaseURL, url.QueryEscape(query)), nil)
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

	return "", nil // Not found
}

func (c *SpotifyClient) GetOrCreatePlaylist(ctx context.Context, name string) (string, error) {
	userID, err := c.getCurrentUserID(ctx)
	if err != nil {
		return "", err
	}

	// Check if playlist already exists
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/users/%s/playlists", spotifyAPIBaseURL, userID), nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var playlists struct {
		Items []SpotifyPlaylist `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&playlists); err != nil {
		return "", err
	}

	for _, p := range playlists.Items {
		if p.Name == name {
			return p.ID, nil
		}
	}

	// Create playlist
	createReqBody, _ := json.Marshal(map[string]interface{}{
		"name":        name,
		"public":      false,
		"description": "Playlist created by Spotify Playlist Creator",
	})

	req, err = http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/users/%s/playlists", spotifyAPIBaseURL, userID), bytes.NewBuffer(createReqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var newPlaylist SpotifyPlaylist
	if err := json.NewDecoder(resp.Body).Decode(&newPlaylist); err != nil {
		return "", err
	}

	return newPlaylist.ID, nil
}

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
		return nil // No new tracks to add
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

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/playlists/%s/tracks", spotifyAPIBaseURL, playlistID), bytes.NewBuffer(reqBody))
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
		req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/playlists/%s/tracks?limit=%d&offset=%d", spotifyAPIBaseURL, playlistID, limit, offset), nil)
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
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/me", spotifyAPIBaseURL), nil)
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

func (c *SpotifyClient) GetTrackDetails(ctx context.Context, trackURI string) (*SpotifyTrack, error) {
	trackID := strings.TrimPrefix(trackURI, "spotify:track:")
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/tracks/%s", spotifyAPIBaseURL, trackID), nil)
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
