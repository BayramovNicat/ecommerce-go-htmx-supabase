package shop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

const productsPerPage = 60

const homeProductsCacheTTL = 30 * time.Second
const productCacheTTL = 5 * time.Minute
const categoriesCacheTTL = 10 * time.Minute

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

var categoriesCache struct {
	mu         sync.RWMutex
	categories []database.Category
	bySlug     map[string]database.Category
	expiresAt  time.Time
}

func init() {
	productCache.items = make(map[string]*cachedProduct)
}

// getCategories returns all categories, using an in-memory cache.
func getCategories(ctx context.Context) ([]database.Category, map[string]database.Category, error) {
	now := time.Now()

	categoriesCache.mu.RLock()
	if now.Before(categoriesCache.expiresAt) && len(categoriesCache.categories) > 0 {
		cats := append([]database.Category(nil), categoriesCache.categories...)
		bySlug := categoriesCache.bySlug
		categoriesCache.mu.RUnlock()
		return cats, bySlug, nil
	}
	categoriesCache.mu.RUnlock()

	cats, err := database.GetCategories(ctx)
	if err != nil {
		return nil, nil, err
	}

	bySlug := make(map[string]database.Category, len(cats))
	for _, c := range cats {
		bySlug[c.Slug] = c
	}

	categoriesCache.mu.Lock()
	categoriesCache.categories = cats
	categoriesCache.bySlug = bySlug
	categoriesCache.expiresAt = now.Add(categoriesCacheTTL)
	categoriesCache.mu.Unlock()

	return cats, bySlug, nil
}

