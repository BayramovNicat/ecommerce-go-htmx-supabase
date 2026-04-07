require("dotenv").config()
const { Pool } = require("pg")
const pool = new Pool({ connectionString: process.env.SUPABASE_DB_URL })

async function assignFallbacks() {
	const result = await pool.query(
		"SELECT id, slug FROM products WHERE image_thumb IS NULL OR image_thumb = ''"
	)
	console.log(`Assigning fallback images to ${result.rows.length} products...`)

	for (const row of result.rows) {
		const thumbUrl = `https://picsum.photos/seed/fallback-${row.id}/200/150`
		const fullUrl = `https://picsum.photos/seed/fallback-${row.id}/600/450`

		await pool.query("UPDATE products SET image_thumb = $1, image_full = $2 WHERE id = $3", [
			thumbUrl,
			fullUrl,
			row.id,
		])
	}

	console.log("Done! All products now have images.")
	await pool.end()
}

assignFallbacks()
