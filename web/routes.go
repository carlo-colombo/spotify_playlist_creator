package web

import (
	"net/http"
)

// SetupRoutes configures all HTTP routes
func SetupRoutes(mux *http.ServeMux, handlers *Handlers) {
	// Home page
	mux.HandleFunc("/", handlers.Home)

	// Auth routes
	mux.HandleFunc("/auth/spotify", handlers.AuthSpotify)
	mux.HandleFunc("/auth/callback", handlers.AuthCallback)

	// API routes
	mux.HandleFunc("/api/artists", handlers.handleArtistsAPI)
	mux.HandleFunc("/api/songs", handlers.handleSongsAPI)
	mux.HandleFunc("/api/releases", handlers.handleReleasesAPI)
	mux.HandleFunc("/api/playlist/create", handlers.APIPlaylistCreate)
	mux.HandleFunc("/api/playlist/status", handlers.APIPlaylistStatus)
	mux.HandleFunc("/api/cached-artists", handlers.APICachedArtists)
}

// handleArtistsAPI routes to correct method based on HTTP verb
func (h *Handlers) handleArtistsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.APIArtistsList(w, r)
	case http.MethodPost:
		h.APIArtistsAdd(w, r)
	case http.MethodDelete:
		h.APIArtistsRemove(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSongsAPI routes to correct method based on HTTP verb
func (h *Handlers) handleSongsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		h.APISongsRemove(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleReleasesAPI routes to correct method based on HTTP verb
func (h *Handlers) handleReleasesAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		h.APIReleasesRemove(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
