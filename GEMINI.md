# Spotify Playlist Creator

## TLDR

A web application that creates Spotify playlists with the latest releases from favorite artists. Run with `go run main.go`, then open http://localhost:8080. Uses SQLite for caching to avoid API rate limits.

## Architecture

- **Web UI** — Go `net/http` server with Tailwind CSS
- **Core Package** — Business logic (MusicBrainz, Spotify clients, processor)
- **Web Package** — HTTP handlers, sessions, templates
- **SQLite Database** — Caches API responses and session data

## Key Features

- **Web Interface** — Add artists, browse albums/songs, create playlists
- **Integrated Albums & Songs** — Songs grouped by album; remove album or individual songs
- **Canonical Artist Names** — Fetches real names from MusicBrainz for playlist naming
- **Alphabetical Sorting** — Playlist name uses canonical names sorted: `spc Linkin Park,Muse,Rammstein`
- **OAuth Authentication** — Spotify OAuth flow with token storage in SQLite
- **Multi-session Support** — Each browser tab is an independent session
- **MusicBrainz Integration** — Fetches latest studio albums and subsequent singles
- **Batched Track Addition** — Adds tracks in batches of 100 (Spotify API limit)
- **Track Search Refinement** — Prioritizes title+artist matching, falls back to album matching

## Test Cases

- Don Broco
  - All songs from "Amazing Things"
  - Singles since: "Cellophane", "Fingernails", "Birthday Party"

## Pre-commit Checks

```bash
go build -o /dev/null .
go fmt ./...
```