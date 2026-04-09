package shop

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"htmxshop/internal/database"
	"htmxshop/web"
)

// HandleSearch performs full-text search and returns results.
func HandleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Search query required", http.StatusBadRequest)
		return
	}

	cursorStr := r.URL.Query().Get("cursor")
	cursor := int64(0)
	if cursorStr != "" {
		var err error
		cursor, err = strconv.ParseInt(cursorStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid cursor", http.StatusBadRequest)
			return
		}
	}

	products, err := database.SearchProducts(r.Context(), query, cursor, productsPerPage)
	if err != nil {
		log.Printf("Search failed for query '%s': %v", query, err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Products":    products,
		"SearchQuery": query,
		"Title":       fmt.Sprintf("Search: %s", query),
		"User":        getUserFromRequest(r),
		"Env":         getEnv(),
		"CriticalCSS": web.GetCriticalCSS(),
	}

	tmpl, err := web.GetTemplate("shop:search", "templates/layouts/base.html", "templates/shop/search.html")
	if err != nil {
		log.Printf("Template parse error: %v", err)
		http.Error(w, "Template parse error", http.StatusInternalServerError)
		return
	}

	target := "search"
	if r.Header.Get("HX-Request") == "true" {
		target = "page_root"
	}

	if err := tmpl.ExecuteTemplate(w, target, data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
