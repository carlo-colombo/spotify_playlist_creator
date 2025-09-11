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

## Usage

You can provide the list of artists in one of three ways:

1.  **Command-line arguments:**

    ```bash
    go run main.go "Artist Name 1" "Artist Name 2"
    ```

2.  **`artists.txt` file:**

    Create a file named `artists.txt` in the same directory as the program, and list each artist on a new line:

    ```
    Artist Name 1
    Artist Name 2
    ```

    Then run the program without any arguments:

    ```bash
    go run main.go
    ```

3.  **Interactive prompt:**

    If you run the program without any arguments and without an `artists.txt` file, you will be prompted to enter the artists manually:

    ```bash
    go run main.go
    Enter artist names (comma-separated): Artist Name 1, Artist Name 2
    ```

### Dry Run

To see what the program would do without actually creating a playlist, use the `-dry-run` flag:

```bash
go run main.go -dry-run "Artist Name 1"
```

## Example

```bash
$ go run main.go Rammstein
Starting Spotify Playlist Creator...
Processing artist: Rammstein

--- Spotify Playlist Recap ---
Playlist Name: spc Rammstein
Playlist Link: https://open.spotify.com/playlist/your_playlist_id

Added Tracks:
Artist                                   Album                                    Song                                     Year
---------------------------------------- ---------------------------------------- ---------------------------------------- ----------
Rammstein                                Zeit                                     Zick Zack                                2022
Rammstein                                Zeit                                     Zeit                                     2022
------------------------------
Spotify Playlist Creator finished.
```
