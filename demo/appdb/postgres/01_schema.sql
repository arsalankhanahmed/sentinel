-- ============================================================================
-- DEMO "ERP" application database (PostgreSQL)
-- This simulates a customer's production app that Sentinel monitors WITHOUT
-- any code change. Sentinel only needs a read-only login.
-- ============================================================================

CREATE TABLE IF NOT EXISTS customers (
    customer_id   SERIAL PRIMARY KEY,
    name          TEXT        NOT NULL,
    city          TEXT        NOT NULL,
    credit_limit  NUMERIC(14,2) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS products (
    sku           TEXT PRIMARY KEY,
    name          TEXT          NOT NULL,
    unit_price    NUMERIC(12,2) NOT NULL
);

CREATE TABLE IF NOT EXISTS inventory (
    sku           TEXT REFERENCES products(sku),
    warehouse     TEXT NOT NULL,
    qty_on_hand   INT  NOT NULL,
    PRIMARY KEY (sku, warehouse)
);

CREATE TABLE IF NOT EXISTS orders (
    order_id      SERIAL PRIMARY KEY,
    customer_id   INT REFERENCES customers(customer_id),
    status        TEXT NOT NULL CHECK (status IN ('NEW','PROCESSING','SHIPPED','CANCELLED')),
    total_amount  NUMERIC(14,2) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invoices (
    invoice_no    TEXT PRIMARY KEY,
    order_id      INT REFERENCES orders(order_id),
    amount        NUMERIC(14,2) NOT NULL,
    posted        BOOLEAN NOT NULL DEFAULT FALSE,
    posted_at     TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS gl_entries (
    entry_id      SERIAL PRIMARY KEY,
    invoice_no    TEXT REFERENCES invoices(invoice_no),
    account       TEXT NOT NULL,
    debit         NUMERIC(14,2) NOT NULL DEFAULT 0,
    credit        NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payments (
    payment_id    SERIAL PRIMARY KEY,
    invoice_no    TEXT REFERENCES invoices(invoice_no),
    amount        NUMERIC(14,2) NOT NULL,
    reference     TEXT NOT NULL,
    paid_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The application's OWN error log table (written by the app's existing logger,
-- e.g. log4net / NLog / Serilog DB sink). Sentinel's Log Watcher reads it.
CREATE TABLE IF NOT EXISTS app_error_log (
    log_id        BIGSERIAL PRIMARY KEY,
    app_name      TEXT NOT NULL,
    level         TEXT NOT NULL,
    message       TEXT NOT NULL,
    stack_trace   TEXT,
    user_name     TEXT,
    logged_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Least-privilege monitoring login (what you would give Sentinel in production).
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sentinel_ro') THEN
    CREATE ROLE sentinel_ro LOGIN PASSWORD 'sentinel_ro';
  END IF;
END $$;
GRANT USAGE ON SCHEMA public TO sentinel_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO sentinel_ro;
