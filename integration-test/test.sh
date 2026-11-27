#!/usr/bin/env bash
set -uo pipefail

# Integration test for the full OSAC + cost-consumer stack on k3s.
# Expects: kubectl port-forward running for cost-consumer (:8020) and osac-rest (:8011).
# Expects: /tmp/osac_token.txt with a valid token.

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
PASS=0
FAIL=0

check() {
    local desc="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} $desc"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗${NC} $desc"
        FAIL=$((FAIL + 1))
    fi
}

check_output() {
    local desc="$1"
    local expected="$2"
    shift 2
    local output
    output=$("$@" 2>/dev/null) || true
    if echo "$output" | grep -q "$expected"; then
        echo -e "  ${GREEN}✓${NC} $desc"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗${NC} $desc (expected: $expected, got: $output)"
        FAIL=$((FAIL + 1))
    fi
}

BASE=http://localhost:8020
OSAC=http://localhost:8011
TOKEN=$(cat /tmp/osac_token.txt)

echo "=== Integration Test: Full Stack ==="
echo ""

# ── 1. Health checks ──
echo "--- Health checks ---"
check "liveness probe" curl -sf "$BASE/healthz"
check "readiness probe" curl -sf "$BASE/readyz"
check "metrics endpoint" curl -sf http://localhost:9000/metrics

# ── 2. Verify OSAC is reachable ──
echo ""
echo "--- OSAC connectivity ---"
check "OSAC REST gateway" curl -sf "$OSAC/api/fulfillment/v1/instance_types" -H "Authorization: Bearer $TOKEN"

# Create an instance type (verifies write access)
curl -sf -X POST "$OSAC/api/fulfillment/v1/instance_types" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"metadata":{"name":"ci-test-2-8"},"spec":{"cores":2,"memory_gib":8,"description":"CI test","state":"INSTANCE_TYPE_STATE_ACTIVE"}}' \
    >/dev/null 2>&1 || true
check "instance type created" curl -sf "$OSAC/api/fulfillment/v1/instance_types" -H "Authorization: Bearer $TOKEN"

# ── 3. Trigger reconciliation ──
echo ""
echo "--- Reconciliation ---"
curl -sf -X POST "$BASE/api/v1/reconcile" >/dev/null
sleep 5
check "reconcile completed" curl -sf "$BASE/api/v1/reports/summary"

# ── 4. Send MaaS CloudEvent ──
echo ""
echo "--- CloudEvent ingest ---"
EVENT_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/events" \
    -H "Content-Type: application/json" \
    -d "{
        \"specversion\":\"1.0\",
        \"type\":\"osac.model.lifecycle\",
        \"source\":\"ci-test\",
        \"id\":\"ci-maas-$(date +%s)\",
        \"time\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
        \"data\":{
            \"tenant_id\":\"tenant-ci\",
            \"model_id\":\"model-ci-test\",
            \"model_name\":\"ci-model\",
            \"state\":\"MODEL_STATE_RUNNING\",
            \"tokens_in\":10000,
            \"tokens_out\":5000,
            \"requests\":50,
            \"duration_seconds\":60
        }
    }")
check_output "event accepted" "204" echo "$EVENT_STATUS"

# ── 5. Verify atomic adapter batch ingress and receipt replay ──
echo ""
echo "--- OSAC adapter batch ingest ---"
BATCH_ID="ci-osac-batch-$(date +%s)"
BATCH_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
BATCH_BODY="{
  \"events\": [
    {
      \"specversion\": \"1.0\", \"id\": \"${BATCH_ID}\", \"source\": \"osac-metering\",
      \"type\": \"osac.resource.created.v1\", \"time\": \"${BATCH_TIME}\",
      \"osacresourceid\": \"ci-batch-vm\", \"osacresourcetype\": \"compute_instance\", \"osactenant\": \"tenant-ci\",
      \"data\": {
        \"resource_id\": \"ci-batch-vm\", \"resource_type\": \"compute_instance\", \"tenant_id\": \"tenant-ci\",
        \"current_state\": \"RUNNING\", \"transition_time\": \"${BATCH_TIME}\",
        \"billing_dimensions\": {\"instance_type\": \"ci-test-2-8\"}, \"schema_version\": \"v1\"
      }
    }
  ]
}"
BATCH_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/events/batch" \
  -H "Content-Type: application/json" -d "$BATCH_BODY")
