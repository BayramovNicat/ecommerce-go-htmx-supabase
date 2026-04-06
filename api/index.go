package handler

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"

	"htmxshop/internal/database"
	"htmxshop/internal/handlers/admin"
	"htmxshop/internal/handlers/shop"
	"htmxshop/internal/middleware"
	"htmxshop/web"
)

// serveStaticFile serves static assets from embedded filesystem
func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	file, err := web.FS.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	// Set appropriate content type
	if strings.HasSuffix(path, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	} else if strings.HasSuffix(path, ".js") {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}

	// Set cache headers for static assets
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	io.Copy(w, file)
}

// Handler is the main entry point for Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Serve static files from embedded filesystem
	if strings.HasPrefix(path, "/dist/") {
		serveStaticFile(w, r)
		return
	}

	// Lazy initialize database connection
	if err := database.Init(); err != nil {
		log.Printf("Database initialization error: %v", err)
		http.Error(w, "Database connection failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Route to admin or shop handlers
	if strings.HasPrefix(path, "/admin") {
		adminMiddleware(adminHandler)(w, r)
		return
	}

	// Public shop routes
	shopHandler(w, r)
}

// adminMiddleware verifies Supabase JWT and admin privileges
func adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract JWT from Authorization header or cookie
		authHeader := r.Header.Get("Authorization")
		cookieValue := ""

		if authHeader == "" {
			cookie, err := r.Cookie("sb-access-token")
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			cookieValue = cookie.Value
		}

		token := middleware.ExtractToken(authHeader, cookieValue)
		if token == "" {
			log.Println("admin auth: missing token")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Verify JWT token
		user, err := middleware.VerifySupabaseToken(token)
		if err != nil {
			log.Printf("admin auth: token verification failed: %v", err)
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		// Check admin privileges
		isAdmin, err := middleware.VerifyAdminAccess(r.Context(), user.ID)
		if err != nil || !isAdmin {
			if err != nil {
				log.Printf("admin auth: verify admin failed: %v", err)
			} else {
				log.Printf("admin auth: user %s not admin", user.ID)
			}
			http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			return
		}

		// Add user ID to context
		ctx := context.WithValue(r.Context(), "userID", user.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// shopHandler handles all public-facing shop routes
func shopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		setPublicRouteCacheHeaders(w, r)
	}

	path := r.URL.Path
	method := r.Method

	switch {
	case path == "/" || path == "/shop":
		if method == http.MethodGet {
			handleShopHome(w, r)
		}
	case path == "/cart":
		if method == http.MethodGet {
			handleCart(w, r)
		}
	case path == "/login":
		if method == http.MethodGet {
			handleLogin(w, r)
		}
	case path == "/auth/google":
		if method == http.MethodGet {
			handleGoogleAuth(w, r)
		}
	case strings.HasPrefix(path, "/products/"):
		if method == http.MethodGet {
			handleProductDetail(w, r)
		}
	case path == "/api/products":
		if method == http.MethodGet {
			handleProductsList(w, r)
		}
	case path == "/search":
		if method == http.MethodGet {
			handleSearch(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func setPublicRouteCacheHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Vary", "HX-Request")

	if isAuthenticatedRequest(r) {
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

func isAuthenticatedRequest(r *http.Request) bool {
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		return true
	}

	_, err := r.Cookie("sb-access-token")
	return err == nil
}

// adminHandler handles all admin dashboard routes
func adminHandler(w http.ResponseWriter, r *http.Request) {
	// No cache for admin routes
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")

	path := strings.TrimPrefix(r.URL.Path, "/admin")
	method := r.Method

	switch {
	case path == "" || path == "/":
		if method == http.MethodGet {
			handleAdminDashboard(w, r)
		}
	case path == "/products":
		if method == http.MethodGet {
			handleAdminProductsList(w, r)
		} else if method == http.MethodPost {
			handleAdminProductCreate(w, r)
		}
	case strings.HasPrefix(path, "/products/"):
		if method == http.MethodPut {
			handleAdminProductUpdate(w, r)
		} else if method == http.MethodDelete {
			handleAdminProductDelete(w, r)
		}
	case path == "/orders":
		if method == http.MethodGet {
			handleAdminOrdersList(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

// Shop handlers
func handleShopHome(w http.ResponseWriter, r *http.Request) {
	shop.HandleHome(w, r)
}

func handleCart(w http.ResponseWriter, r *http.Request) {
	shop.HandleCart(w, r)
}

func handleProductDetail(w http.ResponseWriter, r *http.Request) {
	shop.HandleProductDetail(w, r)
}

func handleProductsList(w http.ResponseWriter, r *http.Request) {
	shop.HandleProductsList(w, r)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	shop.HandleSearch(w, r)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	shop.HandleLogin(w, r)
}

func handleGoogleAuth(w http.ResponseWriter, r *http.Request) {
	shop.HandleGoogleAuth(w, r)
}

// Admin handlers
func handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	admin.HandleDashboard(w, r)
}

func handleAdminProductsList(w http.ResponseWriter, r *http.Request) {
	admin.HandleProductsList(w, r)
}

func handleAdminProductCreate(w http.ResponseWriter, r *http.Request) {
	admin.HandleProductCreate(w, r)
}

func handleAdminProductUpdate(w http.ResponseWriter, r *http.Request) {
	admin.HandleProductUpdate(w, r)
}

func handleAdminProductDelete(w http.ResponseWriter, r *http.Request) {
	admin.HandleProductDelete(w, r)
}

func handleAdminOrdersList(w http.ResponseWriter, r *http.Request) {
	admin.HandleOrdersList(w, r)
}
