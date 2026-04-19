# Spotify Playlist Creator

A web application that creates Spotify playlists with the latest releases from your favorite artists.

## How to Use

1.  **Set Environment Variables:**

    ```bash
    export SPOTIFY_ID="your_client_id"
    export SPOTIFY_SECRET="your_client_secret"
    export SPOTIFY_REDIRECT_URL="http://127.0.0.1:8080/auth/callback"
    ```

2.  **Run the Web Server:**

    ```bash
    go run main.go
    ```

3.  **Open in Browser:**

    Go to http://localhost:8080

4.  **Connect Spotify:**

    Click "Connect Spotify" to authenticate with your Spotify account.

5.  **Add Artists:**

    Enter artist names in the input field. The app fetches their latest releases from MusicBrainz.

6.  **Create Playlist:**

    Review the albums and songs, remove any you don't want, then click "Create Playlist".

## Example

```bash
$ export SPOTIFY_ID="your_client_id"
$ export SPOTIFY_SECRET="your_client_secret"
$ export SPOTIFY_REDIRECT_URL="http://127.0.0.1:8080/auth/callback"
$ go run main.go
Starting Spotify Playlist Creator web server on http://localhost:8080
```

Then open http://localhost:8080 in your browser.

## Features

- **Integrated Albums & Songs UI** — Songs are grouped by album with remove buttons for both
- **Canonical Artist Names** — Uses real artist names from MusicBrainz, sorted alphabetically for playlist name
- **Multi-session Support** — Each browser tab is an independent session
- **SQLite Caching** — API responses are cached locally to avoid rate limits
