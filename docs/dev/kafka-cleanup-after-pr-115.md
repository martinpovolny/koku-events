# Kafka experiment cleanup after PR #115

## Context

PR #115 introduced an opt-in Kafka producer/consumer experiment as a temporary
replacement for the OSAC listener path. It currently publishes HTTP
`/api/v1/events` events to Kafka, while the current ingestion architecture uses
OSAC batch ingestion directly.

PR #123 adds atomic OSAC batch ingestion and its batch path must remain
Kafka-free. The existing OSAC watcher and reconciler are useful to retain as a
legacy or operational fallback, but they are not part of the primary batch
ingestion flow.

## Cleanup tasks

- Remove the temporary Kafka producer/consumer experiment from the application
  ingestion path, including API-to-Kafka publishing from `/api/v1/events`.
- Keep `/api/v1/events/batch` direct: validate, persist, and process without Kafka.
- Retain the OSAC watcher and reconciler, including their existing opt-out
  controls, without making them dependencies of batch ingestion.
- Remove or archive `integration-test/test-kafka.sh` and
  `integration-test/test-osac-kafka.sh` once the temporary Kafka path is gone.
- Remove Kafka workflows, configuration, and dependencies that only support the
  experiment.
- Audit `KAFKA_MODE`, configuration, README, architecture docs, and ADRs so they
  describe batch ingestion as primary and watcher/reconciler as retained
  legacy/fallback components.
- Add regression tests proving API and batch ingestion do not publish to Kafka.

## Acceptance criteria

- No API or batch ingestion endpoint publishes events to Kafka.
- Batch ingestion is direct and does not depend on Kafka, the watcher, or the
  reconciler.
- The watcher and reconciler remain buildable and independently controllable as
  retained legacy/fallback components.
- Temporary Kafka integration CI and runtime wiring are removed or explicitly
  archived.

Related: [PR #115](https://github.com/myersCody/cost_ai_grid_poc/pull/115), [PR #123](https://github.com/myersCody/cost_ai_grid_poc/pull/123)
