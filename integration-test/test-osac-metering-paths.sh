#!/usr/bin/env bash
set -uo pipefail

# End-to-end integration test covering BOTH OSAC metering integration paths
# independently against live Kafka + PostgreSQL:
#
#   Path 1 (In-tree OSAC adapter -> HTTP):
#     Kafka (osac.metering.heartbeat.<tenant> + osac.metering.inference)
#       -> OSAC cost-management-adapter (osac-project/osac#2661)
#       -> HTTP POST /api/v1/events
#       -> cost-event-consumer (costdb_adapter)
#       (rejected records routed to Kafka osac.metering.dlq.<tenant>)
#
#   Path 2 (Direct external Kafka reader):
#     Kafka (osac.metering.heartbeat.<tenant> + osac.metering.inference)
#       -> cost-event-consumer (KAFKA_READER_MODE=osac-reader, costdb_direct)
#       (rejected records persisted to PostgreSQL kafka_dead_letters)
#
# Prerequisites:
#   - Kafka reachable on $BROKER (default 127.0.0.1:19092)
#   - Postgres reachable on 127.0.0.1:$DB_PORT (default 5434) with DBs
#     costdb_adapter and costdb_direct
#   - Built cost-event-consumer at $WATCHER_BIN
#   - Built OSAC cost-management-adapter at $ADAPTER_BIN

GREEN='\033[0;32m'
RED='\033[0;31m'
DIM='\033[2m'
NC='\033[0m'
PASS=0
FAIL=0

check() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        echo -e "  ${GREEN}✓${NC} $desc ${DIM}(got: $actual)${NC}"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗${NC} $desc ${DIM}(expected: $expected, got: $actual)${NC}"
        FAIL=$((FAIL + 1))
    fi
}

check_ge() {
    local desc="$1" minimum="$2" actual="$3"
    if [ "$actual" -ge "$minimum" ] 2>/dev/null; then
        echo -e "  ${GREEN}✓${NC} $desc ${DIM}(got: $actual, min: $minimum)${NC}"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}✗${NC} $desc ${DIM}(expected >= $minimum, got: $actual)${NC}"
        FAIL=$((FAIL + 1))
    fi
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
WATCHER_BIN="${WATCHER_BIN:-$REPO_DIR/inventory-watcher/inventory-watcher}"
ADAPTER_BIN="${ADAPTER_BIN:-/tmp/cost-management-adapter-bin}"
BROKER="${KAFKA_BROKER_OVERRIDE:-127.0.0.1:19092}"
DB_PORT="${DB_PORT:-5434}"
DB_ADAPTER="${DB_ADAPTER:-costdb_adapter}"
DB_DIRECT="${DB_DIRECT:-costdb_direct}"

