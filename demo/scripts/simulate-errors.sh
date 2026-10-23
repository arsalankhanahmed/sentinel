#!/usr/bin/env bash
# ----------------------------------------------------------------------------
# Simulates a live application writing to its OWN error-log table, so the
# zero-code Log Watcher (Module B) raises runtime-exception incidents on stage.
#
#   ./demo/scripts/simulate-errors.sh            # one burst of 3 errors
#   ./demo/scripts/simulate-errors.sh --loop 20  # a burst every 20 seconds
#
# Uses DEMO_DB_URL if set, otherwise the appdb container from docker-compose.demo.yml.
# ----------------------------------------------------------------------------
set -euo pipefail

psql_exec() {
  if [[ -n "${DEMO_DB_URL:-}" ]]; then
    psql "$DEMO_DB_URL" -v ON_ERROR_STOP=1 -q -c "$1"
  else
    docker compose -f docker-compose.demo.yml exec -T appdb psql -U demo -d erp -v ON_ERROR_STOP=1 -q -c "$1"
  fi
}

ERRORS=(
  "OrderPortal|ERROR|System.NullReferenceException: Object reference not set to an instance of an object.|at OrderPortal.Checkout.Submit() in Checkout.aspx.cs:line 214"
  "OrderPortal|ERROR|System.Data.SqlClient.SqlException: Transaction (Process ID 87) was deadlocked on lock resources with another process and has been chosen as the deadlock victim.|at OrderRepository.Save()"
  "BillingSvc|FATAL|Bank gateway returned HTTP 502 for settlement batch 7781|at Billing.Gateway.PostBatch()"
  "BillingSvc|ERROR|System.TimeoutException: The operation has timed out after 30000 ms calling https://bank.example/api/settle|at HttpClientWrapper.Post()"
  "InventorySync|ERROR|Duplicate key value violates unique constraint pk_inventory (SKU-1004, KHI-01)|at InventorySync.Apply()"
  "OrderPortal|WARN|Session state size 1.8 MB exceeds recommended 1 MB|"
)
USERS=(ahmed sara bilal fatima system)

burst() {
  local sql="INSERT INTO app_error_log (app_name, level, message, stack_trace, user_name) VALUES "
  local first=1
  for _ in 1 2 3; do
    IFS='|' read -r app lvl msg st <<<"${ERRORS[RANDOM % ${#ERRORS[@]}]}"
    usr=${USERS[RANDOM % ${#USERS[@]}]}
    msg=${msg//\'/\'\'}; st=${st//\'/\'\'}
    [[ $first -eq 0 ]] && sql+=","
    sql+="('$app','$lvl','$msg',NULLIF('$st',''),'$usr')"
    first=0
  done
  psql_exec "$sql"
  echo "$(date +%H:%M:%S)  inserted 3 log rows (ERROR/FATAL rows will alert; WARN is filtered out)"
}

if [[ "${1:-}" == "--loop" ]]; then
  every=${2:-20}
  while true; do burst; sleep "$every"; done
else
  burst
fi
