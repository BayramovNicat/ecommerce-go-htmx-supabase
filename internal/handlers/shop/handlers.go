package shop

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"

	"htmxshop/internal/database"
	"htmxshop/internal/middleware"
	"htmxshop/web"
)

const productsPerPage = 20

// getEnv returns the current environment (production or development)
func getEnv() string {
	env := os.Getenv("ENV")
	if env == "" {
		return "development"
	}
	return env
}

// jsonHelper is a template function to convert Go data to JSON
func jsonHelper(v interface{}) template.JS {
	b, _ := json.Marshal(v)
	return template.JS(b)
}

// HandleHome renders the shop homepage with initial products
func HandleHome(w http.ResponseWriter, r *http.Request) {
	products, err := database.GetProductsKeyset(r.Context(), 0, productsPerPage)
	if err != nil {
		http.Error(w, "Failed to load products: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check for user session
	var user map[string]interface{}
	cookie, err := r.Cookie("sb-access-token")
	if err == nil && cookie.Value != "" {
		userData, err := middleware.VerifySupabaseToken(cookie.Value)
		if err == nil {
			user = map[string]interface{}{
				"id":            userData.ID,
				"email":         userData.Email,
				"user_metadata": userData.UserMetadata,
			}
		}
	}

	data := map[string]interface{}{
		"Products": products,
		"Title":    "Shop - Premium Products",
		"User":     user,
		"Env":      getEnv(),
	}

	tmpl, err := template.New("base.html").Funcs(template.FuncMap{
		"json": jsonHelper,
	}).ParseFS(web.FS, "templates/layouts/base.html", "templates/shop/home.html")
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	target := "home"
	if r.Header.Get("HX-Request") == "true" {
		target = "page_root"
	}

	if err := tmpl.ExecuteTemplate(w, target, data); err != nil {
		http.Error(w, "Template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

// HandleProductsList returns products as JSON or HTML fragment for HTMX infinite scroll
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

	products, err := database.GetProductsKeyset(r.Context(), cursor, productsPerPage)
	if err != nil {
		http.Error(w, "Failed to load products", http.StatusInternalServerError)
		return
	}

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Return HTML fragment for HTMX
		renderProductCards(w, products)
		return
	}

	// Return JSON for API requests
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

// HandleProductDetail renders a single product page
func HandleProductDetail(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/products/")

	product, err := database.GetProductBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Check for user session
	var user map[string]interface{}
	cookie, err := r.Cookie("sb-access-token")
	if err == nil && cookie.Value != "" {
		userData, err := middleware.VerifySupabaseToken(cookie.Value)
		if err == nil {
			user = map[string]interface{}{
				"id":            userData.ID,
				"email":         userData.Email,
				"user_metadata": userData.UserMetadata,
			}
		}
	}

	data := map[string]interface{}{
		"Product": product,
		"Title":   product.Name,
		"User":    user,
		"Env":     getEnv(),
	}

	tmpl, err := template.New("base.html").Funcs(template.FuncMap{
		"json": jsonHelper,
	}).ParseFS(web.FS, "templates/layouts/base.html", "templates/shop/product.html")
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

// HandleSearch performs full-text search and returns results
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
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Check if this is an HTMX request
	// Full page render
	data := map[string]interface{}{
		"Products":    products,
		"SearchQuery": query,
		"Title":       fmt.Sprintf("Search: %s", query),
	}

	tmpl := template.Must(template.New("base.html").ParseFS(web.FS, "templates/layouts/base.html", "templates/shop/search.html"))

	target := "search"
	if r.Header.Get("HX-Request") == "true" {
		target = "page_root"
	}

	if err := tmpl.ExecuteTemplate(w, target, data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// renderProductCards renders product cards as HTML fragment for HTMX
func renderProductCards(w http.ResponseWriter, products []database.Product) {
	if len(products) == 0 {
		w.Write([]byte(""))
		return
	}

	// Get the last product ID for the next cursor
	lastID := products[len(products)-1].ID

	// Render each product card
	for i, product := range products {
		// Apply hx-trigger="revealed" to the 3rd-to-last item for pre-fetch buffer
		triggerAttr := ""
		if i == len(products)-3 {
			triggerAttr = fmt.Sprintf(` hx-get="/api/products?cursor=%d" hx-trigger="revealed" hx-swap="afterend"`, lastID)
		}

		card := fmt.Sprintf(`
<div class="product-card" style="content-visibility: auto;"%s>
	<a href="/products/%s">
		<img src="%s" alt="%s" loading="lazy" class="product-image">
		<h3 class="product-name">%s</h3>
		<p class="product-price">$%.2f</p>
	</a>
</div>`,
			triggerAttr,
			product.Slug,
			product.ImageThumbURL,
			product.Name,
			product.Name,
			product.Price,
		)

		w.Write([]byte(card))
	}
}

// HandleCart renders the cart page (static placeholder)
func HandleCart(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Your Cart",
		"User":  getUserFromRequest(r),
		"Env":   getEnv(),
	}

	tmpl, err := template.New("base.html").Funcs(template.FuncMap{
		"json": jsonHelper,
	}).ParseFS(web.FS, "templates/layouts/base.html", "templates/shop/cart.html")
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	target := "cart"
	if r.Header.Get("HX-Request") == "true" {
		target = "page_root"
	}

	if err := tmpl.ExecuteTemplate(w, target, data); err != nil {
		http.Error(w, "Template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

// getUserFromRequest extracts user info from the Supabase session cookie
func getUserFromRequest(r *http.Request) map[string]interface{} {
	cookie, err := r.Cookie("sb-access-token")
	if err != nil || cookie.Value == "" {
		return nil
	}

	userData, err := middleware.VerifySupabaseToken(cookie.Value)
	if err != nil {
		return nil
	}

	return map[string]interface{}{
		"id":            userData.ID,
		"email":         userData.Email,
		"user_metadata": userData.UserMetadata,
	}
}