PIDS=()
cleanup() {
    for pid in "${PIDS[@]:-}"; do
        [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
    done
    for pid in "${PIDS[@]:-}"; do
        [ -n "$pid" ] && wait "$pid" 2>/dev/null || true
    done
}
trap cleanup EXIT

db_query() {
    local db="$1" sql="$2"
    PGPASSWORD=pass psql -h 127.0.0.1 -p "$DB_PORT" -U user -d "$db" -t -A -c "$sql" 2>/dev/null
}

kafka_produce() {
    local topic="$1" payload="$2"
    if command -v rpk > /dev/null 2>&1; then
        printf '%s\n' "$payload" | rpk topic produce "$topic" --brokers "$BROKER" > /dev/null 2>&1
    else
        printf '%s\n' "$payload" | docker exec -i kafka /opt/kafka/bin/kafka-console-producer.sh \
            --bootstrap-server "$BROKER" --topic "$topic" > /dev/null 2>&1
    fi
}

kafka_consume() {
    local topic="$1" max_msgs="${2:-10}"
    if command -v rpk > /dev/null 2>&1; then
        timeout 10 rpk topic consume "$topic" --brokers "$BROKER" -o start -f '%v\n' -n "$max_msgs" 2>/dev/null || true
    else
        timeout 10 docker exec kafka /opt/kafka/bin/kafka-console-consumer.sh \
            --bootstrap-server "$BROKER" --topic "$topic" \
            --from-beginning --max-messages "$max_msgs" --timeout-ms 5000 2>/dev/null || true
    fi
}

echo "=== OSAC Metering Paths Integration Test (Path 1 Adapter + Path 2 Direct Reader) ==="
echo "  Kafka:   $BROKER"
echo "  Adapter: $ADAPTER_BIN"
echo "  Consumer: $WATCHER_BIN"
echo ""

# ── Preflight ──
echo "--- Preflight ---"
[ -x "$WATCHER_BIN" ] || { echo "ERROR: consumer binary not found at $WATCHER_BIN"; exit 1; }
[ -x "$ADAPTER_BIN" ] || { echo "ERROR: adapter binary not found at $ADAPTER_BIN"; exit 1; }
db_query "$DB_ADAPTER" "SELECT 1" > /dev/null || { echo "ERROR: $DB_ADAPTER unreachable"; exit 1; }
db_query "$DB_DIRECT" "SELECT 1" > /dev/null || { echo "ERROR: $DB_DIRECT unreachable"; exit 1; }
echo "  Binaries & DBs: OK"

# ── Produce shared OSAC Kafka test corpus ──
echo ""
echo "--- Producing OSAC metering events to Kafka ---"
SUFFIX="$(date +%s)"
RES_ID="osac-e2e-vm-${SUFFIX}"
INF_ID="osac-e2e-llama-${SUFFIX}"
POS_EVT="e2e-pos-${SUFFIX}"
NEG_EVT="e2e-neg-${SUFFIX}"
V2_EVT="e2e-v2-${SUFFIX}"
SYS_EVT="e2e-sys-${SUFFIX}"
SHD_EVT="e2e-shd-${SUFFIX}"
INF_EVT="e2e-inf-${SUFFIX}"
BILLING_ACCT="acct-osac-e2e"

read -r NOW_TS FROM_TS TO_TS BACK_FROM BACK_TO <<< "$(python3 -c '
import datetime
now = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0)
frm = now - datetime.timedelta(minutes=5)
back = now - datetime.timedelta(hours=48)
back_to = back + datetime.timedelta(minutes=5)
fmt = lambda d: d.strftime("%Y-%m-%dT%H:%M:%SZ")
print(fmt(now), fmt(frm), fmt(now), fmt(back), fmt(back_to))
')"

