package web

const IndexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Spotify Playlist Creator</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        .fade-in { animation: fadeIn 0.3s ease-in; }
        @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
    </style>
</head>
<body class="bg-gray-900 text-gray-100 min-h-screen">
    <div class="container mx-auto px-4 py-8 max-w-4xl">
        <!-- Header -->
        <header class="text-center mb-8">
            <h1 class="text-3xl font-bold text-green-500 mb-2">Spotify Playlist Creator</h1>
            <p class="text-gray-400">Create playlists with the latest releases from your favorite artists</p>
        </header>

        <!-- Auth Section -->
        <div class="bg-gray-800 rounded-lg p-6 mb-6">
            <div class="flex justify-between items-center">
                <div>
                    <h2 class="text-xl font-semibold mb-1">Spotify Connection</h2>
                    <p class="text-gray-400 text-sm">
                        {{if .IsAuthenticated}}
                            <span class="text-green-400">✓ Connected</span>
                        {{else}}
                            <span class="text-yellow-400">Not connected</span>
                        {{end}}
                    </p>
                </div>
                {{if not .IsAuthenticated}}
                <a href="/auth/spotify" class="bg-green-500 hover:bg-green-600 text-white px-6 py-2 rounded-lg font-medium transition">
                    Connect Spotify
                </a>
                {{end}}
            </div>
        </div>

        <!-- Artists Section -->
        <div class="bg-gray-800 rounded-lg p-6 mb-6">
            <h2 class="text-xl font-semibold mb-4">Artists</h2>
            
            <!-- Add Artist Form -->
            <div class="flex gap-2 mb-4">
                <input type="text" id="artistInput" placeholder="Enter artist name" 
                    class="flex-1 bg-gray-700 border border-gray-600 rounded-lg px-4 py-2 focus:outline-none focus:border-green-500">
                <button onclick="addArtist()" class="bg-green-500 hover:bg-green-600 text-white px-6 py-2 rounded-lg font-medium transition">
                    Add
                </button>
            </div>

            <!-- Cached Artists (History) -->
            {{if .CachedArtists}}
            <div class="mb-4">
                <p class="text-gray-400 text-sm mb-2">Previously searched:</p>
                <div class="flex flex-wrap gap-2">
                    {{range .CachedArtists}}
                    <button onclick="selectCachedArtist('{{.}}')" 
                        class="bg-gray-700 hover:bg-gray-600 text-gray-300 px-3 py-1 rounded-full text-sm transition">
                        {{.}}
                    </button>
                    {{end}}
                </div>
            </div>
            {{end}}

            <!-- Current Artists List -->
            {{if .Artists}}
            <div>
                <p class="text-gray-400 text-sm mb-2">Current selection:</p>
                <div class="flex flex-wrap gap-2">
                    {{range .Artists}}
                    <span class="bg-gray-700 text-gray-200 px-3 py-1 rounded-full text-sm flex items-center gap-2">
                        {{.}}
                        <button onclick="removeArtist('{{.}}')" class="text-gray-400 hover:text-red-400">&times;</button>
                    </span>
                    {{end}}
                </div>
            </div>
            {{end}}
        </div>

        <!-- Albums & Songs Section (Integrated) -->
        <div class="bg-gray-800 rounded-lg p-4 mb-6">
            <div class="flex justify-between items-center mb-4">
                <h2 class="text-lg font-semibold">Albums & Songs</h2>
                <span class="text-gray-400 text-xs">{{len .Songs}} songs</span>
            </div>

            {{if .Songs}}
            {{$allSongs := .Songs}}
            <div class="space-y-3 max-h-96 overflow-y-auto text-sm">
                {{range $artist, $releaseList := .Releases}}
                {{range $release := $releaseList}}
                {{$releaseTitle := $release.Title}}
                <div class="border border-gray-700 rounded overflow-hidden">
                    <div class="bg-gray-700 px-3 py-2 flex justify-between items-center">
                        <div class="min-w-0">
                            <span class="font-medium text-green-400">{{$releaseTitle}}</span>
                            <span class="text-gray-500 text-xs ml-2">{{$artist}}</span>
                        </div>
                        <button onclick="removeRelease('{{$artist}}', '{{$releaseTitle}}')" 
                            class="text-gray-400 hover:text-red-400 px-2 py-1 text-sm rounded hover:bg-gray-600 transition">
                            Remove Album
                        </button>
                    </div>
                    <div class="divide-y divide-gray-700">
                        {{range $allSongs}}
                        {{if eq .ReleaseTitle $releaseTitle}}
                        <div class="flex justify-between items-center px-3 py-2 hover:bg-gray-750 fade-in">
                            <div class="flex-1 min-w-0">
                                <span class="truncate">{{.Title}}</span>
                            </div>
                            <button onclick="removeSong('{{.Title}}', '{{.Artist}}')" 
                                class="text-gray-500 hover:text-red-400 px-2 text-lg leading-none">
                                &times;
                            </button>
                        </div>
                        {{end}}
                        {{end}}
                    </div>
                </div>
                {{end}}
                {{end}}
            </div>
            {{else}}
            <p class="text-gray-500 text-center py-4 text-sm">Add artists to see albums and songs</p>
            {{end}}
        </div>

        <!-- Playlist Section -->
        <div class="bg-gray-800 rounded-lg p-6">
            <h2 class="text-xl font-semibold mb-4">Playlist</h2>
            
            {{if .PlaylistName}}
            <div class="mb-4">
                <p class="text-gray-400 text-sm mb-1">Playlist name:</p>
                <p class="text-lg font-medium">{{.PlaylistName}}</p>
            </div>
            {{end}}

            {{if .PlaylistURL}}
            <div class="mb-4 p-4 bg-green-900/30 rounded-lg border border-green-500/30">
                <p class="text-gray-400 text-sm mb-1">Playlist created!</p>
                <a href="{{.PlaylistURL}}" target="_blank" class="text-green-400 hover:underline text-lg">
                    {{.PlaylistURL}}
                </a>
            </div>
            {{end}}

            <button onclick="createPlaylist()" 
                {{if or (not .IsAuthenticated) (eq (len .Songs) 0)}}disabled{{end}}
                class="w-full bg-green-500 hover:bg-green-600 disabled:bg-gray-600 disabled:cursor-not-allowed text-white px-6 py-3 rounded-lg font-medium transition">
                {{if not .IsAuthenticated}}
                    Connect Spotify to create playlist
                {{else if eq (len .Songs) 0}}
                    Add songs to create playlist
                {{else}}
                    Create Playlist
                {{end}}
            </button>
        </div>

        <!-- Loading Overlay -->
        <div id="loading" class="fixed inset-0 bg-black/70 flex items-center justify-center hidden">
            <div class="bg-gray-800 rounded-lg p-8 text-center">
                <div class="animate-spin w-12 h-12 border-4 border-green-500 border-t-transparent rounded-full mx-auto mb-4"></div>
                <p class="text-lg">Processing...</p>
            </div>
        </div>
    </div>

    <script>
        function showLoading() { document.getElementById('loading').classList.remove('hidden'); }
        function hideLoading() { document.getElementById('loading').classList.add('hidden'); }

        async function addArtist() {
            const input = document.getElementById('artistInput');
            const artist = input.value.trim();
            if (!artist) return;

            showLoading();
            try {
                const res = await fetch('/api/artists', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({artist})
                });
                if (!res.ok) throw new Error('Failed to add artist');
                input.value = '';
                location.reload();
            } catch (e) {
                alert('Error: ' + e.message);
                hideLoading();
            }
        }

        function selectCachedArtist(artist) {
            document.getElementById('artistInput').value = artist;
            addArtist();
        }

        async function removeArtist(artist) {
            showLoading();
            try {
                const res = await fetch('/api/artists?artist=' + encodeURIComponent(artist), {method: 'DELETE'});
                if (!res.ok) throw new Error('Failed to remove artist');
                location.reload();
            } catch (e) {
                alert('Error: ' + e.message);
                hideLoading();
            }
        }

        async function removeSong(title, artist) {
            try {
                const res = await fetch('/api/songs', {
                    method: 'DELETE',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({title, artist})
                });
                if (!res.ok) throw new Error('Failed to remove song');
                location.reload();
            } catch (e) {
                alert('Error: ' + e.message);
            }
        }

        async function removeRelease(artist, releaseTitle) {
            try {
                const res = await fetch('/api/releases', {
                    method: 'DELETE',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({artist, release_title: releaseTitle})
                });
                if (!res.ok) throw new Error('Failed to remove release');
                location.reload();
            } catch (e) {
                alert('Error: ' + e.message);
            }
        }

        async function createPlaylist() {
            showLoading();
            try {
                const res = await fetch('/api/playlist/create', {method: 'POST'});
                if (!res.ok) {
                    const err = await res.json();
                    throw new Error(err.error || 'Failed to create playlist');
                }
                const data = await res.json();
                location.reload();
            } catch (e) {
                alert('Error: ' + e.message);
                hideLoading();
            }
        }

        // Enter key to add artist
        document.getElementById('artistInput').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') addArtist();
        });
    </script>
</body>
</html>`