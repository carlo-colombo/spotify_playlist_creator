# Spotify Playlist Creator

A web application that creates Spotify playlists with the latest releases from your favorite artists.

## Prerequisites

- Go installed
- A Spotify account

## Setup

1.  Create a Spotify App to get a Client ID and Client Secret:
    - Go to the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard/)
    - Log in with your Spotify account.
    - Click "Create an App".
    - Fill out the form and click "Create".
    - You will find your Client ID and Client Secret in the app's dashboard.

2.  Add a redirect URI:
    - In your Spotify app settings, add `http://127.0.0.1:8080/auth/callback` to the Redirect URIs.

3.  Set the following environment variables:

    ```bash
    export SPOTIFY_ID="your_client_id"
    export SPOTIFY_SECRET="your_client_secret"
    export SPOTIFY_REDIRECT_URL="http://127.0.0.1:8080/auth/callback"
    ```

## Usage

Run the web server:

```bash
go run main.go
```

Then open http://localhost:8080 in your browser.

### Features

- **Add Artists** — Enter artist names to fetch their latest releases from MusicBrainz
- **Browse Albums & Songs** — View albums grouped with their songs; remove individual songs or entire albums
- **Canonical Names** — Artist names are fetched from MusicBrainz and used for playlist naming (sorted alphabetically)
- **Create Playlist** — Click "Create Playlist" to generate a Spotify playlist
- **Session Management** — Each browser session is independent; data is stored in SQLite

### Playlist Naming

Playlists are named using canonical artist names from MusicBrainz, sorted alphabetically:

```
spc Linkin Park,Muse,Rammstein
```

## Example

```bash
$ export SPOTIFY_ID="your_client_id"
$ export SPOTIFY_SECRET="your_client_secret"
$ export SPOTIFY_REDIRECT_URL="http://127.0.0.1:8080/auth/callback"
$ go run main.go
Starting Spotify Playlist Creator web server on http://localhost:8080
```

Then open http://localhost:8080 in your browser, connect Spotify, add artists, and create a playlist.