POS_JSON="{\"specversion\":\"1.0\",\"id\":\"${POS_EVT}\",\"source\":\"osac-metering\",\"type\":\"osac.resource.heartbeat.v1\",\"datacontenttype\":\"application/json\",\"time\":\"${FROM_TS}\",\"osacresourceid\":\"${RES_ID}\",\"osacresourcetype\":\"compute_instance\",\"osactenant\":\"tenant-e2e\",\"osacproject\":\"proj-e2e\",\"data\":{\"tenant_id\":\"tenant-e2e\",\"external_billing_account_id\":\"${BILLING_ACCT}\",\"project_id\":\"proj-e2e\",\"resource_type\":\"compute_instance\",\"resource_id\":\"${RES_ID}\",\"meter\":\"cpu_cores\",\"component\":\"compute\",\"heartbeat_time\":\"${NOW_TS}\",\"last_applied_offset\":11,\"rules_revision\":\"rev-1\",\"billing_dimensions\":{\"instance_type\":\"standard-4-8\",\"project\":\"proj-e2e\",\"catalog\":\"cat-rhel-9\"},\"usage\":{\"semantics\":\"interval\",\"from\":\"${FROM_TS}\",\"to\":\"${TO_TS}\",\"quantity\":\"3600.000000\",\"unit\":\"core_second\",\"precision\":\"second\"}}}"
V2_JSON="{\"specversion\":\"1.0\",\"id\":\"${V2_EVT}\",\"source\":\"osac-metering\",\"type\":\"osac.resource.heartbeat.v2\",\"datacontenttype\":\"application/json\",\"time\":\"${FROM_TS}\",\"osacresourceid\":\"${RES_ID}\",\"osacresourcetype\":\"compute_instance\",\"osactenant\":\"tenant-e2e\",\"data\":{\"tenant_id\":\"tenant-e2e\",\"resource_type\":\"compute_instance\",\"resource_id\":\"${RES_ID}\"}}"
NEG_JSON="{\"specversion\":\"1.0\",\"id\":\"${NEG_EVT}\",\"source\":\"osac-metering\",\"type\":\"osac.resource.heartbeat.v1\",\"datacontenttype\":\"application/json\",\"time\":\"${BACK_FROM}\",\"osacresourceid\":\"${RES_ID}\",\"osacresourcetype\":\"compute_instance\",\"osactenant\":\"tenant-e2e\",\"osacproject\":\"proj-e2e\",\"data\":{\"tenant_id\":\"tenant-e2e\",\"external_billing_account_id\":\"${BILLING_ACCT}\",\"project_id\":\"proj-e2e\",\"resource_type\":\"compute_instance\",\"resource_id\":\"${RES_ID}\",\"meter\":\"cpu_cores\",\"component\":\"compute\",\"heartbeat_time\":\"${NOW_TS}\",\"last_applied_offset\":12,\"rules_revision\":\"rev-1\",\"billing_dimensions\":{\"instance_type\":\"standard-4-8\",\"project\":\"proj-e2e\",\"catalog\":\"cat-rhel-9\"},\"usage\":{\"semantics\":\"interval\",\"from\":\"${BACK_FROM}\",\"to\":\"${BACK_TO}\",\"quantity\":\"-600.500000\",\"unit\":\"core_second\",\"precision\":\"second\"}}}"
SYS_JSON="{\"specversion\":\"1.0\",\"id\":\"${SYS_EVT}\",\"source\":\"osac-metering\",\"type\":\"osac.resource.heartbeat.v1\",\"datacontenttype\":\"application/json\",\"time\":\"${FROM_TS}\",\"osacresourceid\":\"sys-vm-${SUFFIX}\",\"osacresourcetype\":\"compute_instance\",\"osactenant\":\"system\",\"data\":{\"tenant_id\":\"system\",\"resource_type\":\"compute_instance\",\"resource_id\":\"sys-vm-${SUFFIX}\",\"meter\":\"cpu_cores\",\"component\":\"compute\",\"heartbeat_time\":\"${NOW_TS}\",\"usage\":{\"semantics\":\"interval\",\"from\":\"${FROM_TS}\",\"to\":\"${TO_TS}\",\"quantity\":\"100.000000\",\"unit\":\"core_second\",\"precision\":\"second\"}}}"
SHD_JSON="{\"specversion\":\"1.0\",\"id\":\"${SHD_EVT}\",\"source\":\"osac-metering\",\"type\":\"osac.resource.heartbeat.v1\",\"datacontenttype\":\"application/json\",\"time\":\"${FROM_TS}\",\"osacresourceid\":\"shd-vm-${SUFFIX}\",\"osacresourcetype\":\"compute_instance\",\"osactenant\":\"shared\",\"data\":{\"tenant_id\":\"shared\",\"resource_type\":\"compute_instance\",\"resource_id\":\"shd-vm-${SUFFIX}\",\"meter\":\"cpu_cores\",\"component\":\"compute\",\"heartbeat_time\":\"${NOW_TS}\",\"usage\":{\"semantics\":\"interval\",\"from\":\"${FROM_TS}\",\"to\":\"${TO_TS}\",\"quantity\":\"100.000000\",\"unit\":\"core_second\",\"precision\":\"second\"}}}"
INF_JSON="{\"specversion\":\"1.0\",\"id\":\"${INF_EVT}\",\"source\":\"osac-metering\",\"type\":\"osac.inference.usage.v1\",\"datacontenttype\":\"application/json\",\"time\":\"${NOW_TS}\",\"osacresourceid\":\"${INF_ID}\",\"osacresourcetype\":\"maas_inference\",\"osactenant\":\"tenant-e2e\",\"data\":{\"tenant_id\":\"tenant-e2e\",\"external_billing_account_id\":\"${BILLING_ACCT}\",\"project_id\":\"proj-e2e\",\"resource_type\":\"maas_inference\",\"resource_id\":\"${INF_ID}\",\"meter\":\"inference\",\"component\":\"tokens\",\"billing_dimensions\":{\"model\":\"llama-3\",\"project\":\"proj-e2e\"},\"usage\":{\"semantics\":\"delta\",\"quantity\":\"2500\",\"unit\":\"tokens\"}}}"

kafka_produce osac.metering.heartbeat.tenant-e2e "$POS_JSON"
kafka_produce osac.metering.heartbeat.tenant-e2e "$V2_JSON"
kafka_produce osac.metering.heartbeat.tenant-e2e "$NEG_JSON"
kafka_produce osac.metering.heartbeat.system "$SYS_JSON"
kafka_produce osac.metering.heartbeat.shared "$SHD_JSON"
kafka_produce osac.metering.inference "$INF_JSON"
echo "  Produced 6 OSAC events across tenant-e2e, system, shared, and inference topics"

# ──────────────────────────────────────────────────────────────────────
# Path 1: OSAC cost-management-adapter -> HTTP -> cost-event-consumer
# ──────────────────────────────────────────────────────────────────────
echo ""
echo "--- Path 1: OSAC cost-management-adapter -> HTTP -> cost-event-consumer ---"

