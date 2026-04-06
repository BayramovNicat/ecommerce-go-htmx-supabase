-- ============================================
-- SUPABASE SCHEMA FOR HTMX E-COMMERCE
-- ============================================

-- Enable necessary extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ============================================
-- PRODUCTS TABLE
-- ============================================
CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID DEFAULT uuid_generate_v4() UNIQUE NOT NULL,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    stock INTEGER DEFAULT 0,
    image_url TEXT,
    image_thumb_url TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Full-Text Search generated column
    search_vector tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(name, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(description, '')), 'B')
    ) STORED
);

-- ============================================
-- INDEXES FOR EXTREME PERFORMANCE
-- ============================================

-- Keyset Pagination Index (CRITICAL: id DESC for cursor-based pagination)
CREATE INDEX idx_products_keyset ON products (id DESC) WHERE is_active = true;

-- Full-Text Search GIN Index
CREATE INDEX idx_products_search ON products USING GIN (search_vector);

-- Slug lookup (for product detail pages)
CREATE INDEX idx_products_slug ON products (slug) WHERE is_active = true;

-- Covering index for list queries (avoids table lookups)
CREATE INDEX idx_products_list_covering ON products (id DESC, name, price, image_thumb_url, slug) 
    WHERE is_active = true;

-- ============================================
-- ADMIN USERS TABLE
-- ============================================
CREATE TABLE admin_users (
    id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    email TEXT UNIQUE NOT NULL,
    is_admin BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- ORDERS TABLE
-- ============================================
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID DEFAULT uuid_generate_v4() UNIQUE NOT NULL,
    user_id UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    email TEXT NOT NULL,
    total DECIMAL(10, 2) NOT NULL,
    status TEXT DEFAULT 'pending', -- pending, paid, shipped, completed, cancelled
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_orders_keyset ON orders (id DESC);
CREATE INDEX idx_orders_user ON orders (user_id, id DESC);

-- ============================================
-- ORDER ITEMS TABLE
-- ============================================
CREATE TABLE order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT REFERENCES orders(id) ON DELETE CASCADE,
    product_id BIGINT REFERENCES products(id) ON DELETE SET NULL,
    product_name TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_order_items_order ON order_items (order_id);

-- ============================================
-- ROW LEVEL SECURITY (RLS)
-- ============================================

-- Enable RLS
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE admin_users ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE order_items ENABLE ROW LEVEL SECURITY;

-- Public can read active products
CREATE POLICY "Public can view active products" ON products
    FOR SELECT USING (is_active = true);

-- Only admins can modify products
CREATE POLICY "Admins can manage products" ON products
    FOR ALL USING (
        EXISTS (
            SELECT 1 FROM admin_users 
            WHERE id = auth.uid() AND is_admin = true
        )
    );

-- Users can view their own orders
CREATE POLICY "Users can view own orders" ON orders
    FOR SELECT USING (
        user_id = auth.uid() OR 
        EXISTS (SELECT 1 FROM admin_users WHERE id = auth.uid() AND is_admin = true)
    );

-- Admins can manage all orders
CREATE POLICY "Admins can manage orders" ON orders
    FOR ALL USING (
        EXISTS (SELECT 1 FROM admin_users WHERE id = auth.uid() AND is_admin = true)
    );

-- ============================================
-- FUNCTIONS
-- ============================================

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- SAMPLE DATA (Optional - for testing)
-- ============================================
INSERT INTO products (name, slug, description, price, stock, is_active) VALUES
    ('Premium Wireless Headphones', 'premium-wireless-headphones', 'High-quality noise-cancelling headphones with 30-hour battery life', 299.99, 50, true),
    ('Smart Fitness Watch', 'smart-fitness-watch', 'Track your health and fitness goals with this advanced smartwatch', 199.99, 100, true),
    ('Portable Bluetooth Speaker', 'portable-bluetooth-speaker', 'Waterproof speaker with incredible sound quality', 79.99, 75, true),
    ('USB-C Fast Charger', 'usb-c-fast-charger', '65W fast charging adapter compatible with all devices', 39.99, 200, true),
    ('Mechanical Keyboard RGB', 'mechanical-keyboard-rgb', 'Gaming keyboard with customizable RGB lighting', 149.99, 60, true);
