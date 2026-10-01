# OSAC batch ingestion contract

## Status

Approved implementation contract for the primary Cost Management adapter proof
of concept. The adapter delivers canonical OSAC CloudEvents to this HTTP
receiver. The separate direct Kafka consumer is a temporary experiment and is
not part of this batch ownership path; see [Ingestion modes](../ingestion-modes.md).

## Endpoint

`POST /api/v1/events/batch` accepts a JSON object with an `events` array of
canonical CloudEvents 1.0 structured-mode envelopes.

- A request contains from one to 100 events and is at most 1 MiB.
- Every event must have `specversion: "1.0"`, non-empty `id`, `type`,
  `source`, and `time`, and JSON `data`.
- Existing timestamp, resource identity, tenant identity, and OSAC v1
  extension validation applies to every member before any database write.
- The response is `204 No Content` only when all previously unseen events
  have been durably stored and processed. Invalid members return `400`; an
  identity collision returns `409`; neither case writes any member.

## Receipt and replay semantics

`raw_events` remains an append-only, non-unique audit log. It is not the
idempotency mechanism.

The receiver stores an `ingestion_receipts` row, unique on
`(event_source, event_id)`, containing a SHA-256 digest of the canonical
structured CloudEvent.

- A new receipt is claimed and the event is processed in the request
  transaction.
- A replay with the same source/id and digest is a successful no-op.
- The same source/id with a different digest is a `409 Conflict` and rolls
  back the whole request.
- Concurrent deliveries serialize on the receipt primary key; only one can
  produce inventory or billing effects.

## Transaction and processing model

All receipt claims, raw-event inserts, inventory changes, and event-driven
metering entries for a batch share one PostgreSQL transaction. The HTTP and
Kafka entry points use the same validation and event-processing function so
that supported OSAC v1 CloudEvents cannot be silently accepted by one path
and skipped by the other.

The legacy `POST /api/v1/events` endpoint remains for existing local callers;
it is not the adapter delivery protocol.

## Failure behavior

Database or processing failures return `500` and roll back the batch. The
adapter must treat timeouts, `429`, and `5xx` as unacknowledged delivery and
retry the unchanged batch; its Kafka runner must not commit offsets until a
`204` response.

## Gap: a permanent batch rejection can block the Kafka consumer

**Observed on local CRC, 2026-10-01.** The Cost Management adapter was ready
(`1/1`) but repeatedly received HTTP `400` from the batch endpoint:
`event time is too old (about 6,180 minutes ago; max 2h0m0s)`. No new runtime
events were reaching Cost through that adapter. This followed a period in which
the Cost consumer was unavailable and events aged past its timestamp limit.

The shared adapter Runner has a DLQ for an individual Kafka record that
`Submit` rejects as non-retryable. That protection does not cover this case:

1. `Submit` accepts each structurally valid event and buffers it. It does not
   check the Cost receiver's event-age window.
2. `Flush` sends an atomic batch. Cost validates every member before writing
   any member; one stale event rejects the entire batch with `400`.
3. The adapter cannot identify the offending Kafka record from the current
   batch response. It retains the whole batch and retries it every flush cycle.
   The Runner does not advance its committed Kafka offsets.
4. A later valid event behind that batch cannot progress. Adapter readiness
   still reports healthy because it only checks the receiver's `/readyz`.

This behavior is intentional for an *unattributed* `400`: sending the whole
batch to the DLQ or acknowledging it would silently discard valid records.
But indefinite retry of a *permanent* validation failure is a liveness gap.
It also means an event accepted into the buffer while fresh can become stale
during a prolonged flush failure. Separately, Cost validates age before
checking its receipt ledger, so an already-ingested event replayed after the
age window can also fail instead of becoming an idempotent no-op.

### Required follow-up

Design a way to isolate a permanently invalid member while preserving the
batch's atomic success contract and Kafka at-least-once delivery. Possibilities
include a stable machine-readable validation response with the offending
event's index/identity, or adapter-side batch isolation with original Kafka
topic/partition/offset retained for each buffered member. Timestamp checking
only at `Submit` is insufficient because buffered events can age before
`Flush`. Do not treat all `4xx` responses as DLQ-worthy: authentication,
authorization, contract drift, and transient receiver failures need separate
handling.

The fix is complete only when tests show that:

- A stale event is routed to the configured DLQ **with its original record and
  failure reason**, and its source offset advances only after the DLQ write is
  confirmed. If the DLQ write fails, the event remains replayable.
- Fresh events behind it reach Cost and get `204`, receipts, raw-event rows,
  and expected metering rows without restarting or changing consumer groups.
- Mixed valid/invalid batches do not lose valid members or produce duplicate
  billing writes across retries, process restarts, or Kafka rebalances.
- Timeouts, `429`, and `5xx` continue to retry without DLQ or offset advance.
  A genuine incompatible `400` remains visible to operators rather than being
  silently discarded.
- Delivery health reflects a persistently failing flush, not just `/readyz`.

### Local CRC recovery (not the design fix)

For disposable test traffic, an operator may stop the adapter, record the
current group offsets and topic end offsets, then reset **only that consumer
group** to the end and restart it before generating fresh events. This skips
old records for that group; it does not delete Kafka records or put them in the
DLQ. Record the skipped offset range and do not use this procedure for billable
or otherwise non-disposable events. Setting `KAFKA_START_OFFSET=newest` alone
does **not** reset a group that already has committed offsets. A new group ID
also requires a matching Kafka group ACL and changes the delivery checkpoint.
The reset requires an authenticated Kafka administrator; an unauthenticated
broker-local `kafka-consumer-groups.sh --describe` attempt on this CRC cluster
returned `GroupAuthorizationException`, so no offset reset was performed as
part of this gap investigation.
