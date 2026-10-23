#!/usr/bin/env bash
# ----------------------------------------------------------------------------
# "Fixes" every planted business exception in the demo ERP database.
# Within one rule cycle Sentinel AUTO-RESOLVES the incidents and sends a
# RESOLVED notification — great for showing the full lifecycle live.
# Run ./demo/scripts/reset-data.sh to plant the exceptions again.
# ----------------------------------------------------------------------------
set -euo pipefail
SQL="
INSERT INTO gl_entries (invoice_no, account, debit, credit)
  SELECT invoice_no, '1200-Receivables', amount, 0 FROM invoices i
  WHERE posted AND NOT EXISTS (SELECT 1 FROM gl_entries g WHERE g.invoice_no = i.invoice_no);
UPDATE inventory SET qty_on_hand = 0 WHERE qty_on_hand < 0;
UPDATE orders SET status = 'SHIPPED', updated_at = now() WHERE status = 'PROCESSING' AND created_at < now() - interval '2 hours';
DELETE FROM payments WHERE reference LIKE 'BANK-DUP-%';
"
if [[ -n "${DEMO_DB_URL:-}" ]]; then psql "$DEMO_DB_URL" -q -c "$SQL"
else docker compose -f docker-compose.demo.yml exec -T appdb psql -U demo -d erp -q -c "$SQL"; fi
echo "Business data corrected — incidents will auto-resolve on the next rule run."
