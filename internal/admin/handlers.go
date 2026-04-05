package admin

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"htmxshop/internal/db"
	ui "htmxshop/ui"
)

// HandleDashboard renders the admin dashboard
func HandleDashboard(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Admin Dashboard",
	}

	tmpl := template.Must(template.ParseFS(ui.FS, "admin/dashboard.html"))
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// HandleProductsList renders the admin products list
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

	products, err := db.GetProductsKeyset(r.Context(), cursor, 50)
	if err != nil {
		http.Error(w, "Failed to load products", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Products": products,
		"Title":    "Manage Products",
	}

	tmpl := template.Must(template.ParseFS(ui.FS, "admin/products.html"))
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// HandleProductCreate creates a new product
func HandleProductCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	price, err := strconv.ParseFloat(r.FormValue("price"), 64)
	if err != nil {
		http.Error(w, "Invalid price", http.StatusBadRequest)
		return
	}

	stock, err := strconv.Atoi(r.FormValue("stock"))
	if err != nil {
		http.Error(w, "Invalid stock", http.StatusBadRequest)
		return
	}

	product := &db.Product{
		Name:          r.FormValue("name"),
		Slug:          r.FormValue("slug"),
		Description:   r.FormValue("description"),
		Price:         price,
		Stock:         stock,
		ImageURL:      r.FormValue("image_url"),
		ImageThumbURL: r.FormValue("image_thumb_url"),
		IsActive:      r.FormValue("is_active") == "true",
	}

	if err := db.CreateProduct(r.Context(), product); err != nil {
		http.Error(w, "Failed to create product", http.StatusInternalServerError)
		return
	}

	// Return success response for HTMX
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/admin/products")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/admin/products", http.StatusSeeOther)
}

// HandleProductUpdate updates an existing product
func HandleProductUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/admin/products/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	price, err := strconv.ParseFloat(r.FormValue("price"), 64)
	if err != nil {
		http.Error(w, "Invalid price", http.StatusBadRequest)
		return
	}

	stock, err := strconv.Atoi(r.FormValue("stock"))
	if err != nil {
		http.Error(w, "Invalid stock", http.StatusBadRequest)
		return
	}

	product := &db.Product{
		ID:            id,
		Name:          r.FormValue("name"),
		Slug:          r.FormValue("slug"),
		Description:   r.FormValue("description"),
		Price:         price,
		Stock:         stock,
		ImageURL:      r.FormValue("image_url"),
		ImageThumbURL: r.FormValue("image_thumb_url"),
		IsActive:      r.FormValue("is_active") == "true",
	}

	if err := db.UpdateProduct(r.Context(), product); err != nil {
		http.Error(w, "Failed to update product", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleProductDelete soft-deletes a product
func HandleProductDelete(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/admin/products/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	if err := db.DeleteProduct(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete product", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleOrdersList renders the admin orders list
func HandleOrdersList(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Manage Orders",
	}

	tmpl := template.Must(template.ParseFS(ui.FS, "admin/orders.html"))
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
