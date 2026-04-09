package handlers

import (
	"log"
	"net/http"

	"htmxshop/db"
	"htmxshop/web"
)

func HandleHome(w http.ResponseWriter, r *http.Request) {
	categorySlug := r.URL.Query().Get("category")

	categories, _, err := getCategories(r.Context())
	if err != nil {
		log.Printf("Failed to load categories: %v", err)
	}

	categoryID, err := resolveCategoryID(r.Context(), categorySlug)
	if err != nil {
		log.Printf("Failed to resolve category %q: %v", categorySlug, err)
	}

	var products []db.Product
	if categoryID == 0 && categorySlug == "" {
		products, err = getHomeProducts(r.Context())
	} else {
		products, err = db.GetProductsKeyset(r.Context(), 0, productsPerPage, categoryID)
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

	tmpl, err := web.GetTemplate("shop:home",
		"templates/layouts/base.html",
		"templates/shop/home.html",
		"templates/components/product_card_grid.html",
		"templates/components/products_grid.html",
	)
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
