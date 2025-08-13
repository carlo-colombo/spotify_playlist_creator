# Spotify Playlist Creator

This program creates a Spotify playlist with the latest album from a list of artists.

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

2.  Set the following environment variables:

    ```bash
    export SPOTIFY_ID="your_client_id"
    export SPOTIFY_SECRET="your_client_secret"
    ```

3.  Run the program:

    ```bash
    go run main.go
    ```
