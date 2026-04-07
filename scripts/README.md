# Image Generation Script

This script fetches images from Unsplash, optimizes them as WebP format, and uploads them to Supabase Storage.

## Features

- Fetches random product images from Unsplash API
- Creates two versions: thumbnail (400px) and full-size (1200px)
- Converts to WebP format with 75% quality for optimal size
- Uploads to Supabase Storage
- Updates product records with image URLs

## Setup

1. Install dependencies:

```bash
npm install
```

2. Create a Supabase Storage bucket named `products`:
   - Go to your Supabase dashboard
   - Navigate to Storage
   - Create a new public bucket called `products`

3. Run the migration to add image columns:

```bash
# Apply migration 003_add_image_columns.sql to your database
```

4. Set environment variables in `.env`:

```env
UNSPLASH_ACCESS_KEY=your_unsplash_access_key
SUPABASE_URL=your_supabase_url
SUPABASE_ANON_KEY=your_supabase_anon_key
```

## Get Unsplash API Key

1. Go to https://unsplash.com/developers
2. Register your application
3. Copy your Access Key

## Usage

Process 10 products starting from the beginning:

```bash
npm run generate:images
```

Process specific number of products:

```bash
node scripts/generate_images.js 50
```

Process with offset (e.g., skip first 100 products):

```bash
node scripts/generate_images.js 50 100
```

## Rate Limits

- Unsplash free tier: 50 requests/hour
- Script includes 2-second delay between requests
- Process in batches to stay within limits

## Output

The script will:

- Download images from Unsplash
- Create optimized WebP versions (thumb + full)
- Upload to Supabase Storage at `products/{slug}-thumb.webp` and `products/{slug}-full.webp`
- Update database with public URLs
- Clean up temporary files

## Example Output

```
=== Image Generation Script ===
Processing 10 products starting from offset 0

Found 10 products

Processing: Premium Wireless Headphones (premium-wireless-headphones)
  Fetched from Unsplash
  Downloaded (245.32 KB)
  Created thumb (28.45 KB) and full (89.12 KB)
  Uploaded to Supabase
  Updated database
✓ Completed: Premium Wireless Headphones

...

=== Completed ===
Processed 10 products
```
