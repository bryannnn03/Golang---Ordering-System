-- Initialize Database Schema for Order Management System

CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price NUMERIC(10, 2) NOT NULL,
    stock INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    customer_name VARCHAR(255) NOT NULL,
    customer_email VARCHAR(255) NOT NULL,
    total_amount NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id INT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id INT NOT NULL REFERENCES products(id),
    quantity INT NOT NULL,
    unit_price NUMERIC(10, 2) NOT NULL,
    subtotal NUMERIC(10, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast order lookups by status and customer
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_customer_email ON orders(customer_email);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);

-- Insert Sample Products if table is empty
INSERT INTO products (name, description, price, stock)
SELECT 'Banana (per kg)', 'Fresh and ripe bananas', 12.90, 100
WHERE NOT EXISTS (SELECT 1 FROM products WHERE name = 'Banana (per kg)');

INSERT INTO products (name, description, price, stock)
SELECT 'Grapes (per box)', 'Fresh and juicy grapes', 15.50, 30
WHERE NOT EXISTS (SELECT 1 FROM products WHERE name = 'Grapes (per box)');

INSERT INTO products (name, description, price, stock)
SELECT 'Apple (per kg)', 'Fresh and crisp apples', 8.90, 50
WHERE NOT EXISTS (SELECT 1 FROM products WHERE name = 'Apple (per kg)');

INSERT INTO products (name, description, price, stock)
SELECT 'Pineapple (per kg)', 'Fresh and juicy pineapples', 7.90, 40
WHERE NOT EXISTS (SELECT 1 FROM products WHERE name = 'Pineapple (per kg)');
