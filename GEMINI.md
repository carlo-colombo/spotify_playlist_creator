# Spotify Playlist Creator Recap

## TLDR

This program creates a Spotify playlist for the last songs of an artist. The artist'''s name is passed as a command-line argument. The program is run with `go run . <comma separated list of artists>`. It uses a SQLite database for caching to avoid hitting API rate limits and to speed up the process. The cache is stored in a local file named `spotify_playlist_creator.db`.

This program is designed to create and update Spotify playlists with the latest studio album tracks and recent singles from a user-defined list of artists.

## Key Features and Improvements:

*   **Artist and Release Data Fetching:** Integrates with MusicBrainz to fetch comprehensive artist information, including their latest studio albums and singles released since those albums.
*   **Spotify Integration:** Authenticates with Spotify to create and update playlists, and search for tracks.
*   **Robust Caching (SQLite):** Implemented a SQLite database for caching MusicBrainz and Spotify API responses. This significantly reduces API calls and improves performance, especially for repeated runs with the same artists.
*   **MusicBrainz Artist Disambiguation:** Includes a specific workaround for artists with ambiguous names (e.g., "wargasm" is now correctly identified as "WARGASM (UK)") to ensure accurate data retrieval.
*   **MusicBrainz API Rate Limit Handling:** Implemented an exponential backoff strategy for MusicBrainz API requests to gracefully handle rate limiting (HTTP 503 Service Unavailable/429 Too Many Requests errors), ensuring more reliable data fetching.
*   **Generic Playlist Naming:** The generated Spotify playlist name is now a concise "Latest from <List of artist>" to avoid limited to a certain number of characters
*   **Batched Spotify Track Additions:** Tracks are now added to Spotify playlists in batches (100 tracks per request) to comply with Spotify API limits, resolving the "Too many ids requested" error.
*   **Improved Spotify Track Search:** The Spotify track search logic has been refined to be more flexible. It now prioritizes searching by track title and artist, and then attempts to match by album name if multiple results are found. This helps in finding tracks even when MusicBrainz and Spotify titles have minor discrepancies.
*   **Track Title Normalization:** A `cleanTrackTitle` helper function has been introduced to remove common parenthetical information (e.g., "(Live)", "(Remix)", "(feat. Artist)") and standardize delimiters, leading to more accurate Spotify search results.

## Remaining Known Issues:

*   **Persistent Spotify Track Matching Issues:** Despite improvements, some tracks with highly specific or unusual titles (especially those with non-standard characters or very long descriptive titles from MusicBrainz) may still not be found on Spotify. Further refinement of the `cleanTrackTitle` function or a more advanced fuzzy matching algorithm might be necessary for these edge cases.


## Test cases to not overoptimize

* Don Broco
    * All songs from last album "Amazing Things"
    * Songs from Single album released since then
        * Cellophane
        * Fingernails
        * Birthday Party 

## Pre-commit Checks

Before every commit, I will run `go build .` to ensure that the program compiles without errors. This helps to prevent broken commits.