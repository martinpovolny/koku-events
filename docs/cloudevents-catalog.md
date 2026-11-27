# CloudEvents Catalog

> All CloudEvents formats consumed or produced by the cost-event-consumer.
> Sources linked for each schema.

The current OSAC-to-Cost path is the canonical OSAC metering-service v1 contract described near the end of this document. The earlier collector and simulator formats remain supported for compatibility, but they are historical and are not emitted by the OSAC Cost Management adapter.

## VMaaS — Compute Instance Lifecycle

> Historical compatibility format. The current OSAC adapter sends canonical `osac.resource.*.v1` events instead; see [OSAC Metering-Service v1](#osac-metering-service-v1--canonical-cost-management-adapter-contract).

**Type:** `osac.compute_instance.lifecycle`
**Source:** OSAC metering collector ([schema](https://github.com/masayag/osac-metering-discover-poc/blob/main/collector/README.md#cloudevents-schema))
**Our handler:** `internal/api/handler.go` → `handleComputeInstanceEvent`

```json
{
  "specversion": "1.0",
  "type": "osac.compute_instance.lifecycle",
  "source": "osac.metering.collector",
  "id": "<uuid>",
  "time": "<ISO8601>",
  "subject": "<tenant_id>",
  "data": {
    "duration_seconds": 60,
    "cpu_core_seconds": 120,
    "memory_gib_seconds": 240,
    "tenant_id": "osac-e2e-ci",
    "instance_id": "019eb257-8108-773f-99c4-5d7642e9e7d8",
    "template": "osac.templates.ocp_virt_vm",
    "catalog_item": "",
    "state": "RUNNING",
    "cores": 2,
    "memory_gib": 4
  }
}
```

**Fields we meter:** `duration_seconds` → `vm_uptime_seconds`,
`cpu_core_seconds` → `vm_cpu_core_seconds`,
`memory_gib_seconds` → `vm_memory_gib_seconds`

**Note:** `catalog_item` field present but empty in current collector.
`project_id` is missing — Moti noted it should be added alongside `tenant_id`.

---

## CaaS — Cluster Lifecycle

> Historical compatibility format. The current OSAC adapter sends canonical `osac.resource.*.v1` events instead; see [OSAC Metering-Service v1](#osac-metering-service-v1--canonical-cost-management-adapter-contract).

**Type:** `osac.cluster.lifecycle`
**Source:** OSAC metering collector ([schema](https://github.com/masayag/osac-metering-discover-poc/blob/main/collector/README-caas.md#cloudevents-schema))
**Our handler:** `internal/api/handler.go` → `handleClusterEvent`

### Control Plane Event

```json
{
  "specversion": "1.0",
  "type": "osac.cluster.lifecycle",
  "source": "osac.metering.collector",
  "id": "<uuid>",
  "time": "<ISO8601>",
  "subject": "<tenant_id>",
  "data": {
    "duration_seconds": 60,
    "worker_node_seconds": 0,
    "node_count": 0,
    "tenant_id": "shared",
    "cluster_id": "<uuid>",
    "template": "osac.templates.ocp_ci_small",
    "state": "READY",
    "host_type": "_control_plane"
  }
}
```

### Worker Node Set Event

```json
{
  "specversion": "1.0",
  "type": "osac.cluster.lifecycle",
  "source": "osac.metering.collector",
  "id": "<uuid>",
  "time": "<ISO8601>",
  "subject": "<tenant_id>",
  "data": {
    "duration_seconds": 60,
    "worker_node_seconds": 60,
    "node_count": 1,
    "tenant_id": "shared",
    "cluster_id": "<uuid>",
    "template": "osac.templates.ocp_ci_small",
    "state": "READY",
    "host_type": "ci-worker"
  }
}
```

**Fields we meter:** Control plane → `cluster_uptime_seconds`.
Worker → `cluster_worker_node_seconds` (node_count × duration).
`host_type == "_control_plane"` determines which meters fire.

---

## MaaS — Inference Token Usage (IPP Plugin)

> External IPP compatibility format. It is not the event shape emitted by the OSAC Cost Management adapter; the current canonical OSAC inference event is documented below.

**Type:** `inference.tokens.used`
**Source:** IPP external-metering plugin
([plugin.go](https://github.com/opendatahub-io/ai-gateway-payload-processing/blob/main/pkg/plugins/external-metering/plugin.go),
[client.go](https://github.com/opendatahub-io/ai-gateway-payload-processing/blob/main/pkg/plugins/external-metering/client.go))
**Our handler:** `internal/api/handler.go` → `handleModelEvent`

```json
{
  "specversion": "1.0",
  "type": "inference.tokens.used",
  "source": "maas-gateway",
  "id": "<uuid>",
  "time": "<ISO8601>",
  "subject": "<username>",
  "data": {
    "user": "<username>",
    "group": "<k8s-group>",
    "subscription": "<maas-subscription-name>",
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "prompt_tokens": 1500,
    "completion_tokens": 800,
    "total_tokens": 2300,
    "cached_input_tokens": 200,
    "cache_creation_tokens": 0,
    "reasoning_tokens": 150,
    "duration_ms": 3200
  }
}
```

### Token field semantics

The token fields follow the
[OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat/object)
`usage` object structure. The IPP plugin extracts them from the
vLLM/OpenAI-compatible response and forwards them verbatim in the
CloudEvent `data` payload
([source](https://github.com/opendatahub-io/ai-gateway-payload-processing/blob/main/pkg/plugins/external-metering/plugin.go)).

| Field | OpenAI origin | Relationship | Metered? |
|-------|--------------|--------------|----------|
| `prompt_tokens` | `usage.prompt_tokens` | Total input tokens | Yes → `maas_tokens_in` |
| `completion_tokens` | `usage.completion_tokens` | Total output tokens (includes reasoning) | Yes → `maas_tokens_out` |
| `total_tokens` | `usage.total_tokens` | `prompt_tokens + completion_tokens` | No (redundant sum) |
| `cached_input_tokens` | `usage.prompt_tokens_details.cached_tokens` | **Subset** of `prompt_tokens` served from KV cache | No (subset, would double-count) |
| `cache_creation_tokens` | `usage.prompt_tokens_details.cache_creation_input_tokens` | Tokens used to populate the cache this request | No (subset of prompt_tokens) |
| `reasoning_tokens` | `usage.completion_tokens_details.reasoning_tokens` | **Subset** of `completion_tokens` used for chain-of-thought (o1/o3/DeepSeek R1) | No (subset, would double-count) |

**Why only 3 billing meters:** `cached_input_tokens` and
`reasoning_tokens` are subsets of `prompt_tokens` and
`completion_tokens` respectively — not additive quantities. Metering
them as separate cost entries would double-bill. All 6 fields are
parsed and stored in `raw_events` for observability and audit; only
`tokens_in`, `tokens_out`, and `requests` produce `metering_entries`.

See also:
- [OpenAI usage object](https://platform.openai.com/docs/api-reference/chat/object) — canonical field definitions
- [vLLM reasoning outputs](https://docs.vllm.ai/en/latest/features/reasoning_outputs/) — vLLM's reasoning token support
- [Rate configuration guide](rate-configuration-guide.md) — MaaS rate setup

### Identity fields

| Field | Source | Used for |
|-------|--------|----------|
| `user` | Authorino/maas-api identity header | Per-user cost attribution (`user_id` on metering/cost entries) |
| `group` | K8s group from auth | Parsed, not currently used for billing |
| `subscription` | MaaSSubscription CR name | Tenant attribution fallback (namespace parsing) |
| `organization_id` | Proposed addition ([PR #386](https://github.com/opendatahub-io/ai-gateway-payload-processing/pull/386)) | Tenant attribution (`tenant_id`) |
| `cost_center` | Proposed addition ([PR #386](https://github.com/opendatahub-io/ai-gateway-payload-processing/pull/386)) | Project attribution (`project_id`) |

**Note:** `subject` is the username (not `tenant_id` like VMaaS/CaaS
events). Tenant is resolved from `organization_id` (preferred) or
`subscription` namespace (fallback).

---

## Balance Check API (IPP Plugin → Cost Consumer)

**Not a CloudEvent** — this is a synchronous REST call.

**Sources:**
- IPP client: [client.go](https://github.com/opendatahub-io/ai-gateway-payload-processing/blob/61b6160/pkg/plugins/external-metering/client.go)
- Metering simulator OpenAPI spec: [openapi.yaml](https://github.com/noyitz/metering-simulator/blob/main/openapi.yaml) (saved locally at `docs/specs/maas-metering-openapi.yaml`)

### Request

```
GET /api/v1/customers/{customerID}/entitlements/{featureKey}/value?model={model}
```

- `customerID` — resolved from `x-maas-username` or `x-maas-subscription` headers
- `featureKey` — configured, default `"inference-tokens"`
- `model` — the model being requested

### Response

```json
{
  "has_access": true,
  "balance": 45000.0,
  "usage": 5000.0,
  "overage": 0.0
}
```

- `has_access: false` → IPP returns `ResourceExhausted` (blocks inference)
- `has_access: true` → request proceeds
- On error + `failOpen=true` → request proceeds anyway

### What We Need to Implement

`GET /api/v1/customers/{id}/entitlements/{key}/value` that:
1. Maps `customerID` to a tenant (via Keycloak username → tenant lookup,
   or direct if customerID = tenant_id)
2. Queries current consumption: `MeteringSum(tenant, meter, periodStart, periodEnd)`
3. Queries quota limit: `QuotasForTenant(tenant)`
4. Returns `has_access` = (consumed < limit), `balance` = (limit - consumed),
   `usage` = consumed, `overage` = max(0, consumed - limit)

**Status:** Implemented. Contract-verified against the metering-simulator
OpenAPI spec and the IPP client struct tags.

**Note:** Response uses `hasAccess` (camelCase) matching the IPP client.go
struct tag, not `has_access` (snake_case).

---

## MaaS — Our Current Mock Format

> Historical simulator format. Retained as a compatibility reference for older demo data.

**Type:** `osac.model.lifecycle`
**Source:** Our `maas-simulator` (`cmd/maas-simulator/main.go`)
**Our handler:** `internal/api/handler.go` → `handleModelEvent`

```json
{
  "specversion": "1.0",
  "type": "osac.model.lifecycle",
  "source": "osac.maas.simulator",
  "id": "<uuid>",
  "time": "<ISO8601>",
  "subject": "<tenant_id>",
  "data": {
    "model_name": "llama-3-70b",
    "model_id": "<uuid>",
    "tenant_id": "tenant-acme",
    "tokens_in": 1500,
    "tokens_out": 800,
    "request_count": 1,
    "duration_seconds": 3,
    "state": "MODEL_STATE_RUNNING"
  }
}
```

**Differences from real IPP format:**
| Field | Our mock | Real IPP |
|-------|----------|----------|
| Event type | `osac.model.lifecycle` | `inference.tokens.used` |
| Subject | `tenant_id` | `username` |
| Token fields | `tokens_in`, `tokens_out` | `prompt_tokens`, `completion_tokens` + detail fields |
| Identity | `tenant_id` only | `user`, `group`, `subscription`, `organization_id` |
| Duration | `duration_seconds` | `duration_ms` |
| Model | `model_name` | `model` |

**Status:** Handler accepts both formats — IPP fields take precedence
when present, mock fields used as fallback. Both produce the same 3
meters (`maas_tokens_in`, `maas_tokens_out`, `maas_requests`).

---

## OSAC Metering-Service v1 — Canonical Cost Management Adapter Contract

`koku-events` owns the locked Cost Management `v1` usage contract and explicit OSAC-to-Cost mapping in [`internal/osacmetering/`](../inventory-watcher/internal/osacmetering/contract.go). Two delivery paths are supported in both **single-cluster** and **two-cluster** deployments:

1. **In-repo OSAC Cost Management adapter (HTTP):** Reads per-tenant `osac.metering.heartbeat.<tenant>` (and optionally `osac.metering.inference`) inside the OSAC cluster, translates raw OSAC CloudEvents into the locked `CloudEventEnvelope` (stripping OSAC-internal fields like `last_applied_offset` and `rules_revision`), and POSTs each event synchronously to `POST /api/v1/events` (or `/v1/events` behind an external Envoy gateway in 2-cluster deployments).
2. **Direct external Kafka reader (`KAFKA_READER_MODE=osac-reader`):** `cost-event-consumer` connects directly to OSAC's Strimzi Kafka cluster as an external `metering.readers` principal over TLS + SCRAM-SHA-512 (in-cluster Strimzi bootstrap for 1-cluster, or Strimzi external listener + custom CA for 2-cluster), dynamically subscribes via regex to `^osac\.metering\.heartbeat\..+` while excluding `system` and `shared`, translates events through `internal/osacmetering`, and persists permanently invalid records to `kafka_dead_letters`.

**Contract & mapper:** [`internal/osacmetering/contract.go`](../inventory-watcher/internal/osacmetering/contract.go), [`internal/osacmetering/translate.go`](../inventory-watcher/internal/osacmetering/translate.go)
**Receiver:** [`internal/api/handler.go`](../inventory-watcher/internal/api/handler.go) → `IngestEvent` / `IngestEventBatch` / `ProcessKafkaEvent` → `processOSACResourceEvent`
**Kafka reader:** [`internal/kafka/consumer.go`](../inventory-watcher/internal/kafka/consumer.go)

### Event types and Kafka topics

| Event type | Kafka topic | Meaning |
|------------|-------------|---------|
| `osac.resource.heartbeat.v1` | `osac.metering.heartbeat.<tenant>` | Authoritative additive interval usage (and backdated negative/positive corrections) |
| `osac.inference.usage.v1` | `osac.metering.inference` | MaaS inference usage (interval/delta `usage` object or legacy token dimensions) |
| `osac.resource.{created,started,resumed,updated,suspended,deleted}.v1` | `osac.metering.lifecycle` *(legacy PoC mode)* | Legacy resource lifecycle transitions retained for backwards compatibility |

In `KAFKA_READER_MODE=osac-reader`, internal OSAC topics `osac.metering.heartbeat.system` and `osac.metering.heartbeat.shared` are explicitly excluded to match Strimzi `metering.readers` ACLs.

### Supported resource types

All six OSAC metering `v1` resource types plus MaaS inference are normalized into `metering_entries`:

| Resource type | Meters | koku-events handling |
|---------------|--------|----------------------|
| `compute_instance` | `compute` | Interval usage persisted directly into `metering_entries` (legacy lifecycle events still upsert `inventory_compute_instance`) |
| `cluster_order` | `control_plane`, `compute` | Interval usage persisted directly into `metering_entries` (legacy lifecycle events still upsert `inventory_cluster`) |
| `bare_metal_instance` | `compute` | Interval usage persisted directly into `metering_entries` |
| `external_ip` | `allocation` | Interval usage persisted directly into `metering_entries` |
| `nat_gateway` | `allocation`, `consumption` | Interval usage persisted directly into `metering_entries` |
| `volume` | `capacity` | Interval usage persisted directly into `metering_entries` |
| `maas_inference` | `inference` (or token meters) | Normalized `usage` object persisted to `metering_entries`; legacy `prompt_tokens`/`completion_tokens` dimensions still route through `MeterMaaSEvent` |

### Canonical locked CloudEvent envelope (`osac.resource.heartbeat.v1`)

```json
{
  "specversion": "1.0",
  "id": "<deterministic-sha256>",
  "source": "osac-metering",
  "type": "osac.resource.heartbeat.v1",
  "datacontenttype": "application/json",
  "time": "2026-10-01T12:00:00Z",
  "osacresourceid": "<resource-uuid>",
  "osacresourcetype": "compute_instance",
  "osactenant": "<tenant-id>",
  "osacproject": "<project-id>",
  "data": {
    "schema_version": "v1",
    "tenant_id": "<tenant-id>",
    "external_billing_account_id": "<optional-billing-account-id>",
    "project_id": "<project-id>",
    "resource_type": "compute_instance",
    "resource_id": "<resource-uuid>",
    "meter": "compute",
    "component": "compute",
    "heartbeat_time": "2026-10-01T12:05:00Z",
    "usage": {
      "semantics": "interval",
      "from": "2026-10-01T12:00:00Z",
      "to": "2026-10-01T12:05:00Z",
      "quantity": "300.000000",
      "unit": "seconds",
      "precision": "second"
    },
    "billing_dimensions": {
      "deployment": "cluster-a",
      "tenant": "<tenant-id>",
      "project": "<project-id>",
      "catalog": "cat-rhel-9",
      "instance_type": "standard-4-8"
    }
  }
}
```

### CloudEvent extensions

| Extension | Description |
|-----------|-------------|
| `osacresourceid` | Resource UUID (partition key for per-resource ordering) |
| `osacresourcetype` | Resource type: `compute_instance`, `cluster_order`, `bare_metal_instance`, `external_ip`, `nat_gateway`, `volume`, or `maas_inference` |
| `osactenant` | Internal OSAC tenant identifier |
| `osacproject` | Project identifier (optional) |

### Semantics and validation rules

- **Locked v1 mapping:** `internal/osacmetering` accepts both raw OSAC `osac.resource.heartbeat.v1` events (where `data.schema_version` is absent and `last_applied_offset` / `rules_revision` may appear) and adapter-mapped locked `CloudEventEnvelope` payloads (`data.schema_version: "v1"`). Unsupported major versions (e.g. `osac.resource.heartbeat.v2` or unknown `data.schema_version`) are rejected permanently.
- **Billing tenant attribution:** When `external_billing_account_id` is populated, it becomes the effective `tenant_id` on `raw_events` and `metering_entries`; otherwise `tenant_id` is used.
- **Additive corrections & backdated timestamps:** OSAC sets CloudEvent `time` equal to `usage.from` and emits signed `usage.quantity` (negative values represent billing corrections for late-arriving UTC-day or invoiced-period adjustments). `koku-events` allows historical `time` / `usage.from` on `osac.resource.heartbeat.v1` interval events while still rejecting future `time` or `heartbeat_time` (>5 minutes ahead) and applying signed values in rating (`ApplyRate` and `ApplyRateCumulative`).
- **Idempotency & dead-lettering:** Every event is protected by `ingestion_receipts` (`(event_source, event_id)` + payload SHA-256) inside the same PostgreSQL transaction as `raw_events` and `metering_entries`. In Kafka consumer mode, permanent failures are persisted to `kafka_dead_letters` before offset commit, while transient database errors retry without advancing offsets.
- **Identity & size rules:** `osacresourceid`, `osacresourcetype`, and `osactenant` are required extensions and must match `data.resource_id`, `data.resource_type`, and `data.tenant_id`. HTTP request bodies are limited to 1 MiB (1–100 events for `/api/v1/events/batch`).
