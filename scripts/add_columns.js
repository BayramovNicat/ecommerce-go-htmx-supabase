require("dotenv").config();
const { Client } = require("pg");

const client = new Client({
	connectionString: process.env.SUPABASE_DB_URL,
	ssl: { rejectUnauthorized: false },
});

async function addColumns() {
	try {
		await client.connect();
		console.log("Connected to database");

		await client.query(`
            ALTER TABLE products ADD COLUMN IF NOT EXISTS image_thumb TEXT;
            ALTER TABLE products ADD COLUMN IF NOT EXISTS image_full TEXT;
        `);

		console.log("Columns image_thumb and image_full added successfully");
	} catch (error) {
		console.error("Error:", error.message);
	} finally {
		await client.end();
		console.log("Connection closed");
	}
}

addColumns();
