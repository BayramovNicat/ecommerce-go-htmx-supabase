-- Add separate columns for thumbnail and full-size images
ALTER TABLE products ADD COLUMN IF NOT EXISTS image_thumb TEXT;
ALTER TABLE products ADD COLUMN IF NOT EXISTS image_full TEXT;

-- Drop old columns if they exist
ALTER TABLE products DROP COLUMN IF EXISTS image_url;
ALTER TABLE products DROP COLUMN IF EXISTS image_thumb_url;

-- Update covering index to use new column names
DROP INDEX IF EXISTS idx_products_list_covering;
CREATE INDEX idx_products_list_covering ON products (id DESC, name, price, image_thumb, slug) 
    WHERE is_active = true;
