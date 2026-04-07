package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool
var initOnce sync.Once
var initErr error

// Init initializes the database connection pool
func Init() error {
	initOnce.Do(func() {
		dbURL := os.Getenv("SUPABASE_DB_URL")
		if dbURL == "" {
			initErr = fmt.Errorf("SUPABASE_DB_URL environment variable is required")
			return
		}

		config, err := pgxpool.ParseConfig(dbURL)
		if err != nil {
			initErr = fmt.Errorf("unable to parse database URL: %w", err)
			return
		}

		// Supabase pooler does not allow prepared statement cache or prepared statements
		if config.ConnConfig.RuntimeParams == nil {
			config.ConnConfig.RuntimeParams = map[string]string{}
		}
		config.ConnConfig.RuntimeParams["statement_cache_mode"] = "describe"
		config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

		// Connection pool settings optimized for serverless
		config.MaxConns = 5
		config.MinConns = 0
		config.MaxConnLifetime = time.Minute * 10
		config.MaxConnIdleTime = time.Minute * 5
		config.HealthCheckPeriod = time.Minute * 5

		Pool, err = pgxpool.NewWithConfig(context.Background(), config)
		if err != nil {
			initErr = fmt.Errorf("unable to create connection pool: %w", err)
			return
		}

		log.Println("Database connection pool initialized successfully")
	})

	return initErr
}

// Close closes the database connection pool
func Close() {
	if Pool != nil {
		Pool.Close()
	}
}

// Category represents a product category
type Category struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	SortOrder int    `json:"sort_order"`
}

// GetCategories retrieves all categories ordered by sort_order
func GetCategories(ctx context.Context) ([]Category, error) {
	rows, err := Pool.Query(ctx, `SELECT id, name, slug, sort_order FROM categories ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	var cats []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.SortOrder); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// Product represents a product in the database
type Product struct {
	ID            int64     `json:"id"`
	UUID          string    `json:"uuid"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   string    `json:"description"`
	Price         float64   `json:"price"`
	Stock         int       `json:"stock"`
	ImageURL      string    `json:"image_url"`
	ImageThumbURL string    `json:"image_thumb_url"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// GetProductsKeyset retrieves products using keyset pagination (cursor-based).
// cursor is the last product ID from the previous page (0 = first page).
// categoryID filters by category; pass 0 for all categories.
func GetProductsKeyset(ctx context.Context, cursor int64, limit int, categoryID int) ([]Product, error) {
	if cursor == 0 {
		cursor = 9223372036854775807 // Max int64
	}

	var query string
	var args []interface{}

	if categoryID > 0 {
		// Uses idx_products_category_keyset (category_id, id DESC)
		query = `
			SELECT id, uuid, name, slug, COALESCE(description, ''), price, stock,
			       COALESCE(image_full, ''), COALESCE(image_thumb, ''), is_active, created_at, updated_at
			FROM products
			WHERE is_active = true AND category_id = $1 AND id < $2
			ORDER BY id DESC
			LIMIT $3
		`
		args = []interface{}{categoryID, cursor, limit}
	} else {
		// Uses idx_products_keyset (id DESC)
		query = `
			SELECT id, uuid, name, slug, COALESCE(description, ''), price, stock,
			       COALESCE(image_full, ''), COALESCE(image_thumb, ''), is_active, created_at, updated_at
			FROM products
			WHERE is_active = true AND id < $1
			ORDER BY id DESC
			LIMIT $2
		`
		args = []interface{}{cursor, limit}
	}

	rows, err := Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(
			&p.ID, &p.UUID, &p.Name, &p.Slug, &p.Description,
			&p.Price, &p.Stock, &p.ImageURL, &p.ImageThumbURL,
			&p.IsActive, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}

	return products, rows.Err()
}

// GetProductBySlug retrieves a single product by its slug
func GetProductBySlug(ctx context.Context, slug string) (*Product, error) {
	query := `
		SELECT id, uuid, name, slug, COALESCE(description, ''), price, stock,
		       COALESCE(image_full, ''), COALESCE(image_thumb, ''), is_active, created_at, updated_at
		FROM products
		WHERE slug = $1 AND is_active = true
		LIMIT 1
	`

	var p Product
	err := Pool.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.UUID, &p.Name, &p.Slug, &p.Description,
		&p.Price, &p.Stock, &p.ImageURL, &p.ImageThumbURL,
		&p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("query product by slug: %w", err)
	}

	return &p, nil
}

// SearchProducts performs full-text search using Postgres FTS
func SearchProducts(ctx context.Context, searchQuery string, cursor int64, limit int) ([]Product, error) {
	query := `
		SELECT id, uuid, name, slug, COALESCE(description, ''), price, stock,
		       COALESCE(image_full, ''), COALESCE(image_thumb, ''), is_active, created_at, updated_at
		FROM products
		WHERE is_active = true 
		  AND search_vector @@ plainto_tsquery('english', $1)
		  AND id < $2
		ORDER BY id DESC
		LIMIT $3
	`

	if cursor == 0 {
		cursor = 9223372036854775807
	}

	rows, err := Pool.Query(ctx, query, searchQuery, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		err := rows.Scan(
			&p.ID, &p.UUID, &p.Name, &p.Slug, &p.Description,
			&p.Price, &p.Stock, &p.ImageURL, &p.ImageThumbURL,
			&p.IsActive, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}

	return products, nil
}

// CreateProduct creates a new product (admin only)
func CreateProduct(ctx context.Context, p *Product) error {
	query := `
		INSERT INTO products (name, slug, description, price, stock, image_full, image_thumb, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, uuid, created_at, updated_at
	`

	err := Pool.QueryRow(ctx, query,
		p.Name, p.Slug, p.Description, p.Price, p.Stock,
		p.ImageURL, p.ImageThumbURL, p.IsActive,
	).Scan(&p.ID, &p.UUID, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create product: %w", err)
	}

	return nil
}

// UpdateProduct updates an existing product (admin only)
func UpdateProduct(ctx context.Context, p *Product) error {
	query := `
		UPDATE products
		SET name = $1, slug = $2, description = $3, price = $4, 
		    stock = $5, image_full = $6, image_thumb = $7, is_active = $8
		WHERE id = $9
		RETURNING updated_at
	`

	err := Pool.QueryRow(ctx, query,
		p.Name, p.Slug, p.Description, p.Price, p.Stock,
		p.ImageURL, p.ImageThumbURL, p.IsActive, p.ID,
	).Scan(&p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}

	return nil
}

// DeleteProduct soft-deletes a product by setting is_active to false
func DeleteProduct(ctx context.Context, id int64) error {
	query := `UPDATE products SET is_active = false WHERE id = $1`

	_, err := Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}

	return nil
}

// VerifyAdmin checks if a user is an admin
func VerifyAdmin(ctx context.Context, userID string) (bool, error) {
	query := `SELECT is_admin FROM admin_users WHERE id = $1`

	var isAdmin bool
	err := Pool.QueryRow(ctx, query, userID).Scan(&isAdmin)
	if err != nil {
		return false, fmt.Errorf("verify admin: %w", err)
	}

	return isAdmin, nil
}
