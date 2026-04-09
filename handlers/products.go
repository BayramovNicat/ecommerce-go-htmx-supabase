package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"htmxshop/db"
	"htmxshop/web"
)

func HandleProductsList(w http.ResponseWriter, r *http.Request) {
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

	categorySlug := r.URL.Query().Get("category")
	categoryID, err := resolveCategoryID(r.Context(), categorySlug)
	if err != nil {
		log.Printf("Failed to resolve category %q: %v", categorySlug, err)
	}

	products, err := db.GetProductsKeyset(r.Context(), cursor, productsPerPage, categoryID)
	if err != nil {
		http.Error(w, "Failed to load products", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		renderProductCards(w, products, categorySlug)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func HandleProductDetail(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/products/")

	product, err := getCachedProduct(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=600")

	data := map[string]interface{}{
		"Product":     product,
		"Title":       product.Name,
		"User":        getUserFromRequest(r),
		"Env":         getEnv(),
		"CriticalCSS": web.GetCriticalCSS(),
	}

	tmpl, err := web.GetTemplate("shop:product", "templates/layouts/base.html", "templates/shop/product.html")
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	target := "product"
	if r.Header.Get("HX-Request") == "true" {
		target = "page_root"
	}

	if err := tmpl.ExecuteTemplate(w, target, data); err != nil {
		http.Error(w, "Template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

func renderProductCards(w http.ResponseWriter, products []db.Product, categorySlug string) {
	if len(products) == 0 {
		return
	}

	tmpl, err := web.GetTemplate("shop:products_scroll",
		"templates/components/products_scroll_fragment.html",
		"templates/components/product_card_grid.html",
	)
	if err != nil {
		log.Printf("renderProductCards: template parse error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Products": products,
		"Category": categorySlug,
	}

	if err := tmpl.ExecuteTemplate(w, "products_scroll_fragment", data); err != nil {
		log.Printf("renderProductCards: template execute error: %v", err)
	}
}
