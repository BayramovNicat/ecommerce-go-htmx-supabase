package shop

import (
	"context"
	"sync"
	"time"

	"htmxshop/internal/database"
)

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
