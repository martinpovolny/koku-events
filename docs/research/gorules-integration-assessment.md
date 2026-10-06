# GoRules/Zen Integration Assessment

> Practical assessment of integrating GoRules/Zen engine into the
> cost-event-consumer for REQ-13 (custom rate dimensions).
> Date: 2026-06-29

> **Current status (2026-10-06):** GoRules is integrated into the rating path. The runtime engine is database-backed; the JSON files under `inventory-watcher/rules/` are embedded only to seed missing database rows on first initialization.

## Go SDK

Package: `github.com/gorules/zen-go`

The Zen engine core is Rust. The Go SDK ships precompiled static libraries
for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 — linked via CGO.
No Rust toolchain required.

```bash
go get github.com/gorules/zen-go
```

## How It Works

The service creates a Zen engine with a database-backed loader. The loader resolves rule names from an in-memory cache populated from the `pricing_rules` table:

```go
engine := ruleengine.NewFromStore(store)
defer engine.Close()

if _, err := engine.ReloadIfChanged(ctx); err != nil {
	return err
}

result, err := engine.EvaluateRate("compute-pricing.json", ruleengine.PricingInput{
	CatalogItem:  "catalog-live-vm-standard",
	InstanceType: "standard-4-16",
	TenantTier:   "gold",
	Value:        1500.0,
})
// result contains cost_amount, effective_rate, currency, and description.
```

On startup, the consumer inserts the bundled JSON Decision Models only when their names are absent from `pricing_rules`. Existing database rows are never overwritten. On each rating sweep, a version check reloads the in-memory cache when a rule changes, without restarting the process.

## JDM Decision Table Example

A decision table for tiered pricing looks like:

```json
{
  "nodes": [
    {"id": "input", "type": "inputNode", "name": "MeteringInput"},
    {
      "id": "rates",
      "type": "decisionTableNode",
      "name": "RateLookup",
      "content": {
        "hitPolicy": "first",
        "inputs": [
          {"field": "resource_type"},
          {"field": "meter_name"},
          {"field": "value"}
        ],
        "outputs": [
          {"field": "cost"},
          {"field": "currency"}
        ],
        "rules": [
          {
            "resource_type": "== 'compute_instance'",
            "meter_name": "== 'vm_uptime_seconds'",
            "value": "<= 3600",
            "cost": "value * 0.01 / 3600",
            "currency": "'USD'"
          },
          {
            "resource_type": "== 'compute_instance'",
            "meter_name": "== 'vm_uptime_seconds'",
            "value": "> 3600",
            "cost": "3600 * 0.01/3600 + (value - 3600) * 0.008/3600",
            "currency": "'USD'"
          }
        ]
      }
    },
    {"id": "output", "type": "outputNode", "name": "CostResult"}
  ],
  "edges": [
    {"sourceId": "input", "targetId": "rates"},
    {"sourceId": "rates", "targetId": "output"}
  ]
}
```

## JDM Editor

Available as:
- **Standalone Docker:** `docker run -p 8080:8080 gorules/editor`
- **React component:** `npm i @gorules/jdm-editor` — embeddable `<DecisionGraph />`
- **Live demo:** https://gorules.github.io/jdm-editor/

## Performance

| Mode | Latency | Throughput |
|------|---------|------------|
| Embedded SDK (our path) | <1ms | 10-100K evals/sec |
| HTTP Agent | 10-20ms | 1-10K req/s |

Our 500-entry rating batch would process in <50ms. No concern.

## Constraints

- **CGO_ENABLED=1** required — no pure-Go option
- **No Alpine/musl** — needs glibc (use `golang:1.22-bookworm`)
- **`Dispose()` calls** — Rust memory is outside Go's GC
- Expression language compiles to bytecode (stack VM), not AST traversal

## Rule Storage Options

| Storage | Mechanism | Hot reload |
|---------|-----------|------------|
| Database | `pricing_rules` JSONB table, cached by the service | On the next rating sweep after a version change |
| Embedded defaults | Bundled `rules/*.json`, inserted only for missing rule names | Startup seed only |

## Recommendation

**Current implementation:** GoRules handles programmable dimensions such as catalog item, instance type, and tenant tier. Static SQL-backed rates remain the fallback when no matching rule exists or rule evaluation fails, preserving the existing rating path.

The CGO dependency and JDM learning curve remain deployment considerations. The database-backed loader avoids a runtime filesystem dependency and supports rule changes without rebuilding or restarting the consumer.
