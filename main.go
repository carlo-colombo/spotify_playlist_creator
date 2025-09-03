package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

func main() {
	fmt.Println("Starting Spotify Playlist Creator...")

	dryRun := flag.Bool("dry-run", false, "Run in dry-run mode (do not create/modify playlists)")
	flag.Parse()

	artists, err := getArtists()
	if err != nil {
		log.Fatalf("Error getting artists: %v", err)
	}

	if len(artists) == 0 {
		log.Println("No artists found. Exiting.")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := setupDatabase()
	if err != nil {
		log.Fatalf("Error setting up database: %v", err)
	}
	defer db.Close()

	spotifyClient, err := setupSpotifyClient(ctx, db)
	if err != nil {
		log.Fatalf("Error setting up Spotify client: %v", err)
	}

	musicBrainzClient := setupMusicBrainzClient(db)

	var allTrackURIs []string
	for _, artistName := range artists {
		fmt.Printf("Processing artist: %s\n", artistName)
		trackURIs, err := processArtist(ctx, artistName, spotifyClient, musicBrainzClient)
		if err != nil {
			log.Printf("Error processing artist %s: %v", artistName, err)
			continue
		}
		allTrackURIs = append(allTrackURIs, trackURIs...)
	}

	if len(allTrackURIs) == 0 {
		log.Println("No tracks found for any artist. Exiting.")
		return
	}

			sort.Strings(artists)
	playlistName := fmt.Sprintf("spc %s", strings.Join(artists, ","))

	if *dryRun {
		fmt.Printf("Dry run: Would create/update playlist '%s'. Tracks to be added:\n", playlistName)
		for _, trackURI := range allTrackURIs {
			track, err := spotifyClient.GetTrackDetails(ctx, trackURI)
			if err != nil {
				fmt.Printf("\tError getting track details for %s: %v\n", trackURI, err)
				continue
			}
			artistNames := []string{}
			for _, artist := range track.Artists {
				artistNames = append(artistNames, artist.Name)
			}
			fmt.Printf("\t- %s - %s (Album: %s)\n", strings.Join(artistNames, ", "), track.Name, track.Album.Name)
		}
		return
	}

	playlist, err := spotifyClient.GetOrCreatePlaylist(ctx, playlistName)
	if err != nil {
		log.Fatalf("Error getting or creating playlist: %v", err)
	}

	err = spotifyClient.AddTracksToPlaylist(ctx, playlist.ID, allTrackURIs)
	if err != nil {
		log.Fatalf("Error adding tracks to playlist: %v", err)
	}

	fmt.Printf("Successfully updated playlist '%s'. You can find it here: %s\n", playlist.Name, playlist.ExternalURLs.Spotify)

	fmt.Println("Spotify Playlist Creator finished.")
}

func getArtists() ([]string, error) {
	var artists []string

	if len(flag.Args()) > 0 {
		artists = flag.Args()
		return artists, nil
	}

	// Check for artists.txt file
	if _, err := os.Stat("artists.txt"); err == nil {
		file, err := os.Open("artists.txt")
		if err != nil {
			return nil, fmt.Errorf("error opening artists.txt: %w", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			artists = append(artists, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading artists.txt: %w", err)
		}
	} else if os.IsNotExist(err) {
		// If no file, prompt the user
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter artist names (comma-separated): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if len(input) > 0 {
			artists = strings.Split(input, ",")
			for i := range artists {
				artists[i] = strings.TrimSpace(artists[i])
			}
		}
	} else {
		return nil, err
	}

	return artists, nil
}

func processArtist(ctx context.Context, artistName string, spotifyClient *SpotifyClient, musicBrainzClient *MusicBrainzClient) ([]string, error) {

	// 1. Get artist ID from MusicBrainz
	artistID, err := musicBrainzClient.GetArtistID(artistName)
	if err != nil {
		return nil, fmt.Errorf("error getting artist ID for %s: %w", artistName, err)
	}

	// 2. Get releases from MusicBrainz
	releases, err := musicBrainzClient.GetLatestReleases(artistID)
	if err != nil {
		return nil, fmt.Errorf("error getting releases for %s: %w", artistName, err)
	}

	// 3. Get tracks from Spotify
	var trackURIs []string
	for _, release := range releases {
		for _, trackTitle := range release.TrackTitles {
			trackURI, err := spotifyClient.SearchTrack(ctx, trackTitle, artistName, release.Title)
			if err != nil {
				log.Printf("Error searching for track '%s' by '%s': %v", trackTitle, artistName, err)
				continue
			}
			if trackURI != "" {
				trackURIs = append(trackURIs, trackURI)
			}
		}
	}

	return trackURIs, nil
}
