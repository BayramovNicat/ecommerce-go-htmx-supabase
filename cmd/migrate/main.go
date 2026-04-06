package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	dbURL := os.Getenv("SUPABASE_DB_URL")
	if dbURL == "" {
		fmt.Println("Error: SUPABASE_DB_URL not set")
		os.Exit(1)
	}

	// Connect to database
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	fmt.Println("Connected to database successfully")

	// Read migration file
	migrationPath := filepath.Join("migrations", "002_seed_products.sql")
	sqlContent, err := os.ReadFile(migrationPath)
	if err != nil {
		fmt.Printf("Error reading migration file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running migration: %s\n", migrationPath)
	fmt.Println("This may take a minute...")

	// Execute migration
	_, err = conn.Exec(ctx, string(sqlContent))
	if err != nil {
		fmt.Printf("Error executing migration: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Migration completed successfully!")
	fmt.Println("✓ 5000 products have been seeded to the database")

	// Verify count
	var count int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&count)
	if err != nil {
		fmt.Printf("Warning: Could not verify product count: %v\n", err)
	} else {
		fmt.Printf("✓ Total products in database: %d\n", count)
	}
}
