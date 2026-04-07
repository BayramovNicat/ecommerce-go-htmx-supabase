require("dotenv").config()
const { Pool } = require("pg")
const pool = new Pool({ connectionString: process.env.SUPABASE_DB_URL })

async function check() {
	const r = await pool.query(
		"SELECT COUNT(*) FROM products WHERE image_thumb LIKE 'https://picsum.photos%'"
	)
	console.log("Products with direct Picsum URLs (not Supabase Storage):", r.rows[0].count)
	await pool.end()
}

check()
