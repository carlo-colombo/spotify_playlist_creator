package core

import "time"

// MusicBrainz Structs
type MusicBrainzArtist struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

type MusicBrainzReleaseGroup struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	PrimaryType      string   `json:"primary-type"`
	SecondaryTypes   []string `json:"secondary-types"`
	FirstReleaseDate string   `json:"first-release-date"`
}

type MusicBrainzRelease struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Date    string `json:"date"`
	Country string `json:"country"`
}

type MusicBrainzTrack struct {
	Title string `json:"title"`
}

// Spotify Structs
type SpotifyTrack struct {
	ID      string          `json:"id"`
	URI     string          `json:"uri"`
	Name    string          `json:"name"`
	Album   SpotifyAlbum    `json:"album"`
	Artists []SpotifyArtist `json:"artists"`
}

type SpotifyAlbum struct {
	Name        string `json:"name"`
	ReleaseDate string `json:"release_date"`
}

type SpotifyArtist struct {
	Name string `json:"name"`
}

type SpotifyPlaylist struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ExternalURLs struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
}

// App-specific Structs
type Release struct {
	ID          string
	Title       string
	Date        time.Time
	TrackTitles []string
	Artist      string
}

// SongWithRelease wraps TrackDetails with release info
type SongWithRelease struct {
	TrackDetails
	ReleaseTitle string
}

// TrackDetails holds track information for the playlist
type TrackDetails struct {
	Title  string
	Artist string
	Album  string
	Year   string
	URI    string
}