INVENTORY_DB_URL="postgres://user:pass@127.0.0.1:${DB_PORT}/${DB_ADAPTER}" \
INGEST_LISTEN_ADDR="127.0.0.1:18031" \
METRICS_PORT="19031" \
DISABLE_COMPONENTS="watcher,reconciler,metering,rating" \
"$WATCHER_BIN" > /tmp/path1-consumer.log 2>&1 &
P1_CONSUMER_PID=$!
PIDS+=("$P1_CONSUMER_PID")

for i in $(seq 1 30); do
    curl -sf http://127.0.0.1:18031/readyz > /dev/null 2>&1 && break
    sleep 1
done
curl -sf http://127.0.0.1:18031/readyz > /dev/null 2>&1 || {
    echo "ERROR: Path 1 consumer not ready"; tail -30 /tmp/path1-consumer.log; exit 1
}

KAFKA_BROKERS="$BROKER" \
KAFKA_TLS_ENABLED="false" \
KAFKA_CONSUMER_GROUP="ci-cm-adapter-${SUFFIX}" \
COST_MANAGEMENT_API_URL="http://127.0.0.1:18031" \
FLUSH_INTERVAL="1s" \
METRICS_ADDR="127.0.0.1:2112" \
"$ADAPTER_BIN" > /tmp/path1-adapter.log 2>&1 &
P1_ADAPTER_PID=$!
PIDS+=("$P1_ADAPTER_PID")

for i in $(seq 1 30); do
    curl -sf http://127.0.0.1:2112/readyz > /dev/null 2>&1 && break
    sleep 1
done
P1_READY=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:2112/readyz || echo "000")
check "Path 1 adapter /readyz passes against consumer" "200" "$P1_READY"

for i in $(seq 1 30); do
    P1_HB=$(db_query "$DB_ADAPTER" "SELECT count(*) FROM metering_entries WHERE resource_id = '${RES_ID}';")
    P1_INF=$(db_query "$DB_ADAPTER" "SELECT count(*) FROM metering_entries WHERE resource_id = '${INF_ID}';")
    if [ "$P1_HB" = "2" ] && [ "$P1_INF" = "1" ]; then
        break
    fi
    sleep 1
done

P1_ME_SUMMARY=$(db_query "$DB_ADAPTER" "SELECT count(*) || ':' || round(COALESCE(SUM(value), 0), 1) FROM metering_entries WHERE resource_id = '${RES_ID}' AND tenant_id = '${BILLING_ACCT}' AND meter_name = 'cpu_cores';")
check "Path 1 interval + negative correction net metering" "2:2999.5" "$P1_ME_SUMMARY"

P1_INF_VAL=$(db_query "$DB_ADAPTER" "SELECT round(COALESCE(SUM(value), 0), 0) FROM metering_entries WHERE resource_id = '${INF_ID}' AND tenant_id = '${BILLING_ACCT}' AND resource_type = 'maas_inference';")
check "Path 1 MaaS inference usage metering" "2500" "$P1_INF_VAL"

P1_SCHEMA_VER=$(db_query "$DB_ADAPTER" "SELECT data->'data'->>'schema_version' FROM raw_events WHERE event_id = '${POS_EVT}';")
check "Path 1 adapter emitted locked schema_version=v1" "v1" "$P1_SCHEMA_VER"

P1_NO_OFFSET=$(db_query "$DB_ADAPTER" "SELECT (data->'data' ? 'last_applied_offset')::text FROM raw_events WHERE event_id = '${POS_EVT}';")
check "Path 1 adapter stripped OSAC-internal last_applied_offset" "false" "$P1_NO_OFFSET"

P1_BUILTIN=$(db_query "$DB_ADAPTER" "SELECT count(*) FROM raw_events WHERE event_id IN ('${SYS_EVT}', '${SHD_EVT}');")
check "Path 1 adapter ignored system and shared topics" "0" "$P1_BUILTIN"

P1_DLQ_MSGS=$(kafka_consume osac.metering.dlq.tenant-e2e 5)
P1_DLQ_FOUND=$(echo "$P1_DLQ_MSGS" | grep -c "${V2_EVT}" || true)
check_ge "Path 1 adapter routed unsupported v2 event to Kafka DLQ" 1 "$P1_DLQ_FOUND"

