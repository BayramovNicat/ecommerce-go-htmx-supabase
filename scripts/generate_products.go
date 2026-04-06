package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
)

func main() {
	gofakeit.Seed(0) // Use consistent seed for reproducibility

	const numProducts = 5000
	const batchSize = 100

	file, err := os.Create("migrations/002_seed_products.sql")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	file.WriteString("-- Seed 5000 fake products\n\n")

	for batch := 0; batch < numProducts/batchSize; batch++ {
		file.WriteString("INSERT INTO products (name, slug, description, price, stock, is_active) VALUES\n")

		for i := 0; i < batchSize; i++ {
			product := generateProduct()

			// Escape single quotes in strings
			name := strings.ReplaceAll(product.Name, "'", "''")
			slug := strings.ReplaceAll(product.Slug, "'", "''")
			description := strings.ReplaceAll(product.Description, "'", "''")

			line := fmt.Sprintf("    ('%s', '%s', '%s', %.2f, %d, true)",
				name, slug, description, product.Price, product.Stock)

			if i < batchSize-1 {
				line += ","
			} else {
				line += ";"
			}

			file.WriteString(line + "\n")
		}

		file.WriteString("\n")
	}

	fmt.Printf("Successfully generated %d products in migrations/002_seed_products.sql\n", numProducts)
}

type Product struct {
	Name        string
	Slug        string
	Description string
	Price       float64
	Stock       int
}

func generateProduct() Product {
	// Generate diverse product types
	productTypes := []func() string{
		func() string { return gofakeit.CarMaker() + " " + gofakeit.CarModel() + " Accessory" },
		func() string { return gofakeit.Adjective() + " " + gofakeit.Noun() },
		func() string { return gofakeit.Color() + " " + gofakeit.Noun() },
		func() string { return gofakeit.BuzzWord() + " " + gofakeit.Noun() },
	}

	name := productTypes[gofakeit.Number(0, len(productTypes)-1)]()

	// Generate slug from name
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "'", "")

	// Add random suffix to ensure uniqueness
	slug = fmt.Sprintf("%s-%d", slug, gofakeit.Number(1000, 9999))

	// Generate description
	description := gofakeit.Sentence(gofakeit.Number(10, 25))

	// Generate price between $9.99 and $999.99
	price := float64(gofakeit.Number(999, 99999)) / 100.0

	// Generate stock between 0 and 500
	stock := gofakeit.Number(0, 500)

	return Product{
		Name:        name,
		Slug:        slug,
		Description: description,
		Price:       price,
		Stock:       stock,
	}
}
