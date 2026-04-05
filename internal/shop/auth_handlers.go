package shop

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"htmxshop/internal/auth"
	ui "htmxshop/ui"
)

// HandleLogin renders the Supabase login page with OAuth wiring
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseAnonKey := os.Getenv("SUPABASE_ANON_KEY")
	if supabaseURL == "" || supabaseAnonKey == "" {
		http.Error(w, "Supabase authentication is not configured", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":            "Secure Login",
		"SupabaseURL":      supabaseURL,
		"SupabaseAnonKey":  supabaseAnonKey,
		"OAuthRedirectURL": oauthRedirectURL(r),
	}

	tmpl, err := template.ParseFS(ui.FS, "shop/login.html")
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

// HandleGoogleAuth finalizes the Supabase OAuth login flow
func HandleGoogleAuth(w http.ResponseWriter, r *http.Request) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseAnonKey := os.Getenv("SUPABASE_ANON_KEY")
	if supabaseURL == "" || supabaseAnonKey == "" {
		http.Error(w, "Supabase authentication is not configured", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":           "Authenticating",
		"SupabaseURL":     supabaseURL,
		"SupabaseAnonKey": supabaseAnonKey,
		"SuccessRedirect": "/",
		"FailureRedirect": "/login",
	}

	tmpl, err := template.ParseFS(ui.FS, "shop/oauth-callback.html")
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func oauthRedirectURL(r *http.Request) string {
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	host = stripPort(host)

	return fmt.Sprintf("%s://%s/auth/google", scheme, host)
}

func stripPort(host string) string {
	if colon := strings.Index(host, ":"); colon != -1 {
		return host[:colon]
	}
	return host
}

// HandleGetSession returns the current user session from the server
func HandleGetSession(w http.ResponseWriter, r *http.Request) {
	// Extract JWT from cookie
	cookie, err := r.Cookie("sb-access-token")
	if err != nil {
		// No session
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": nil,
		})
		return
	}

	token := cookie.Value
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": nil,
		})
		return
	}

	// Verify the token with Supabase
	user, err := auth.VerifySupabaseToken(token)
	if err != nil {
		// Invalid token
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": nil,
		})
		return
	}

	// Return user info with email and metadata
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": map[string]interface{}{
			"id":            user.ID,
			"email":         user.Email,
			"user_metadata": user.UserMetadata,
		},
	})
}
