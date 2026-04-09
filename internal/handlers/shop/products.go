package shop

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"htmxshop/internal/database"
	"htmxshop/web"
)

// HandleProductsList returns products as JSON or HTML fragment for HTMX infinite scroll.
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

	products, err := database.GetProductsKeyset(r.Context(), cursor, productsPerPage, categoryID)
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

// HandleProductDetail renders a single product page.
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

// renderProductCards renders product cards as HTML fragment for HTMX infinite scroll.
func renderProductCards(w http.ResponseWriter, products []database.Product, categorySlug string) {
	if len(products) == 0 {
		w.Write([]byte(""))
		return
	}

	lastID := products[len(products)-1].ID

	nextURL := fmt.Sprintf("/api/products?cursor=%d", lastID)
	if categorySlug != "" {
		nextURL += "&category=" + categorySlug
	}

	var html strings.Builder
	html.Grow(len(products) * 512)

	for i, product := range products {
		triggerAttr := ""
		if i == len(products)-10 {
			triggerAttr = fmt.Sprintf(` hx-get="%s" hx-trigger="revealed" hx-swap="beforeend" hx-target="#products-grid" hx-push-url="false" hx-history="false"`, nextURL)
		}

		stockStatus := `<span class="text-sm text-green-600">In Stock</span>`
		if product.Stock <= 0 {
			stockStatus = `<span class="text-sm text-red-500">Out of Stock</span>`
		}

		imageHTML := ""
		if product.ImageThumbURL != "" {
			imageHTML = fmt.Sprintf(`<img src="%s" alt="%s" loading="lazy" class="absolute inset-0 w-full h-full object-cover group-hover:scale-105 transition duration-300" onerror="this.style.display = 'none'" />`,
				product.ImageThumbURL, product.Name)
		}

		fmt.Fprintf(&html, `
<div class="group"%s>
	<a href="/products/%s" class="block">
		<div class="aspect-square overflow-hidden rounded-lg bg-gray-200 mb-4 relative flex items-center justify-center">
			%s
			<div class="w-3/4 aspect-[3/1] border-2 border-gray-400 rounded-md flex items-center justify-center">
				<div class="w-16 h-16 rounded-full bg-gray-400"></div>
			</div>
		</div>
		<h3 class="text-lg font-medium text-gray-900 mb-2">%s</h3>
		<div class="flex items-center justify-between">
			<p class="text-xl font-semibold text-gray-900">$%.2f</p>
			%s
		</div>
	</a>
</div>`,
			triggerAttr,
			product.Slug,
			imageHTML,
			product.Name,
			product.Price,
			stockStatus,
		)
	}

	_, _ = w.Write([]byte(html.String()))
}