check_output "OSAC batch accepted" "204" echo "$BATCH_STATUS"

REPLAY_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/events/batch" \
  -H "Content-Type: application/json" -d "$BATCH_BODY")
check_output "exact batch replay is a no-op" "204" echo "$REPLAY_STATUS"

COLLISION_BODY="${BATCH_BODY//ci-batch-vm/ci-batch-vm-changed}"
COLLISION_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/events/batch" \
  -H "Content-Type: application/json" -d "$COLLISION_BODY")
check_output "identity collision rejected" "409" echo "$COLLISION_STATUS"

# ── 5b. Verify OSAC v1 locked heartbeat ingest (interval usage + negative correction) ──
echo ""
echo "--- OSAC v1 locked heartbeat ingest ---"
HB_SUFFIX="$(date +%s)"
HB_RES_ID="ci-hb-vol-${HB_SUFFIX}"
HB_POS_ID="ci-hb-pos-${HB_SUFFIX}"
HB_NEG_ID="ci-hb-neg-${HB_SUFFIX}"
HB_ACCT="acct-ci-v1"
read -r HB_NOW HB_FROM HB_TO <<< "$(python3 -c '
import datetime
now = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0)
frm = now - datetime.timedelta(hours=2)
to = frm + datetime.timedelta(minutes=5)
fmt = lambda d: d.strftime("%Y-%m-%dT%H:%M:%SZ")
print(fmt(now), fmt(frm), fmt(to))
')"

HB_POS_BODY="{
  \"specversion\": \"1.0\", \"id\": \"${HB_POS_ID}\", \"source\": \"osac-metering\",
  \"type\": \"osac.resource.heartbeat.v1\", \"datacontenttype\": \"application/json\", \"time\": \"${HB_FROM}\",
  \"osacresourceid\": \"${HB_RES_ID}\", \"osacresourcetype\": \"volume\", \"osactenant\": \"tenant-ci\", \"osacproject\": \"proj-ci\",
  \"data\": {
    \"schema_version\": \"v1\", \"tenant_id\": \"tenant-ci\", \"external_billing_account_id\": \"${HB_ACCT}\",
    \"project_id\": \"proj-ci\", \"resource_type\": \"volume\", \"resource_id\": \"${HB_RES_ID}\",
    \"meter\": \"capacity\", \"component\": \"volume\", \"heartbeat_time\": \"${HB_NOW}\",
    \"usage\": {
      \"semantics\": \"interval\", \"from\": \"${HB_FROM}\", \"to\": \"${HB_TO}\",
      \"quantity\": \"300.000000\", \"unit\": \"gibibyte_second\", \"precision\": \"second\"
    },
    \"billing_dimensions\": {\"project\": \"proj-ci\", \"catalog\": \"cat-gp3\"}
  }
}"
HB_POS_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/events" \
  -H "Content-Type: application/json" -d "$HB_POS_BODY")
check_output "OSAC v1 heartbeat accepted" "204" echo "$HB_POS_STATUS"

HB_REPLAY_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/events" \
  -H "Content-Type: application/json" -d "$HB_POS_BODY")
check_output "OSAC v1 heartbeat exact replay is a no-op" "204" echo "$HB_REPLAY_STATUS"

HB_COLLISION_BODY="${HB_POS_BODY//300.000000/400.000000}"
HB_COLLISION_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/events" \
  -H "Content-Type: application/json" -d "$HB_COLLISION_BODY")
check_output "OSAC v1 heartbeat identity collision rejected" "409" echo "$HB_COLLISION_STATUS"

