package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	handler "htmxshop/api"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
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

	// Start file watcher for live reload (dev only)
	if os.Getenv("ENV") != "production" {
		go watchFiles()
	}

	mux := http.NewServeMux()

	// Live reload WebSocket endpoint (dev only)
	if os.Getenv("ENV") != "production" {
		mux.HandleFunc("/livereload", handleLiveReload)
	}

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

func handleLiveReload(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
		conn.Close()
	}()

	// Keep connection alive
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func watchFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("Failed to create file watcher:", err)
		return
	}
	defer watcher.Close()

	// Watch templates directory
	templatesDir := "web/templates"
	err = filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		log.Println("Failed to watch templates directory:", err)
		return
	}

	// Watch dist directory for CSS/JS changes
	distDir := "web/dist"
	if err := watcher.Add(distDir); err != nil {
		log.Println("Failed to watch dist directory:", err)
	}

	// Watch Go files in api, internal, web, and cmd directories
	goDirs := []string{"api", "internal", "web", "cmd"}
	for _, dir := range goDirs {
		err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return watcher.Add(path)
			}
			return nil
		})
		if err != nil {
			log.Printf("Failed to watch %s directory: %v\n", dir, err)
		}
	}

	log.Println("File watcher started for live reload")

	// Debounce rapid file changes
	var debounceTimer *time.Timer
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// Check if it's a Go file change
				if strings.HasSuffix(event.Name, ".go") {
					log.Println("Go file changed:", event.Name, "- Server restart required")
					// For Go files, we need to exit so nodemon can restart the server
					os.Exit(0)
				}

				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
					log.Println("File changed:", event.Name)
					notifyClients()
				})
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("File watcher error:", err)
		}
	}
}

func notifyClients() {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, []byte("reload")); err != nil {
			log.Println("Failed to notify client:", err)
			client.Close()
			delete(clients, client)
		}
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

	relativePath := strings.TrimPrefix(path, "/")
	if strings.HasPrefix(relativePath, "dist/") {
		relativePath = strings.TrimPrefix(relativePath, "dist/")
	}

	filePath := filepath.Join("web", "dist", relativePath)
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}

	if strings.HasPrefix(path, "/dist/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	http.ServeFile(w, r, filePath)
	return true
}
