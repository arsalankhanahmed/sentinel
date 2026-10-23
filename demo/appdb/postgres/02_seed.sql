-- ============================================================================
-- DEMO seed data. Mostly clean data, plus DELIBERATE business exceptions so that
-- every demo rule has something to find. Each planted exception is marked "<<".
-- Idempotent: truncates and reloads.
-- ============================================================================
TRUNCATE payments, gl_entries, invoices, orders, inventory, products, customers, app_error_log RESTART IDENTITY CASCADE;

INSERT INTO customers (name, city, credit_limit) VALUES
 ('Al-Noor Traders',        'Karachi',    500000),
 ('Punjab Textiles Ltd',    'Lahore',     1200000),
 ('Margalla Pharma',        'Islamabad',  800000),
 ('Indus Steel Works',      'Hyderabad',  2000000),
 ('Khyber Foods',           'Peshawar',   300000),
 ('Quetta Minerals',        'Quetta',     650000),
 ('Sialkot Sports Co',      'Sialkot',    450000),
 ('Faisal Cotton Mills',    'Faisalabad', 1500000);

INSERT INTO products (sku, name, unit_price) VALUES
 ('SKU-1001', 'Cotton Yarn 30s (bag)',      18500),
 ('SKU-1002', 'Polyester Fabric (roll)',    42000),
 ('SKU-1003', 'Paracetamol 500mg (carton)',  9600),
 ('SKU-1004', 'Steel Rebar 12mm (ton)',     265000),
 ('SKU-1005', 'Basmati Rice 25kg',           8900),
 ('SKU-1006', 'Football Size 5 (dozen)',    14400),
 ('SKU-1007', 'Packaging Carton (100)',      5200);

INSERT INTO inventory (sku, warehouse, qty_on_hand) VALUES
 ('SKU-1001','KHI-01', 420), ('SKU-1001','LHE-01', 180),
 ('SKU-1002','LHE-01',  75), ('SKU-1003','ISB-01', 960),
 ('SKU-1004','KHI-01',  -12),              -- << negative stock (oversold)
 ('SKU-1005','KHI-01', 1500), ('SKU-1005','LHE-01', -40), -- << negative stock
 ('SKU-1006','SKT-01', 310), ('SKU-1007','KHI-01', 2200);

-- 40 healthy orders spread over the last 10 days
INSERT INTO orders (customer_id, status, total_amount, created_at, updated_at)
SELECT 1 + (g % 8),
       (ARRAY['SHIPPED','SHIPPED','SHIPPED','NEW','CANCELLED'])[1 + (g % 5)],
       round((50000 + (g * 7919) % 900000)::numeric, 2),
       now() - (g * interval '6 hours'),
       now() - (g * interval '6 hours') + interval '1 hour'
FROM generate_series(1, 40) g;

-- << Orders stuck in PROCESSING for hours (SLA breach)
INSERT INTO orders (customer_id, status, total_amount, created_at, updated_at) VALUES
 (2, 'PROCESSING', 845000, now() - interval '5 hours',  now() - interval '5 hours'),
 (4, 'PROCESSING', 1325000, now() - interval '3 hours', now() - interval '3 hours'),
 (7, 'PROCESSING', 172800, now() - interval '9 hours',  now() - interval '9 hours');
-- a PROCESSING order that is still within SLA (must NOT be flagged)
INSERT INTO orders (customer_id, status, total_amount, created_at, updated_at) VALUES
 (3, 'PROCESSING', 96000, now() - interval '20 minutes', now() - interval '20 minutes');

-- Invoices for shipped orders, all posted with GL entries ...
INSERT INTO invoices (invoice_no, order_id, amount, posted, posted_at)
SELECT 'INV-' || lpad(order_id::text, 5, '0'), order_id, total_amount, TRUE, updated_at
FROM orders WHERE status = 'SHIPPED';

INSERT INTO gl_entries (invoice_no, account, debit, credit, created_at)
SELECT invoice_no, '1200-Receivables', amount, 0, posted_at FROM invoices;
INSERT INTO gl_entries (invoice_no, account, debit, credit, created_at)
SELECT invoice_no, '4000-Sales', 0, amount, posted_at FROM invoices;

-- << ... except two invoices posted WITHOUT any GL entry (integration failure)
INSERT INTO invoices (invoice_no, order_id, amount, posted, posted_at) VALUES
 ('INV-90001', 4, 275000, TRUE, now() - interval '2 hours'),
 ('INV-90002', 9, 1180000, TRUE, now() - interval '40 minutes');

-- Payments (healthy)
INSERT INTO payments (invoice_no, amount, reference, paid_at)
SELECT invoice_no, amount, 'BANK-' || substr(md5(invoice_no), 1, 8), posted_at + interval '2 days'
FROM invoices WHERE invoice_no NOT IN ('INV-90001','INV-90002');

-- << Duplicate payment: same invoice paid twice
INSERT INTO payments (invoice_no, amount, reference, paid_at)
SELECT invoice_no, amount, 'BANK-DUP-7781', now() - interval '1 hour'
FROM invoices WHERE invoice_no NOT IN ('INV-90001','INV-90002') ORDER BY invoice_no LIMIT 1;

-- Historical app log rows (INFO/WARN + an old ERROR). Sentinel starts watching
-- from the current max id, so history is NOT re-alerted.
INSERT INTO app_error_log (app_name, level, message, user_name, logged_at) VALUES
 ('OrderPortal', 'INFO',  'Application started',                        'system', now() - interval '1 day'),
 ('OrderPortal', 'WARN',  'Slow query: GetOrderHistory took 2140 ms',   'ahmed',  now() - interval '20 hours'),
 ('BillingSvc',  'ERROR', 'Timeout calling bank gateway (historical)',  'system', now() - interval '18 hours'),
 ('OrderPortal', 'INFO',  'Nightly job completed',                      'system', now() - interval '6 hours');
