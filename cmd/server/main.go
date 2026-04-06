package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
	handler "htmxshop/api"
)

func main() {
	// Load .env file for local development
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if serveDistFile(w, r) {
			return
		}
		handler.Handler(w, r)
	})

	handlerWithCors := cors.AllowAll().Handler(mux)

	log.Printf("Server starting on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, handlerWithCors); err != nil {
		log.Fatal(err)
	}
}

func serveDistFile(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}

	path := r.URL.Path
	if path == "/" || strings.Contains(path, "..") {
		return false
	}

	filePath := filepath.Join("dist", strings.TrimPrefix(path, "/"))
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000")
	http.ServeFile(w, r, filePath)
	return true
}
