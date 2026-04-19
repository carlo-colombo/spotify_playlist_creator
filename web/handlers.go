package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"spotify_playlist_creator/core"
)

// Session holds per-session data
type Session struct {
	ID            string
	Artists       []string
	Releases      map[string][]core.Release  // artist -> releases
	Songs         []core.SongWithRelease
	PlaylistURL   string
	PlaylistName  string
	SpotifyClient *core.SpotifyClient
	MusicBrainz   *core.MusicBrainzClient
	CreatedAt     time.Time
}

// SessionStore manages sessions
type SessionStore struct {
	sessions map[string]*Session
	db       *core.Database
}

func NewSessionStore(db *core.Database) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
		db:       db,
	}
}

func (s *SessionStore) GetOrCreate(sessionID string) *Session {
	if session, exists := s.sessions[sessionID]; exists {
		return session
	}
	session := &Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
	}
	s.sessions[sessionID] = session
	return session
}

func (s *SessionStore) Get(sessionID string) (*Session, bool) {
	session, exists := s.sessions[sessionID]
	return session, exists
}

func (s *SessionStore) Delete(sessionID string) {
	delete(s.sessions, sessionID)
}

// Handlers holds all HTTP handlers
type Handlers struct {
	db          *core.Database
	sessions    *SessionStore
	spotifyConf *SpotifyConfig
	templates   *template.Template
}

// SpotifyConfig holds OAuth configuration
type SpotifyConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

