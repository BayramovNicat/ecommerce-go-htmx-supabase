require("dotenv").config()
const { Pool } = require("pg")
const pool = new Pool({ connectionString: process.env.SUPABASE_DB_URL })

async function check() {
	const result = await pool.query(
		"SELECT id, name, slug, image_thumb, image_full FROM products ORDER BY id DESC LIMIT 100"
	)
	for (const row of result.rows) {
		console.log(`ID: ${row.id}`)
		console.log(`  Slug: ${row.slug}`)
		console.log(`  Thumb: ${row.image_thumb}`)
		console.log(`  Full: ${row.image_full}`)
		console.log()
	}
	await pool.end()
}

check()
