package shop

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"htmxshop/internal/database"
	"htmxshop/internal/middleware"
	"htmxshop/web"
)

const productsPerPage = 20

const homeProductsCacheTTL = 30 * time.Second
const productCacheTTL = 5 * time.Minute

var homeProductsCache struct {
	mu        sync.RWMutex
	products  []database.Product
	expiresAt time.Time
}

var productCache struct {
	mu    sync.RWMutex
	items map[string]*cachedProduct
}

type cachedProduct struct {
	product   *database.Product
	expiresAt time.Time
}

func init() {
	productCache.items = make(map[string]*cachedProduct)
}

// getEnv returns the current environment (production or development)
func getEnv() string {
	env := os.Getenv("ENV")
	if env == "" {
		return "development"
	}
	return env
}

// HandleHome renders the shop homepage with initial products
func HandleHome(w http.ResponseWriter, r *http.Request) {
	products, err := getHomeProducts(r.Context())
	if err != nil {
		http.Error(w, "Failed to load products: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Products":    products,
		"Title":       "Shop - Premium Products",
		"User":        getUserFromRequest(r),
		"Env":         getEnv(),
		"CriticalCSS": web.GetCriticalCSS(),
	}

	tmpl, err := web.GetTemplate("shop:home", "templates/layouts/base.html", "templates/shop/home.html")
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

	product, err := getCachedProduct(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set cache headers for browser caching
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
		log.Printf("Search failed for query '%s': %v", query, err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Check if this is an HTMX request
	// Full page render
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

// renderProductCards renders product cards as HTML fragment for HTMX
func renderProductCards(w http.ResponseWriter, products []database.Product) {
	if len(products) == 0 {
		w.Write([]byte(""))
		return
	}

	// Get the last product ID for the next cursor
	lastID := products[len(products)-1].ID

	var html strings.Builder
	html.Grow(len(products) * 512)

	// Render each product card
	for i, product := range products {
		// Apply hx-trigger="revealed" to the 3rd-to-last item for pre-fetch buffer
		triggerAttr := ""
		if i == len(products)-3 {
			triggerAttr = fmt.Sprintf(` hx-get="/api/products?cursor=%d" hx-trigger="revealed" hx-swap="afterend"`, lastID)
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

// HandleCart renders the cart page (static placeholder)
func HandleCart(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":       "Your Cart",
		"User":        getUserFromRequest(r),
		"Env":         getEnv(),
		"CriticalCSS": web.GetCriticalCSS(),
	}

	tmpl, err := web.GetTemplate("shop:cart", "templates/layouts/base.html", "templates/shop/cart.html")
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

func getHomeProducts(ctx context.Context) ([]database.Product, error) {
	now := time.Now()

	homeProductsCache.mu.RLock()
	if now.Before(homeProductsCache.expiresAt) && len(homeProductsCache.products) > 0 {
		cached := append([]database.Product(nil), homeProductsCache.products...)
		homeProductsCache.mu.RUnlock()
		return cached, nil
	}
	homeProductsCache.mu.RUnlock()

	products, err := database.GetProductsKeyset(ctx, 0, productsPerPage)
	if err != nil {
		return nil, err
	}

	homeProductsCache.mu.Lock()
	homeProductsCache.products = append(homeProductsCache.products[:0], products...)
	homeProductsCache.expiresAt = now.Add(homeProductsCacheTTL)
	homeProductsCache.mu.Unlock()

	return products, nil
}

func getCachedProduct(ctx context.Context, slug string) (*database.Product, error) {
	now := time.Now()

	productCache.mu.RLock()
	if cached, ok := productCache.items[slug]; ok && now.Before(cached.expiresAt) {
		productCache.mu.RUnlock()
		return cached.product, nil
	}
	productCache.mu.RUnlock()

	product, err := database.GetProductBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	productCache.mu.Lock()
	productCache.items[slug] = &cachedProduct{
		product:   product,
		expiresAt: now.Add(productCacheTTL),
	}
	productCache.mu.Unlock()

	return product, nil
}