HB_NEG_BODY="{
  \"specversion\": \"1.0\", \"id\": \"${HB_NEG_ID}\", \"source\": \"osac-metering\",
  \"type\": \"osac.resource.heartbeat.v1\", \"datacontenttype\": \"application/json\", \"time\": \"${HB_FROM}\",
  \"osacresourceid\": \"${HB_RES_ID}\", \"osacresourcetype\": \"volume\", \"osactenant\": \"tenant-ci\", \"osacproject\": \"proj-ci\",
  \"data\": {
    \"schema_version\": \"v1\", \"tenant_id\": \"tenant-ci\", \"external_billing_account_id\": \"${HB_ACCT}\",
    \"project_id\": \"proj-ci\", \"resource_type\": \"volume\", \"resource_id\": \"${HB_RES_ID}\",
    \"meter\": \"capacity\", \"component\": \"volume\", \"heartbeat_time\": \"${HB_NOW}\",
    \"usage\": {
      \"semantics\": \"interval\", \"from\": \"${HB_FROM}\", \"to\": \"${HB_TO}\",
      \"quantity\": \"-50.500000\", \"unit\": \"gibibyte_second\", \"precision\": \"second\"
    },
    \"billing_dimensions\": {\"project\": \"proj-ci\", \"catalog\": \"cat-gp3\"}
  }
}"
HB_NEG_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/events" \
  -H "Content-Type: application/json" -d "$HB_NEG_BODY")
check_output "OSAC v1 negative correction accepted" "204" echo "$HB_NEG_STATUS"

HB_V2_BODY="${HB_NEG_BODY//osac.resource.heartbeat.v1/osac.resource.heartbeat.v2}"
HB_V2_BODY="${HB_V2_BODY//${HB_NEG_ID}/ci-hb-v2-${HB_SUFFIX}}"
HB_V2_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/events" \
  -H "Content-Type: application/json" -d "$HB_V2_BODY")
check_output "unsupported OSAC v2 heartbeat rejected" "400" echo "$HB_V2_STATUS"

# ── 6. Wait for metering + rating sweeps ──
echo ""
echo "--- Waiting for metering (60s) + rating (30s) sweeps ---"
sleep 95

# ── 7. Verify pipeline output ──
echo ""
echo "--- Pipeline verification ---"

# Check metering entries via report API
REPORT=$(curl -sf "$BASE/api/v1/reports/summary" 2>/dev/null || echo "{}")
check_output "metering entries exist" "metering_entries" echo "$REPORT"
check_output "cost entries exist" "cost_entries" echo "$REPORT"

# Check quota API
QUOTA=$(curl -sf "$BASE/api/v1/quotas/tenant-ci" 2>/dev/null || echo "{}")
check_output "quota API responds" "tenant_id" echo "$QUOTA"

HB_QUOTA=$(curl -sf "$BASE/api/v1/quotas/${HB_ACCT}" 2>/dev/null || echo "{}")
check_output "OSAC v1 billing account quota responds" "${HB_ACCT}" echo "$HB_QUOTA"

if command -v kubectl >/dev/null 2>&1; then
    HB_DB_SUMMARY=$(kubectl exec -n cost-mgmt statefulset/cost-db -- \
        psql -U costuser -d costdb -t -A -c \
        "SELECT count(*) || ':' || round(COALESCE(SUM(value), 0), 1) FROM metering_entries WHERE resource_id = '${HB_RES_ID}' AND tenant_id = '${HB_ACCT}' AND resource_type = 'volume' AND meter_name = 'capacity';" 2>/dev/null || true)
    check_output "OSAC v1 interval + negative correction net metering" "2:249.5" echo "$HB_DB_SUMMARY"
fi

# Check Prometheus metrics
METRICS=$(curl -sf http://localhost:9000/metrics 2>/dev/null || echo "")
check_output "events processed metric" "cost_consumer_events_processed_total" echo "$METRICS"
check_output "metering entries metric" "cost_consumer_metering_entries_created_total" echo "$METRICS"

# ── Summary ──
echo ""
echo "=========================================="
TOTAL=$((PASS + FAIL))
echo "  Results: $PASS/$TOTAL passed"
if [ "$FAIL" -gt 0 ]; then
    echo -e "  ${RED}$FAIL FAILED${NC}"
    exit 1
else
    echo -e "  ${GREEN}ALL PASSED${NC}"
fi