// resolveCategoryID returns the category ID for a slug, or 0 if slug is empty/unknown.
func resolveCategoryID(ctx context.Context, slug string) (int, error) {
	if slug == "" {
		return 0, nil
	}
	_, bySlug, err := getCategories(ctx)
	if err != nil {
		return 0, err
	}
	if cat, ok := bySlug[slug]; ok {
		return cat.ID, nil
	}
	return 0, nil
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
	categorySlug := r.URL.Query().Get("category")

	categories, _, err := getCategories(r.Context())
	if err != nil {
		log.Printf("Failed to load categories: %v", err)
		// Non-fatal: continue without categories
	}

	categoryID, err := resolveCategoryID(r.Context(), categorySlug)
	if err != nil {
		log.Printf("Failed to resolve category %q: %v", categorySlug, err)
	}

	var products []database.Product
	if categoryID == 0 && categorySlug == "" {
		// All-products first page: use cache
		products, err = getHomeProducts(r.Context())
	} else {
		products, err = database.GetProductsKeyset(r.Context(), 0, productsPerPage, categoryID)
	}
	if err != nil {
		http.Error(w, "Failed to load products: "+err.Error(), http.StatusInternalServerError)
		return
	}

	title := "Shop - Premium Products"
	if categorySlug != "" {
		for _, c := range categories {
			if c.Slug == categorySlug {
				title = c.Name + " - Meridian Living"
				break
			}
		}
	}

	data := map[string]interface{}{
		"Products":       products,
		"Title":          title,
		"User":           getUserFromRequest(r),
		"Env":            getEnv(),
		"CriticalCSS":    web.GetCriticalCSS(),
		"Categories":     categories,
		"ActiveCategory": categorySlug,
		"Category":       categorySlug,
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

// renderProductCards renders product cards as HTML fragment for HTMX.
// categorySlug is threaded into the infinite-scroll cursor URL so pagination
// stays within the same category.
func renderProductCards(w http.ResponseWriter, products []database.Product, categorySlug string) {
	if len(products) == 0 {
		w.Write([]byte(""))
		return
	}

	lastID := products[len(products)-1].ID

	// Build the next-page URL including category if present
	nextURL := fmt.Sprintf("/api/products?cursor=%d", lastID)
	if categorySlug != "" {
		nextURL += "&category=" + categorySlug
	}

	var html strings.Builder
	html.Grow(len(products) * 512)

	for i, product := range products {
		// Apply hx-trigger="revealed" to the 10th-to-last item for pre-fetch buffer
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

// CartItemView is the template-friendly representation of a cart line.
type CartItemView struct {
	Slug          string
	Name          string
	Price         float64
	ImageThumb    string
	Quantity      int
	QuantityMinus int
	QuantityPlus  int
	Subtotal      float64
}

// getCartSessionID reads the cart_sid cookie, or generates and sets a new one.
func getCartSessionID(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie("cart_sid"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "fallback-session"
	}
	sid := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     "cart_sid",
		Value:    sid,
		Path:     "/",
		MaxAge:   7 * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return sid
}

// buildCartData loads cart items for the session and returns template data.
func buildCartData(w http.ResponseWriter, r *http.Request) (map[string]interface{}, error) {
	sid := getCartSessionID(w, r)
	dbItems, err := database.GetCartItems(r.Context(), sid)
	if err != nil {
		return nil, err
	}

	items := make([]CartItemView, len(dbItems))
	var total float64
	for i, item := range dbItems {
		qm := item.Quantity - 1
		if qm < 0 {
			qm = 0
		}
		subtotal := item.Price * float64(item.Quantity)
		total += subtotal
		items[i] = CartItemView{
			Slug:          item.ProductSlug,
			Name:          item.ProductName,
			Price:         item.Price,
			ImageThumb:    item.ImageThumb,
			Quantity:      item.Quantity,
			QuantityMinus: qm,
			QuantityPlus:  item.Quantity + 1,
			Subtotal:      subtotal,
		}
	}

	return map[string]interface{}{
		"Title":       "Your Cart",
		"User":        getUserFromRequest(r),
		"Env":         getEnv(),
		"CriticalCSS": web.GetCriticalCSS(),
		"Items":       items,
		"Total":       total,
	}, nil
}

// HandleCart renders the cart page.
func HandleCart(w http.ResponseWriter, r *http.Request) {
	data, err := buildCartData(w, r)
	if err != nil {
		http.Error(w, "Failed to load cart: "+err.Error(), http.StatusInternalServerError)
		return
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

// renderCartContent renders just the #cart-content fragment (used after mutations).
func renderCartContent(w http.ResponseWriter, r *http.Request) {
	data, err := buildCartData(w, r)
	if err != nil {
		http.Error(w, "Failed to load cart: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl, err := web.GetTemplate("shop:cart", "templates/layouts/base.html", "templates/shop/cart.html")
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "cart_content", data); err != nil {
		http.Error(w, "Template execute error: "+err.Error(), http.StatusInternalServerError)
	}
}

// HandleCartAdd handles POST /api/cart/items.
// Body JSON: {"slug": "product-slug", "quantity": 2}
func HandleCartAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug     string `json:"slug"`
		Quantity int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Slug == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if body.Quantity < 1 {
		body.Quantity = 1
	}

	sid := getCartSessionID(w, r)
	if err := database.UpsertCartItem(r.Context(), sid, body.Slug, body.Quantity); err != nil {
		http.Error(w, "Failed to add item: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// HandleCartUpdate handles PUT /api/cart/items/{slug}.
// Body JSON: {"quantity": N} — if N <= 0 the item is removed.
func HandleCartUpdate(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/cart/items/")
	if slug == "" {
		http.Error(w, "Missing slug", http.StatusBadRequest)
		return
	}

	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	sid := getCartSessionID(w, r)
	if body.Quantity <= 0 {
		if err := database.RemoveCartItem(r.Context(), sid, slug); err != nil {
			http.Error(w, "Failed to remove item: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if err := database.UpsertCartItem(r.Context(), sid, slug, body.Quantity); err != nil {
			http.Error(w, "Failed to update item: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	renderCartContent(w, r)
}

// HandleCartRemove handles DELETE /api/cart/items/{slug}.
func HandleCartRemove(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/cart/items/")
	if slug == "" {
		http.Error(w, "Missing slug", http.StatusBadRequest)
		return
	}

	sid := getCartSessionID(w, r)
	if err := database.RemoveCartItem(r.Context(), sid, slug); err != nil {
		http.Error(w, "Failed to remove item: "+err.Error(), http.StatusInternalServerError)
		return
	}

	renderCartContent(w, r)
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

	products, err := database.GetProductsKeyset(ctx, 0, productsPerPage, 0)
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
