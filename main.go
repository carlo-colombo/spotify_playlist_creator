package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"spotify_playlist_creator/core"
	"spotify_playlist_creator/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize database
	db, err := core.NewDatabase()
	if err != nil {
		log.Fatalf("Error setting up database: %v", err)
	}
	defer db.Close()

	// Create session store and handlers
	sessions := web.NewSessionStore(db)
	handlers := web.NewHandlers(db, sessions)

	// Setup routes
	mux := http.NewServeMux()
	web.SetupRoutes(mux, handlers)

	// Start server
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting Spotify Playlist Creator web server on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
