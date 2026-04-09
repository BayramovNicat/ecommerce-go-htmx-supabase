package handler

import (
	"io"
	"log"
	"net/http"
	"strings"

	"htmxshop/db"
	"htmxshop/handlers"
	"htmxshop/web"
)

func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	file, err := web.FS.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	if strings.HasSuffix(path, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	} else if strings.HasSuffix(path, ".js") {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	io.Copy(w, file)
}

// Handler is the main entry point for Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/dist/") {
		serveStaticFile(w, r)
		return
	}

	if err := db.Init(); err != nil {
		log.Printf("Database initialization error: %v", err)
		http.Error(w, "Database connection failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	route(w, r)
}

func route(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		setCacheHeaders(w, r)
	}

	path := r.URL.Path
	method := r.Method

	switch {
	case path == "/" || path == "/shop":
		if method == http.MethodGet {
			handlers.HandleHome(w, r)
		}
	case path == "/cart":
		if method == http.MethodGet {
			handlers.HandleCart(w, r)
		}
	case path == "/login":
		if method == http.MethodGet {
			handlers.HandleLogin(w, r)
		}
	case path == "/auth/google":
		if method == http.MethodGet {
			handlers.HandleGoogleAuth(w, r)
		}
	case strings.HasPrefix(path, "/products/"):
		if method == http.MethodGet {
			handlers.HandleProductDetail(w, r)
		}
	case path == "/api/products":
		if method == http.MethodGet {
			handlers.HandleProductsList(w, r)
		}
	case path == "/api/cart/items":
		if method == http.MethodPost {
			handlers.HandleCartAdd(w, r)
		}
	case strings.HasPrefix(path, "/api/cart/items/"):
		if method == http.MethodPut {
			handlers.HandleCartUpdate(w, r)
		} else if method == http.MethodDelete {
			handlers.HandleCartRemove(w, r)
		}
	case path == "/search":
		if method == http.MethodGet {
			handlers.HandleSearch(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func setCacheHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Vary", "HX-Request")

	if isAuthenticated(r) {
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		return
	}

	path := r.URL.Path
	switch {
	case path == "/" || path == "/shop":
		w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	case strings.HasPrefix(path, "/products/"):
		w.Header().Set("Cache-Control", "public, max-age=120, stale-while-revalidate=300")
	case path == "/search" || path == "/api/products":
		w.Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=120")
	default:
		w.Header().Set("Cache-Control", "public, max-age=15, stale-while-revalidate=60")
	}
}

func isAuthenticated(r *http.Request) bool {
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		return true
	}
	_, err := r.Cookie("sb-access-token")
	return err == nil
}
