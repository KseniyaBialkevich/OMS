-- sudo -u postgres psql
-- \c oms_db
-- \d

CREATE DATABASE oms_db;

CREATE TABLE menu (
    id_menu SERIAL PRIMARY KEY, 
    name VARCHAR(100) NOT NULL, 
    price INTEGER NOT NULL
);

INSERT INTO menu  (name, price)
    VALUES
        ('pasta', 12),
        ('steak', 26),
        ('soup', 7),
        ('potatoes', 4),
        ('salad', 10),
        ('water', 1),
        ('coffee', 3);

CREATE TABLE orders (
    id_order SERIAL PRIMARY KEY, 
    status VARCHAR(100) NOT NULL,
    total_cost INTEGER
);

CREATE TABLE orders_to_menu (
    id_order BIGINT NOT NULL, 
    id_menu INTEGER NOT NULL, 
    number INTEGER NOT NULL
);

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO kspsql;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO kspsql;

