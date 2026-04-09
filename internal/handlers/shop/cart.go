package shop

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"htmxshop/internal/database"
	"htmxshop/web"
)

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
