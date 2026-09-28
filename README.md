# koku-events

Event-based costing for [Red Hat Cost Management](https://github.com/project-koku/koku).

[![CI](https://github.com/project-koku/koku-events/actions/workflows/ci.yml/badge.svg)](https://github.com/project-koku/koku-events/actions/workflows/ci.yml)

Integrates Cost Management with [OSAC](https://github.com/osac-project) (Open Sovereign AI Console) for the AI Grid sovereign cloud offering. This repository continues the accepted cost AI Grid proof of concept, with that history preserved.

Standalone Go service that consumes OSAC CloudEvents, meters infrastructure and MaaS usage, applies rates, and produces cost/quota data.

**Status:** [Implementation tracking](docs/implementation-status.md)

**Docs:** [Technical guide](docs/index.md)

**Ingestion modes:** [Direct OSAC, Kafka experiment, and batch API](docs/ingestion-modes.md)

## Deployment

| Environment | Guide |
|-------------|-------|
| **OpenShift / CRC** | [docs/dev/crc-full-deployment.md](docs/dev/crc-full-deployment.md) — full stack including OSAC + MaaS metering |
| **Local development** | [docs/dev/local-dev-setup.md](docs/dev/local-dev-setup.md) |
| **k3d (integration tests)** | [docs/dev/k3d-ipp-deployment.md](docs/dev/k3d-ipp-deployment.md) |

## License

[Apache License 2.0](LICENSE).
