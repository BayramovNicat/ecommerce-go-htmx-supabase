-- Add categories table and wire products to it

CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    sort_order INT DEFAULT 0
);

INSERT INTO categories (name, slug, sort_order) VALUES
    ('Furniture',  'furniture',  1),
    ('Lighting',   'lighting',   2),
    ('Textiles',   'textiles',   3),
    ('Decor',      'decor',      4),
    ('Storage',    'storage',    5),
    ('Kitchen',    'kitchen',    6),
    ('Outdoor',    'outdoor',    7),
    ('Art',        'art',        8);

ALTER TABLE products ADD COLUMN category_id INT REFERENCES categories(id);

-- Distribute seed products evenly across categories
UPDATE products SET category_id = (id % 8) + 1;

-- Composite index: O(1) keyset pagination per category
CREATE INDEX idx_products_category_keyset
    ON products (category_id, id DESC)
    WHERE is_active = true;

-- RLS: public can read categories
ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
CREATE POLICY "public read categories" ON categories FOR SELECT USING (true);
