require("dotenv").config()
const { Pool } = require("pg")
const pool = new Pool({ connectionString: process.env.SUPABASE_DB_URL })

async function check() {
	const result = await pool.query(
		"SELECT id, name, slug, image_thumb, image_full FROM products WHERE image_thumb IS NULL OR image_thumb = ''"
	)
	console.log("Products without images:", result.rows.length)
	console.log()
	for (const row of result.rows) {
		console.log(`ID: ${row.id} | Name: ${row.name} | Slug: ${row.slug}`)
	}
	await pool.end()
}

check()
