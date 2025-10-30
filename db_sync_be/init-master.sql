-- Init script for Master Database
USE master_db;

-- Tabel Users
CREATE TABLE IF NOT EXISTS users (
  id INT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(100) NOT NULL,
  email VARCHAR(100) NOT NULL UNIQUE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_id (id),
  INDEX idx_email (email)
);

-- Tabel Products
CREATE TABLE IF NOT EXISTS products (
  id INT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(200) NOT NULL,
  description TEXT,
  price DECIMAL(10, 2) NOT NULL,
  stock INT DEFAULT 0,
  category VARCHAR(50),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_id (id),
  INDEX idx_category (category)
);

-- Tabel Orders
CREATE TABLE IF NOT EXISTS orders (
  id INT PRIMARY KEY AUTO_INCREMENT,
  user_id INT NOT NULL,
  total_amount DECIMAL(10, 2) NOT NULL,
  status ENUM('pending', 'processing', 'completed', 'cancelled') DEFAULT 'pending',
  order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_id (id),
  INDEX idx_user_id (user_id),
  INDEX idx_status (status)
);

-- Insert sample data untuk Users
INSERT INTO users (name, email) VALUES
  ('John Doe', 'john@example.com'),
  ('Jane Smith', 'jane@example.com'),
  ('Bob Johnson', 'bob@example.com'),
  ('Alice Williams', 'alice@example.com'),
  ('Charlie Brown', 'charlie@example.com'),
  ('Diana Prince', 'diana@example.com'),
  ('Edward Norton', 'edward@example.com'),
  ('Fiona Apple', 'fiona@example.com'),
  ('George Martin', 'george@example.com'),
  ('Hannah Montana', 'hannah@example.com');

-- Insert sample data untuk Products
INSERT INTO products (name, description, price, stock, category) VALUES
  ('Laptop Dell XPS 13', 'Powerful ultrabook', 15000000, 10, 'Electronics'),
  ('iPhone 15 Pro', 'Latest Apple smartphone', 18000000, 25, 'Electronics'),
  ('Nike Air Max', 'Comfortable running shoes', 1500000, 50, 'Fashion'),
  ('Coffee Maker', 'Automatic coffee machine', 2500000, 15, 'Home Appliances'),
  ('Gaming Mouse', 'RGB gaming mouse', 500000, 100, 'Electronics'),
  ('Office Chair', 'Ergonomic office chair', 3000000, 20, 'Furniture'),
  ('Wireless Headphones', 'Noise cancelling', 2000000, 30, 'Electronics'),
  ('Smart Watch', 'Fitness tracker', 3500000, 40, 'Electronics'),
  ('Backpack', 'Travel backpack', 750000, 60, 'Fashion'),
  ('Water Bottle', 'Insulated bottle', 250000, 200, 'Sports');

-- Insert sample data untuk Orders
INSERT INTO orders (user_id, total_amount, status) VALUES
  (1, 15000000, 'completed'),
  (2, 18000000, 'completed'),
  (1, 1500000, 'processing'),
  (3, 2500000, 'pending'),
  (4, 500000, 'completed'),
  (5, 3000000, 'processing'),
  (2, 2000000, 'completed'),
  (6, 3500000, 'pending'),
  (7, 750000, 'completed'),
  (8, 250000, 'cancelled');

-- Log
SELECT 'Master database initialized successfully' AS message;
SELECT 
  (SELECT COUNT(*) FROM users) AS total_users,
  (SELECT COUNT(*) FROM products) AS total_products,
  (SELECT COUNT(*) FROM orders) AS total_orders;
