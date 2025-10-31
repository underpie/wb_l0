-- init.sql: create table for orders
CREATE TABLE IF NOT EXISTS orders (
  order_uid TEXT PRIMARY KEY,
  data JSONB NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
