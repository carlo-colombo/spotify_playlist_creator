package core

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// ArtistResult holds releases and songs for an artist
type ArtistResult struct {
	CanonicalName string // Real artist name from MusicBrainz
	Releases      []Release
	Songs         []SongWithRelease
}

// ProcessArtist processes an artist and returns their releases and songs
func ProcessArtist(ctx context.Context, artistName string, spotifyClient *SpotifyClient, musicBrainzClient *MusicBrainzClient) (*ArtistResult, error) {
	// Ensure Spotify client is authenticated
	if err := spotifyClient.EnsureAuthenticated(ctx); err != nil {
		return nil, fmt.Errorf("spotify not authenticated: %w", err)
	}

	// 1. Get artist ID and canonical name from MusicBrainz
	artistID, canonicalName, err := musicBrainzClient.GetArtistIDAndName(artistName)
	if err != nil {
		return nil, fmt.Errorf("error getting artist ID for %s: %w", artistName, err)
	}

	// 2. Get releases from MusicBrainz
	releases, err := musicBrainzClient.GetLatestReleases(artistID)
	if err != nil {
		return nil, fmt.Errorf("error getting releases for %s: %w", artistName, err)
	}

	// Set artist name on releases
	for i := range releases {
		releases[i].Artist = canonicalName
	}

	// 3. Get tracks from Spotify
	var songs []SongWithRelease
	for _, release := range releases {
		for _, trackTitle := range release.TrackTitles {
			trackURI, err := spotifyClient.SearchTrack(ctx, trackTitle, artistName, release.Title)
			if err != nil {
				log.Printf("Error searching for track '%s' by '%s': %v", trackTitle, artistName, err)
				continue
			}
			if trackURI != "" {
				// Get full track details for recap
				track, err := spotifyClient.GetTrackDetails(ctx, trackURI)
				if err != nil {
					log.Printf("Error getting track details for %s: %v", trackURI, err)
					continue
				}
				artistNames := []string{}
				for _, artist := range track.Artists {
					artistNames = append(artistNames, artist.Name)
				}
				year := ""
				if len(track.Album.ReleaseDate) >= 4 {
					year = track.Album.ReleaseDate[:4]
				}
				songs = append(songs, SongWithRelease{
					TrackDetails: TrackDetails{
						Title:  track.Name,
						Artist: strings.Join(artistNames, ", "),
						Album:  track.Album.Name,
						Year:   year,
						URI:    track.URI,
					},
					ReleaseTitle: release.Title,
				})
			}
		}
	}

	return &ArtistResult{
		CanonicalName: canonicalName,
		Releases:      releases,
		Songs:         songs,
	}, nil
}
