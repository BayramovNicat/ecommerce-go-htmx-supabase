package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"htmxshop/web"
)

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
		"User":             getUserFromRequest(r),
		"Env":              getEnv(),
		"CriticalCSS":      web.GetCriticalCSS(),
	}

	tmpl, err := web.GetTemplate("shop:login", "templates/layouts/base.html", "templates/shop/login.html")
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	target := "login"
	if r.Header.Get("HX-Request") == "true" {
		target = "page_root"
	}

	if err := tmpl.ExecuteTemplate(w, target, data); err != nil {
		http.Error(w, "Template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

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

	tmpl, err := web.GetTemplate("shop:oauth-callback", "templates/shop/oauth-callback.html")
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "oauth_callback", data); err != nil {
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

	if !strings.Contains(host, "localhost") && !strings.Contains(host, "127.0.0.1") {
		host = stripPort(host)
	}

	return fmt.Sprintf("%s://%s/auth/google", scheme, host)
}

func stripPort(host string) string {
	if colon := strings.Index(host, ":"); colon != -1 {
		return host[:colon]
	}
	return host
}
