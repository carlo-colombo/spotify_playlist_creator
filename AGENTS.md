# Spotify Playlist Creator Agent

This agent creates a Spotify playlist with the latest album from a list of artists.

## How to Use

1.  **Set Environment Variables:**

    Before running the agent, you need to set your Spotify API credentials as environment variables. Open your terminal and run the following commands, replacing `your_client_id` and `your_client_secret` with your actual credentials:

    ```bash
    export SPOTIFY_ID="your_client_id"
    export SPOTIFY_SECRET="your_client_secret"
    ```

2.  **Run the Agent:**

    Once the environment variables are set, you can run the agent using the following command:

    ```bash
    go run main.go
    ```

3.  **Provide Artists:**

    The agent will prompt you to enter a comma-separated list of artists. Type the names of the artists you want to include in the playlist and press Enter.

    ```
    Enter a comma-separated list of artists:
    Rammstein, Linkin Park, Muse
    ```

## Example

Here is an example of how to run the agent and the expected output:

```bash
$ export SPOTIFY_ID="your_client_id"
$ export SPOTIFY_SECRET="your_client_secret"
$ go run main.go
Enter a comma-separated list of artists:
Rammstein
Found artist: Rammstein
Latest album: Zeit by Rammstein
Successfully created playlist! View it here: https://open.spotify.com/playlist/your_playlist_id
```
