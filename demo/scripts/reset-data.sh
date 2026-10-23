#!/usr/bin/env bash
# Reloads the demo ERP seed (re-plants all business exceptions).
set -euo pipefail
DIR="$(cd "$(dirname "$0")/../appdb/postgres" && pwd)"
if [[ -n "${DEMO_DB_URL:-}" ]]; then psql "$DEMO_DB_URL" -q -f "$DIR/02_seed.sql"
else docker compose -f docker-compose.demo.yml exec -T appdb psql -U demo -d erp -q < "$DIR/02_seed.sql"; fi
echo "Demo ERP data reset — exceptions re-planted."
