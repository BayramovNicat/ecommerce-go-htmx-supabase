require("dotenv").config()
const { Pool } = require("pg")
const pool = new Pool({ connectionString: process.env.SUPABASE_DB_URL })

async function check() {
	const result = await pool.query("SELECT COUNT(*) FROM products")
	console.log("Total products:", result.rows[0].count)

	const withImages = await pool.query(
		"SELECT COUNT(*) FROM products WHERE image_thumb IS NOT NULL AND image_thumb != ''"
	)
	console.log("Products with images:", withImages.rows[0].count)

	const withoutImages = await pool.query(
		"SELECT COUNT(*) FROM products WHERE image_thumb IS NULL OR image_thumb = ''"
	)
	console.log("Products without images:", withoutImages.rows[0].count)

	await pool.end()
}

check()