kill "$P1_ADAPTER_PID" "$P1_CONSUMER_PID" 2>/dev/null || true
wait "$P1_ADAPTER_PID" "$P1_CONSUMER_PID" 2>/dev/null || true

# ──────────────────────────────────────────────────────────────────────
# Path 2: Direct Kafka reader (KAFKA_READER_MODE=osac-reader)
# ──────────────────────────────────────────────────────────────────────
echo ""
echo "--- Path 2: Direct Kafka reader (KAFKA_READER_MODE=osac-reader) ---"

KAFKA_BROKERS="$BROKER" \
KAFKA_READER_MODE="osac-reader" \
KAFKA_INCLUDE_INFERENCE="true" \
KAFKA_TLS_ENABLED="false" \
KAFKA_CONSUMER_GROUP="ci-osac-reader-${SUFFIX}" \
INVENTORY_DB_URL="postgres://user:pass@127.0.0.1:${DB_PORT}/${DB_DIRECT}" \
INGEST_LISTEN_ADDR="127.0.0.1:18032" \
METRICS_PORT="19032" \
DISABLE_COMPONENTS="watcher,reconciler,metering,rating" \
"$WATCHER_BIN" > /tmp/path2-reader.log 2>&1 &
P2_READER_PID=$!
PIDS+=("$P2_READER_PID")

for i in $(seq 1 30); do
    curl -sf http://127.0.0.1:18032/readyz > /dev/null 2>&1 && break
    sleep 1
done
curl -sf http://127.0.0.1:18032/readyz > /dev/null 2>&1 || {
    echo "ERROR: Path 2 reader not ready"; tail -30 /tmp/path2-reader.log; exit 1
}

for i in $(seq 1 30); do
    P2_HB=$(db_query "$DB_DIRECT" "SELECT count(*) FROM metering_entries WHERE resource_id = '${RES_ID}';")
    P2_INF=$(db_query "$DB_DIRECT" "SELECT count(*) FROM metering_entries WHERE resource_id = '${INF_ID}';")
    P2_DLQ=$(db_query "$DB_DIRECT" "SELECT count(*) FROM kafka_dead_letters WHERE topic = 'osac.metering.heartbeat.tenant-e2e';")
    if [ "$P2_HB" = "2" ] && [ "$P2_INF" = "1" ] && [ "$P2_DLQ" = "1" ]; then
        break
    fi
    sleep 1
done

P2_ME_SUMMARY=$(db_query "$DB_DIRECT" "SELECT count(*) || ':' || round(COALESCE(SUM(value), 0), 1) FROM metering_entries WHERE resource_id = '${RES_ID}' AND tenant_id = '${BILLING_ACCT}' AND meter_name = 'cpu_cores';")
check "Path 2 interval + negative correction net metering (matches Path 1)" "2:2999.5" "$P2_ME_SUMMARY"

P2_INF_VAL=$(db_query "$DB_DIRECT" "SELECT round(COALESCE(SUM(value), 0), 0) FROM metering_entries WHERE resource_id = '${INF_ID}' AND tenant_id = '${BILLING_ACCT}' AND resource_type = 'maas_inference';")
check "Path 2 MaaS inference usage metering (matches Path 1)" "2500" "$P2_INF_VAL"

P2_RAW_OFFSET=$(db_query "$DB_DIRECT" "SELECT data->'data'->>'last_applied_offset' FROM raw_events WHERE event_id = '${POS_EVT}';")
check "Path 2 raw_events preserved raw OSAC last_applied_offset verbatim" "11" "$P2_RAW_OFFSET"

P2_RAW_NO_SCHEMA=$(db_query "$DB_DIRECT" "SELECT (data->'data' ? 'schema_version')::text FROM raw_events WHERE event_id = '${POS_EVT}';")
check "Path 2 raw_events did not inject schema_version" "false" "$P2_RAW_NO_SCHEMA"

P2_BUILTIN=$(db_query "$DB_DIRECT" "SELECT count(*) FROM raw_events WHERE event_id IN ('${SYS_EVT}', '${SHD_EVT}');")
check "Path 2 direct reader ignored system and shared topics" "0" "$P2_BUILTIN"

P2_DLQ_COUNT=$(db_query "$DB_DIRECT" "SELECT count(*) FROM kafka_dead_letters WHERE topic = 'osac.metering.heartbeat.tenant-e2e' AND error_reason = 'unsupported_version';")
check "Path 2 persisted unsupported v2 event to kafka_dead_letters" "1" "$P2_DLQ_COUNT"

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
