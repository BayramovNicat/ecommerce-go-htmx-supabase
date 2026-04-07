require("dotenv").config()
const { Client } = require("pg")

const client = new Client({
	connectionString: process.env.SUPABASE_DB_URL,
	ssl: { rejectUnauthorized: false },
})

async function checkImages() {
	try {
		await client.connect()
		const result = await client.query(
			"SELECT COUNT(*) as count FROM products WHERE image_thumb IS NOT NULL"
		)
		console.log("Products with images:", result.rows[0].count)
	} catch (error) {
		console.error("Error:", error.message)
	} finally {
		await client.end()
	}
}

checkImages()
