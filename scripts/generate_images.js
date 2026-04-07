require("dotenv").config()

const https = require("https")
const fs = require("fs")
const path = require("path")
const { createClient } = require("@supabase/supabase-js")
const { Client } = require("pg")
const sharp = require("sharp")

// Configuration
const SUPABASE_URL = process.env.SUPABASE_URL
const SUPABASE_KEY = process.env.SUPABASE_SERVICE_ROLE_KEY || process.env.SUPABASE_ANON_KEY
const DB_URL = process.env.SUPABASE_DB_URL

const THUMB_WIDTH = 200
const FULL_WIDTH = 600
const QUALITY = 75

const supabase = createClient(SUPABASE_URL, SUPABASE_KEY)
const db = new Client({
	connectionString: DB_URL,
	ssl: { rejectUnauthorized: false },
})

// Download image from URL (follows redirects)
function downloadImage(url, maxRedirects = 5) {
	return new Promise((resolve, reject) => {
		const fetch = (currentUrl, redirects) => {
			if (redirects > maxRedirects) {
				reject(new Error("Too many redirects"))
				return
			}

			https
				.get(currentUrl, response => {
					if (
						response.statusCode >= 300 &&
						response.statusCode < 400 &&
						response.headers.location
					) {
						// Follow redirect
						fetch(response.headers.location, redirects + 1)
					} else if (response.statusCode === 200) {
						const chunks = []
						response.on("data", chunk => chunks.push(chunk))
						response.on("end", () => resolve(Buffer.concat(chunks)))
						response.on("error", reject)
					} else {
						reject(new Error(`Failed to download: ${response.statusCode}`))
					}
				})
				.on("error", reject)
		}

		fetch(url, 0)
	})
}

// Fetch image from Picsum Photos (free, unlimited, reliable)
async function fetchRandomImage(productId) {
	// Use Picsum - free, no API key, unlimited requests
	// Use product ID as seed to get consistent images
	const url = `https://picsum.photos/seed/${productId}/${FULL_WIDTH}/${Math.round(FULL_WIDTH * 0.75)}`
	return url
}

// Sanitize slug for file paths (replace invalid characters)
function sanitizeSlug(slug) {
	return slug.replace(/[\/\\&()\s]/g, "-")
}

// Process image: create thumbnail and full-size WebP versions
async function processImage(imageBuffer, productSlug) {
	const tempDir = path.join(__dirname, "../temp")
	if (!fs.existsSync(tempDir)) {
		fs.mkdirSync(tempDir, { recursive: true })
	}

	// Sanitize slug for file system compatibility
	const safeSlug = sanitizeSlug(productSlug)

	// Generate thumbnail
	const thumbPath = path.join(tempDir, `${safeSlug}-thumb.webp`)
	await sharp(imageBuffer)
		.resize(THUMB_WIDTH, null, { withoutEnlargement: true })
		.webp({ quality: QUALITY })
		.toFile(thumbPath)

	// Generate full-size
	const fullPath = path.join(tempDir, `${safeSlug}-full.webp`)
	await sharp(imageBuffer)
		.resize(FULL_WIDTH, null, { withoutEnlargement: true })
		.webp({ quality: QUALITY })
		.toFile(fullPath)

	return { thumbPath, fullPath, safeSlug }
}

// Upload to Supabase Storage
async function uploadToSupabase(filePath, storagePath) {
	const fileBuffer = fs.readFileSync(filePath)

	const { data, error } = await supabase.storage.from("products").upload(storagePath, fileBuffer, {
		contentType: "image/webp",
		upsert: true,
	})

	if (error) throw error
	return data
}

// Get products from database using direct PostgreSQL connection
async function getProducts(limit = 100, offset = 0) {
	console.log("Fetching products from database...")

	try {
		const result = await db.query(
			"SELECT id, slug, name FROM products WHERE image_thumb IS NULL ORDER BY id LIMIT $1 OFFSET $2",
			[limit, offset]
		)
		return result.rows
	} catch (error) {
		console.error("Database query error:", error)
		throw error
	}
}

// Update product with image URLs using direct PostgreSQL connection
async function updateProductImages(productId, thumbUrl, fullUrl) {
	try {
		await db.query("UPDATE products SET image_thumb = $1, image_full = $2 WHERE id = $3", [
			thumbUrl,
			fullUrl,
			productId,
		])
	} catch (error) {
		console.error("Database update error:", error)
		throw error
	}
}

// Main processing function
async function processProduct(product) {
	try {
		console.log(`Processing: ${product.name} (${product.slug})`)

		// Fetch image from Picsum (unique per product via slug seed)
		const imageUrl = await fetchRandomImage(product.slug)
		console.log(`  Fetched from Picsum`)

		// Download image
		const imageBuffer = await downloadImage(imageUrl)
		console.log(`  Downloaded (${(imageBuffer.length / 1024).toFixed(2)} KB)`)

		// Process and create WebP versions (with sanitized slug for file paths)
		const { thumbPath, fullPath, safeSlug } = await processImage(imageBuffer, product.slug)
		const thumbSize = fs.statSync(thumbPath).size
		const fullSize = fs.statSync(fullPath).size
		console.log(
			`  Created thumb (${(thumbSize / 1024).toFixed(2)} KB) and full (${(fullSize / 1024).toFixed(2)} KB)`
		)

		// Upload to Supabase using sanitized slug
		await uploadToSupabase(thumbPath, `${safeSlug}-thumb.webp`)
		await uploadToSupabase(fullPath, `${safeSlug}-full.webp`)
		console.log(`  Uploaded to Supabase`)

		// Get public URLs using sanitized slug
		const { data: thumbData } = supabase.storage
			.from("products")
			.getPublicUrl(`${safeSlug}-thumb.webp`)

		const { data: fullData } = supabase.storage
			.from("products")
			.getPublicUrl(`${safeSlug}-full.webp`)

		// Update database with URLs (these point to sanitized slug filenames)
		await updateProductImages(product.id, thumbData.publicUrl, fullData.publicUrl)
		console.log(`  Updated database`)

		// Cleanup temp files
		fs.unlinkSync(thumbPath)
		fs.unlinkSync(fullPath)

		console.log(`✓ Completed: ${product.name}\n`)

		// Small delay to be polite to the server
		await new Promise(resolve => setTimeout(resolve, 500))
	} catch (error) {
		console.error(`✗ Failed: ${product.name} - ${error.message}\n`)
	}
}

// Main execution
async function main() {
	const args = process.argv.slice(2)
	const limit = parseInt(args[0]) || 10
	const offset = parseInt(args[1]) || 0

	console.log(`\n=== Image Generation Script ===`)
	console.log(`Processing ${limit} products starting from offset ${offset}\n`)

	try {
		// Connect to database
		await db.connect()
		console.log("Connected to database\n")

		const products = await getProducts(limit, offset)
		console.log(`Found ${products.length} products\n`)

		for (const product of products) {
			await processProduct(product)
		}

		console.log(`\n=== Completed ===`)
		console.log(`Processed ${products.length} products`)

		// Disconnect from database
		await db.end()
		console.log("Database connection closed")
	} catch (error) {
		console.error("Fatal error:", error)
		await db.end()
		process.exit(1)
	}
}

main()
