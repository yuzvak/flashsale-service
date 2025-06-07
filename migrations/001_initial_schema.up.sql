-- Flash Sale Service Database Schema
CREATE TABLE IF NOT EXISTS sales (
    id VARCHAR(20) PRIMARY KEY,           -- Format: S-{uuid}
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP NOT NULL,
    total_items INTEGER NOT NULL DEFAULT 10000,
    items_sold INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sales_active ON sales(ended_at);

-- Items table: all sale items
CREATE TABLE IF NOT EXISTS items (
    id VARCHAR(255) PRIMARY KEY,
    sale_id VARCHAR(20) NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    image_url VARCHAR(500) NOT NULL,
    sold BOOLEAN NOT NULL DEFAULT FALSE,
    sold_to_user_id VARCHAR(255),
    sold_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Optimized indexes for performance
CREATE INDEX IF NOT EXISTS idx_items_sale_sold ON items(sale_id) WHERE sold = FALSE;
CREATE INDEX IF NOT EXISTS idx_items_sale_user ON items(sale_id, sold_to_user_id) WHERE sold = TRUE;
CREATE INDEX IF NOT EXISTS idx_items_sold_at ON items(sold_at) WHERE sold = TRUE;

-- Normalized checkout structure - removed JSONB
CREATE TABLE IF NOT EXISTS checkout_attempts (
    id VARCHAR(255) PRIMARY KEY,
    sale_id VARCHAR(20) NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    checkout_code VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_checkout_user_sale ON checkout_attempts(sale_id, user_id);
CREATE INDEX IF NOT EXISTS idx_checkout_code ON checkout_attempts(checkout_code);

-- Separate table for checkout items (normalized structure)
CREATE TABLE IF NOT EXISTS checkout_items (
    id VARCHAR(255) PRIMARY KEY,
    checkout_attempt_id VARCHAR(255) NOT NULL REFERENCES checkout_attempts(id) ON DELETE CASCADE,
    item_id VARCHAR(255) NOT NULL,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (item_id) REFERENCES items(id),
    UNIQUE(checkout_attempt_id, item_id)
);

CREATE INDEX IF NOT EXISTS idx_checkout_items_attempt ON checkout_items(checkout_attempt_id);
CREATE INDEX IF NOT EXISTS idx_checkout_items_item ON checkout_items(item_id);

-- Purchases: successful purchases only
CREATE TABLE IF NOT EXISTS purchases (
    id VARCHAR(255) PRIMARY KEY,
    sale_id VARCHAR(20) NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    item_id VARCHAR(255) NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    checkout_code VARCHAR(64) NOT NULL,
    purchased_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_purchases_user_sale ON purchases(user_id, sale_id);
CREATE INDEX IF NOT EXISTS idx_purchases_sale ON purchases(sale_id);

-- Idempotency table for purchase operations
CREATE TABLE IF NOT EXISTS purchase_results (
    checkout_code VARCHAR(64) PRIMARY KEY,
    result JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_purchase_results_created ON purchase_results(created_at);