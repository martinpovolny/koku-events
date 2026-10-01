# OSAC Catalog Synchronization Bridge (Interim)

## Status

**Interim / Temporary Bridge**  
**Tracking Jira:** [OSAC-3876: OSAC Catalog Synchronization](https://redhat.atlassian.net/browse/OSAC-3876)  
**Related PRs / Specs:** [OSAC Batch Ingestion Contract](osac-batch-ingest-contract.md), [Ingestion Modes](../ingestion-modes.md)

---

## 1. Problem Statement & Motivation

The primary ingestion path for Cost Management is the **Cost Adapter**:
```text
OSAC Services → Kafka → OSAC Cost Adapter → POST /api/v1/events/batch → Ingestion Receipts / Raw Events / Inventory / Metering
```

This batch pipeline is optimized for runtime metering (resource lifecycle events, VM/cluster heartbeats, and token/inference usage metrics). However:
- The OSAC Cost Adapter is an event meter; it **does not discover or emit catalog items or instance type definitions**.
- The primary ingestion API (`POST /api/v1/events/batch`) receives only workload events and does not accept catalog definitions.

Without catalog entities, the cost engine cannot:
1. **Assign rates / price lists to catalog items**: Operators cannot define per-SKU pricing for cluster offerings, VM catalog items, or bare metal instances.
2. **Resolve resource specifications**: Metering cannot look up cores and memory when runtime events omit resource sizing (e.g. resolving `standard-4-16` to 4 vCPUs and 16 GiB RAM).
3. **Attribute costs to named entities**: Tenant and project UUIDs cannot be resolved to human-readable names and labels in reports, dashboards, and wallet ledgers.

The permanent, native catalog synchronization mechanism between OSAC and Cost Management is currently being designed and tracked under **[OSAC-3876](https://redhat.atlassian.net/browse/OSAC-3876)**.

Until OSAC-3876 is implemented and delivered, this document defines an **interim bridge** using the retained reconciler to selectively synchronize catalog and metadata entities from OSAC REST endpoints.

---

## 2. End-to-End Pipeline & Interim Topology

```text
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │                                OSAC Control Plane                           │
 │                                                                             │
 │   Fulfillment REST API                      Kafka Broker                    │
 │   (/api/fulfillment/v1/...)                 (runtime event topics)          │
 └─────────────┬──────────────────────────────────────┬────────────────────────┘
               │                                      │
               │ REST Polling (1h)                    │ Consumer
               │ (Filtered Catalog Entities)          │
               ▼                                      ▼
 ┌───────────────────────────┐         ┌───────────────────────────┐
 │     Legacy Reconciler     │         │     OSAC Cost Adapter     │
 │  (Temporary Interim Sync) │         │    (Runtime Event Emitter)│
 └─────────────┬─────────────┘         └──────────────┬────────────┘
               │                                      │
               │ Upsert reference data                │ POST /api/v1/events/batch
               │                                      │ (Canonical CloudEvents)
               ▼                                      ▼
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │                         Cost Management Storage                             │
 │                                                                             │
 │  Reference / Catalog Tables:              Runtime Event Tables:             │
 │  - inventory_catalog_item                 - ingestion_receipts (dedup)      │
 │  - inventory_instance_type                - raw_events (audit)              │
 │  - inventory_tenant                       - inventory_compute_instance      │
 │  - inventory_project                      - metering_entries                │
 └─────────────────────────────┬──────────────────────┬────────────────────────┘
                               │                      │
                               └──────────┬───────────┘
                                          ▼
                         ┌─────────────────────────────────┐
                         │          Rating Sweep           │
                         │                                 │
                         │  Matches unrated metering rows  │
                         │  with Rates (keyed on SKU /     │
                         │  instance_type / resource_type) │
                         └────────────────┬────────────────┘
                                          ▼
                         ┌─────────────────────────────────┐
                         │          cost_entries           │
                         │   (Rated monetary cost rows)    │
                         └─────────────────────────────────┘
```

### Flow Verification
1. **Raw Ingestion**: OSAC events arrive via Kafka → Cost Adapter → `POST /api/v1/events/batch`, creating raw event audit records and `metering_entries` rows.
2. **Catalog Availability**: The interim reconciler periodically queries OSAC REST endpoints to populate `inventory_catalog_item` and `inventory_instance_type`.
3. **Rate Configuration**: Price administrators assign rates in the `rates` table corresponding to catalog items and instance types.
4. **Cost Creation**: The periodic rating worker sweeps `metering_entries`, looks up the corresponding rates (using instance type and catalog metadata), and creates billable `cost_entries`.

---

## 3. Selective Entity Filtering (`RECONCILE_ENTITIES`)

### Why Selective Filtering is Required

Previously, running the reconciler would poll **all** OSAC entities, including active runtime workloads:
- `compute_instances`
- `clusters`
- `bare_metal_instances`

Because the Cost Adapter actively meters those exact workloads via batch CloudEvents, running the reconciler against workload instances would create race conditions, conflicting state overwrites, or spurious deletions.

To avoid this conflict while still syncing catalog data, the reconciler supports selective entity filtering via `RECONCILE_ENTITIES`.

### Supported Entity Tokens

| Token | Target Table | Description | Default Status in Interim Bridge |
|---|---|---|---|
| `catalog_items` | `inventory_catalog_item` | Cluster, compute, and bare metal catalog templates / SKUs | **Enabled** |
| `instance_types` | `inventory_instance_type` | Machine sizing specifications (cores, memory GiB) | **Enabled** |
| `tenants` | `inventory_tenant` | Tenant IDs, names, and labels for wallet attribution | **Enabled** |
| `projects` | `inventory_project` | Project IDs, names, and tenant relationships | **Enabled** |
| `compute_instances` | `inventory_compute_instance` | Live VM instances | **Disabled** (owned by Cost Adapter) |
| `clusters` | `inventory_cluster` | Live Kubernetes clusters | **Disabled** (owned by Cost Adapter) |
| `bare_metal_instances`| `inventory_bare_metal_instance` | Live bare metal instances | **Disabled** (owned by Cost Adapter) |

---

## 4. Configuration Reference

### Environment Variables

| Variable | Type | Default | Description |
|---|---|---|---|
| `RECONCILE_ENTITIES` | string | `all` | Comma-separated list of entities to reconcile. Can be `all`, `*`, or a specific subset (e.g. `catalog_items,instance_types,tenants,projects`). |
| `DISABLE_COMPONENTS` | string | *(empty)* | Comma-separated list of application components to disable (e.g. `watcher`). |
| `RECONCILE_INTERVAL` | duration | `1h` | Polling frequency for reconciler sweeps. |
| `OSAC_BASE_URL` | string | `http://localhost:8011` | OSAC REST API base URL. |
| `OSAC_TOKEN` | string | *(empty)* | Authentication token for OSAC REST API. |

### Recommended Deployment Configuration (CRC & Production)

In deployments using the Cost Adapter and batch API:

```yaml
env:
  # The OSAC Cost Adapter delivers runtime CloudEvents via POST /api/v1/events/batch.
  # The gRPC watcher is disabled:
  - name: DISABLE_COMPONENTS
    value: "watcher"

  # The reconciler is active as an interim bridge for catalog and tenancy metadata only.
  # Runtime workload instances (compute_instances, clusters, bare_metal_instances) are excluded:
  - name: RECONCILE_ENTITIES
    value: "catalog_items,instance_types,tenants,projects"

  - name: RECONCILE_INTERVAL
    value: "1h"
```

---

## 5. Decommissioning & Sunset Plan

This implementation is strictly a **temporary bridge**:

1. **Jira Tracking**: [OSAC-3876](https://redhat.atlassian.net/browse/OSAC-3876) defines the requirements for a first-class catalog synchronization protocol (e.g. catalog event streaming, unified metadata push, or an OSAC-managed catalog exporter).
2. **Sunset Criteria**: Upon completion and verification of OSAC-3876:
   - The interim reconciler catalog REST polling will be deprecated.
   - The remaining legacy reconciler and watcher code will be retired.
   - Catalog ingestion will migrate to the standardized OSAC-3876 contract.