func NewHandlers(db *core.Database, sessions *SessionStore) *Handlers {
	templates := template.Must(template.New("").Parse(IndexHTML))
	
	return &Handlers{
		db:        db,
		sessions:  sessions,
		templates: templates,
		spotifyConf: &SpotifyConfig{
			ClientID:     getEnv("SPOTIFY_ID", ""),
			ClientSecret: getEnv("SPOTIFY_SECRET", ""),
			RedirectURL:  getEnv("SPOTIFY_REDIRECT_URL", "http://127.0.0.1:8080/auth/callback"),
			Scopes:       []string{"playlist-modify-private", "playlist-modify-public", "playlist-read-private", "playlist-read-collaborative"},
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Home serves the main page
func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getOrCreateSessionID(w, r)
	session := h.sessions.GetOrCreate(sessionID)

	// Initialize clients if not already done
	if session.SpotifyClient == nil {
		session.SpotifyClient = core.NewSpotifyClient(h.db, sessionID)
	}
	if session.MusicBrainz == nil {
		session.MusicBrainz = core.NewMusicBrainzClient(h.db)
	}

	// Check if authenticated by verifying token exists in cache
	isAuthenticated := false
	tokenJSON, found := h.db.GetCache(fmt.Sprintf("spotify:token:%s", sessionID))
	if found && tokenJSON != "" {
		isAuthenticated = true
	}

	// Get cached artists (history)
	var cachedArtists []string
	artistsJSON, found := h.db.GetCache("artists:history")
	if found {
		json.Unmarshal([]byte(artistsJSON), &cachedArtists)
	}

	data := struct {
		SessionID        string
		IsAuthenticated  bool
		Artists          []string
		CachedArtists    []string
		Releases         map[string][]core.Release
		Songs            []core.SongWithRelease
		PlaylistName     string
		PlaylistURL      string
	}{
		SessionID:        sessionID,
		IsAuthenticated:  isAuthenticated,
		Artists:          session.Artists,
		CachedArtists:    cachedArtists,
		Releases:         session.Releases,
		Songs:            session.Songs,
		PlaylistName:     session.PlaylistName,
		PlaylistURL:      session.PlaylistURL,
	}

	h.templates.Execute(w, data)
}

// AuthSpotify initiates OAuth flow
func (h *Handlers) AuthSpotify(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	if sessionID == "" {
		sessionID = h.getOrCreateSessionID(w, r)
	}

	// Store session ID in state
	state := fmt.Sprintf("%s|%s", sessionID, generateRandomState())
	h.db.SetCache("oauth:state:"+state, sessionID, 300) // 5 min TTL

	authURL := fmt.Sprintf("https://accounts.spotify.com/authorize?client_id=%s&response_type=code&redirect_uri=%s&scope=%s&state=%s",
		h.spotifyConf.ClientID,
		url.QueryEscape(h.spotifyConf.RedirectURL),
		url.QueryEscape(strings.Join(h.spotifyConf.Scopes, " ")),
		url.QueryEscape(state),
	)

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// AuthCallback handles OAuth callback
func (h *Handlers) AuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	// Verify state - the full state string (sessionID|random) should be the key
	storedSession, found := h.db.GetCache("oauth:state:" + state)
	if !found {
		http.Error(w, "Invalid state token", http.StatusBadRequest)
		return
	}

	// Extract session ID from state for later use
	parts := strings.Split(state, "|")
	if len(parts) != 2 {
		http.Error(w, "Invalid state format", http.StatusBadRequest)
		return
	}
	sessionID := parts[0]

	// Verify the stored session matches
	if storedSession != sessionID {
		http.Error(w, "Invalid state token", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := h.exchangeCodeForToken(code, sessionID)
	if err != nil {
		log.Printf("Token exchange error: %v", err)
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}

	// Store token in database for this session
	tokenJSON, _ := json.Marshal(token)
	h.db.SetCache(fmt.Sprintf("spotify:token:%s", sessionID), string(tokenJSON), int64(token.Expiry.Sub(time.Now()).Seconds()))

	// Redirect to home
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (h *Handlers) exchangeCodeForToken(code, sessionID string) (*oauth2.Token, error) {
	// Spotify token endpoint
	data := fmt.Sprintf("grant_type=authorization_code&code=%s&redirect_uri=%s",
		code,
		url.QueryEscape(h.spotifyConf.RedirectURL),
	)

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(h.spotifyConf.ClientID, h.spotifyConf.ClientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &oauth2.Token{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

// APIArtistsList returns list of tracked artists
func (h *Handlers) APIArtistsList(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	session, _ := h.sessions.Get(sessionID)
	if session == nil {
		json.NewEncoder(w).Encode([]string{})
		return
	}

	json.NewEncoder(w).Encode(session.Artists)
}

// APIArtistsAdd adds an artist to track
func (h *Handlers) APIArtistsAdd(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	var req struct {
		Artist string `json:"artist"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	session := h.sessions.GetOrCreate(sessionID)

	// Initialize clients if needed
	if session.SpotifyClient == nil {
		session.SpotifyClient = core.NewSpotifyClient(h.db, sessionID)
	}
	if session.MusicBrainz == nil {
		session.MusicBrainz = core.NewMusicBrainzClient(h.db)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Process artist
	result, err := core.ProcessArtist(ctx, req.Artist, session.SpotifyClient, session.MusicBrainz)
	if err != nil {
		log.Printf("Error processing artist %s: %v", req.Artist, err)
		http.Error(w, fmt.Sprintf("Error processing artist: %v", err), http.StatusInternalServerError)
		return
	}

	// Initialize releases map if needed
	if session.Releases == nil {
		session.Releases = make(map[string][]core.Release)
	}

	// Add to session
	session.Artists = append(session.Artists, req.Artist)
	session.Releases[req.Artist] = result.Releases
	session.Songs = append(session.Songs, result.Songs...)

	// Deduplicate songs
	session.Songs = deduplicateSongs(session.Songs)

	// Update playlist name
	sort.Strings(session.Artists)
	session.PlaylistName = fmt.Sprintf("spc %s", strings.Join(session.Artists, ","))

	// Update cached artists history
	h.updateArtistsHistory(req.Artist)

	json.NewEncoder(w).Encode(struct {
		Artist string `json:"artist"`
		Songs  int    `json:"songs_count"`
	}{Artist: req.Artist, Songs: len(result.Songs)})
}

// APIArtistsRemove removes an artist from tracking
func (h *Handlers) APIArtistsRemove(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	artist := r.URL.Query().Get("artist")
	if artist == "" {
		http.Error(w, "Missing artist", http.StatusBadRequest)
		return
	}

	session, _ := h.sessions.Get(sessionID)
	if session == nil {
		json.NewEncoder(w).Encode(struct{ Success bool }{true})
		return
	}

	// Remove artist, their releases and songs
	var newSongs []core.SongWithRelease
	for _, song := range session.Songs {
		if !strings.EqualFold(song.Artist, artist) {
			newSongs = append(newSongs, song)
		}
	}
	session.Songs = newSongs

	// Remove releases for this artist
	if session.Releases != nil {
		delete(session.Releases, artist)
	}

	var newArtists []string
	for _, a := range session.Artists {
		if !strings.EqualFold(a, artist) {
			newArtists = append(newArtists, a)
		}
	}
	session.Artists = newArtists

	// Update playlist name
	if len(session.Artists) > 0 {
		sort.Strings(session.Artists)
		session.PlaylistName = fmt.Sprintf("spc %s", strings.Join(session.Artists, ","))
	} else {
		session.PlaylistName = ""
	}

	json.NewEncoder(w).Encode(struct{ Success bool }{true})
}

// APISongsRemove removes a song from selection
func (h *Handlers) APISongsRemove(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	var req struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	session, _ := h.sessions.Get(sessionID)
	if session == nil {
		json.NewEncoder(w).Encode(struct{ Success bool }{false})
		return
	}

	// Remove song
	var newSongs []core.SongWithRelease
	for _, song := range session.Songs {
		if !(strings.EqualFold(song.Title, req.Title) && strings.EqualFold(song.Artist, req.Artist)) {
			newSongs = append(newSongs, song)
		}
	}
	session.Songs = newSongs

	json.NewEncoder(w).Encode(struct{ Success bool }{true})
}

// APIPlaylistCreate creates the playlist
func (h *Handlers) APIPlaylistCreate(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	session, _ := h.sessions.Get(sessionID)
	if session == nil || session.SpotifyClient == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	if len(session.Songs) == 0 {
		http.Error(w, "No songs to add", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Extract URIs
	var uris []string
	for _, song := range session.Songs {
		uris = append(uris, song.URI)
	}

	// Create or get playlist
	playlist, err := session.SpotifyClient.GetOrCreatePlaylist(ctx, session.PlaylistName, false)
	if err != nil {
		log.Printf("Error creating playlist: %v", err)
		http.Error(w, fmt.Sprintf("Error creating playlist: %v", err), http.StatusInternalServerError)
		return
	}

	// Add tracks
	if err := session.SpotifyClient.AddTracksToPlaylist(ctx, playlist.ID, uris); err != nil {
		log.Printf("Error adding tracks: %v", err)
		http.Error(w, fmt.Sprintf("Error adding tracks: %v", err), http.StatusInternalServerError)
		return
	}

	session.PlaylistURL = playlist.ExternalURLs.Spotify

	json.NewEncoder(w).Encode(struct {
		PlaylistURL  string `json:"playlist_url"`
		PlaylistName string `json:"playlist_name"`
	}{
		PlaylistURL:  session.PlaylistURL,
		PlaylistName: session.PlaylistName,
	})
}

// APIPlaylistStatus returns current playlist status
func (h *Handlers) APIPlaylistStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	if sessionID == "" {
		json.NewEncoder(w).Encode(struct {
			HasSongs        bool   `json:"has_songs"`
			PlaylistName    string `json:"playlist_name"`
			PlaylistURL     string `json:"playlist_url"`
			SongsCount      int    `json:"songs_count"`
			IsAuthenticated bool   `json:"is_authenticated"`
		}{
			HasSongs: false,
		})
		return
	}

	session, _ := h.sessions.Get(sessionID)
	
	// Check authentication via database token
	isAuth := false
	if tokenJSON, found := h.db.GetCache(fmt.Sprintf("spotify:token:%s", sessionID)); found && tokenJSON != "" {
		isAuth = true
	}

	json.NewEncoder(w).Encode(struct {
		HasSongs        bool   `json:"has_songs"`
		PlaylistName    string `json:"playlist_name"`
		PlaylistURL     string `json:"playlist_url"`
		SongsCount      int    `json:"songs_count"`
		IsAuthenticated bool   `json:"is_authenticated"`
	}{
		HasSongs:        session != nil && len(session.Songs) > 0,
		PlaylistName:    "",
		PlaylistURL:     "",
		SongsCount:      0,
		IsAuthenticated: isAuth,
	})
}

// APICachedArtists returns cached artists history
func (h *Handlers) APICachedArtists(w http.ResponseWriter, r *http.Request) {
	artistsJSON, found := h.db.GetCache("artists:history")
	if !found {
		json.NewEncoder(w).Encode([]string{})
		return
	}

	var artists []string
	json.Unmarshal([]byte(artistsJSON), &artists)
	json.NewEncoder(w).Encode(artists)
}

// Helper functions

func (h *Handlers) getSessionID(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *Handlers) getOrCreateSessionID(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Create new session ID
	sessionID := generateSessionID()
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400 * 30, // 30 days
	})
	return sessionID
}

func (h *Handlers) updateArtistsHistory(artist string) {
	var artists []string
	artistsJSON, found := h.db.GetCache("artists:history")
	if found {
		json.Unmarshal([]byte(artistsJSON), &artists)
	}

	// Check if artist already exists (case-insensitive)
	for _, a := range artists {
		if strings.EqualFold(a, artist) {
			return
		}
	}

	artists = append(artists, artist)
	artistsJSONBytes, _ := json.Marshal(artists)
	h.db.SetCache("artists:history", string(artistsJSONBytes), 3600*24*30) // 30 days
}

// APIReleasesRemove removes a release and all its songs
func (h *Handlers) APIReleasesRemove(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	if sessionID == "" {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	var req struct {
		Artist      string `json:"artist"`
		ReleaseTitle string `json:"release_title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	session, _ := h.sessions.Get(sessionID)
	if session == nil {
		json.NewEncoder(w).Encode(struct{ Success bool }{false})
		return
	}

	// Remove songs from this release
	var newSongs []core.SongWithRelease
	for _, song := range session.Songs {
		if !(strings.EqualFold(song.Artist, req.Artist) && song.ReleaseTitle == req.ReleaseTitle) {
			newSongs = append(newSongs, song)
		}
	}
	session.Songs = newSongs

	// Remove release from releases map
	if session.Releases != nil {
		var remainingReleases []core.Release
		for _, rel := range session.Releases[req.Artist] {
			if rel.Title != req.ReleaseTitle {
				remainingReleases = append(remainingReleases, rel)
			}
		}
		if len(remainingReleases) > 0 {
			session.Releases[req.Artist] = remainingReleases
		} else {
			delete(session.Releases, req.Artist)
		}
	}

	json.NewEncoder(w).Encode(struct{ Success bool }{true})
}

func deduplicateSongs(songs []core.SongWithRelease) []core.SongWithRelease {
	seen := make(map[string]bool)
	var result []core.SongWithRelease

	for _, song := range songs {
		key := fmt.Sprintf("%s|%s", strings.ToLower(song.Title), strings.ToLower(song.Artist))
		if !seen[key] {
			seen[key] = true
			result = append(result, song)
		}
	}
	return result
}

func generateSessionID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(16))
}

func generateRandomState() string {
	return randomString(32)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}