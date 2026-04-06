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

// GetProductsKeyset retrieves products using keyset pagination (cursor-based)
// cursor is the last product ID from the previous page (0 for first page)
// limit is the number of products to retrieve
func GetProductsKeyset(ctx context.Context, cursor int64, limit int) ([]Product, error) {
	query := `
		SELECT id, uuid, name, slug, COALESCE(description, ''), price, stock,
		       COALESCE(image_url, ''), COALESCE(image_thumb_url, ''), is_active, created_at, updated_at
		FROM products
		WHERE is_active = true AND id < $1
		ORDER BY id DESC
		LIMIT $2
	`

	// For first page, use a very large cursor value
	if cursor == 0 {
		cursor = 9223372036854775807 // Max int64
	}

	rows, err := Pool.Query(ctx, query, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return products, nil
}

// GetProductBySlug retrieves a single product by its slug
func GetProductBySlug(ctx context.Context, slug string) (*Product, error) {
	query := `
		SELECT id, uuid, name, slug, COALESCE(description, ''), price, stock,
		       COALESCE(image_url, ''), COALESCE(image_thumb_url, ''), is_active, created_at, updated_at
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
		       COALESCE(image_url, ''), COALESCE(image_thumb_url, ''), is_active, created_at, updated_at
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
		INSERT INTO products (name, slug, description, price, stock, image_url, image_thumb_url, is_active)
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
		    stock = $5, image_url = $6, image_thumb_url = $7, is_active = $8
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
