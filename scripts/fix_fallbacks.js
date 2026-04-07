require("dotenv").config()
const https = require("https")
const fs = require("fs")
const path = require("path")
const { createClient } = require("@supabase/supabase-js")
const { Client } = require("pg")
const sharp = require("sharp")

const SUPABASE_URL = process.env.SUPABASE_URL
const SUPABASE_KEY = process.env.SUPABASE_SERVICE_ROLE_KEY || process.env.SUPABASE_ANON_KEY
const DB_URL = process.env.SUPABASE_DB_URL
const THUMB_WIDTH = 200
const FULL_WIDTH = 600
const QUALITY = 75

const supabase = createClient(SUPABASE_URL, SUPABASE_KEY)
const db = new Client({ connectionString: DB_URL, ssl: { rejectUnauthorized: false } })

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
						fetch(response.headers.location, redirects + 1)
					} else if (response.statusCode === 200) {
						const chunks = []
						response.on("data", chunk => chunks.push(chunk))
						response.on("end", () => resolve(Buffer.concat(chunks)))
						response.on("error", reject)
					} else {
						reject(new Error(`Failed: ${response.statusCode}`))
					}
				})
				.on("error", reject)
		}
		fetch(url, 0)
	})
}

function sanitizeSlug(slug) {
	return slug.replace(/[\/\\&()\s]/g, "-")
}

async function main() {
	await db.connect()
	const result = await db.query(
		`SELECT id, slug, name FROM products WHERE image_thumb LIKE 'https://picsum.photos%'`
	)
	console.log(`Processing ${result.rows.length} products...\n`)

	for (const product of result.rows) {
		try {
			const safeSlug = sanitizeSlug(product.slug)
			// Use ID as seed since slug has special chars
			const imageUrl = `https://picsum.photos/seed/product-${product.id}/${FULL_WIDTH}/${Math.round(FULL_WIDTH * 0.75)}`

			const imageBuffer = await downloadImage(imageUrl)

			const tempDir = path.join(__dirname, "../temp")
			if (!fs.existsSync(tempDir)) fs.mkdirSync(tempDir, { recursive: true })

			const thumbPath = path.join(tempDir, `${safeSlug}-thumb.webp`)
			const fullPath = path.join(tempDir, `${safeSlug}-full.webp`)

			await sharp(imageBuffer)
				.resize(THUMB_WIDTH, null, { withoutEnlargement: true })
				.webp({ quality: QUALITY })
				.toFile(thumbPath)
			await sharp(imageBuffer)
				.resize(FULL_WIDTH, null, { withoutEnlargement: true })
				.webp({ quality: QUALITY })
				.toFile(fullPath)

			const thumbBuffer = fs.readFileSync(thumbPath)
			const fullBuffer = fs.readFileSync(fullPath)

			await supabase.storage
				.from("products")
				.upload(`${safeSlug}-thumb.webp`, thumbBuffer, { contentType: "image/webp", upsert: true })
			await supabase.storage
				.from("products")
				.upload(`${safeSlug}-full.webp`, fullBuffer, { contentType: "image/webp", upsert: true })

			const { data: thumbData } = supabase.storage
				.from("products")
				.getPublicUrl(`${safeSlug}-thumb.webp`)
			const { data: fullData } = supabase.storage
				.from("products")
				.getPublicUrl(`${safeSlug}-full.webp`)

			await db.query("UPDATE products SET image_thumb = $1, image_full = $2 WHERE id = $3", [
				thumbData.publicUrl,
				fullData.publicUrl,
				product.id,
			])

			fs.unlinkSync(thumbPath)
			fs.unlinkSync(fullPath)
			console.log(`✓ ${product.name}`)
		} catch (e) {
			console.log(`✗ ${product.name}: ${e.message}`)
		}
		await new Promise(r => setTimeout(r, 500))
	}

	await db.end()
	console.log("\nDone!")
}

main()
