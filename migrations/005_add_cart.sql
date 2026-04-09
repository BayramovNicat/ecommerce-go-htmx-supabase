-- ============================================
-- CART TABLE
-- ============================================

CREATE TABLE cart_items (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (session_id, product_id)
);

CREATE INDEX idx_cart_items_session ON cart_items (session_id);

CREATE TRIGGER update_cart_items_updated_at BEFORE UPDATE ON cart_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
