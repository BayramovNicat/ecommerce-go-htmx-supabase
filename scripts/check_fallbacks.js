require("dotenv").config()
const { Pool } = require("pg")
const pool = new Pool({ connectionString: process.env.SUPABASE_DB_URL })

async function check() {
	const result = await pool.query(
		`SELECT id, name, slug, image_thumb, image_full FROM products 
		 WHERE image_thumb LIKE 'https://picsum.photos%' OR image_thumb IS NULL OR image_thumb = ''
		 ORDER BY id`
	)
	console.log("Products with fallback Picsum URLs (not Supabase Storage):", result.rows.length)
	console.log()
	for (const row of result.rows) {
		console.log(`ID: ${row.id} | Slug: ${row.slug}`)
		console.log(`  Thumb: ${row.image_thumb}`)
		console.log(`  Full: ${row.image_full}`)
		console.log()
	}
	await pool.end()
}

check()
