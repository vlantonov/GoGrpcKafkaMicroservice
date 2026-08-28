# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Fixed

- Added `permissions: contents: write` to the release workflow job to resolve a 403 error when `softprops/action-gh-release` creates a GitHub Release
- Bumped `actions/checkout` to v7, `actions/setup-go` to v7, and `softprops/action-gh-release` to v3 to eliminate Node 20 deprecation warnings (actions now run on Node 24)
- Replaced removed `run.skip-dirs` with `issues.exclude-dirs` in `.golangci.yml` to fix config schema validation failure on golangci-lint v1.57+

## [0.1.0] - 2026-08-28

### Added

- gRPC `OrderService` with `CreateOrder`, `GetOrder`, `ListOrders`, and `UpdateOrderStatus` RPCs on port `50051`
- Kafka producer publishes `OrderEvent` JSON to the `orders` topic on every mutating RPC (`order.created`, `order.status_updated`)
- Kafka consumer group `order-processor` logs received events at INFO level; starts from the earliest offset on first run
- Thread-safe in-memory order store (`sync.RWMutex`) with sentinel errors `ErrNotFound` and `ErrInvalidStatus`
- State-transition enforcement: `PENDING → CONFIRMED`, `PENDING → CANCELLED`, `CONFIRMED → CANCELLED`; illegal transitions return gRPC `INVALID_ARGUMENT`
- HTTP `/healthz` endpoint on port `8080` returning `{"status":"ok"}`
- Docker Compose stack: Zookeeper, Kafka, `kafka-init` (one-shot topic creation), Kafka-UI (port `8090`), and the app service
- Multi-stage `Dockerfile` — builder stage `golang:1.22-alpine`, final stage `gcr.io/distroless/static`
- GitHub Actions workflows: CI (build, test, coverage), lint (`golangci-lint`), release (binary upload on tag push)
- `Makefile` targets: `proto`, `build`, `test`, `lint`, `docker-up`, `docker-down`, `help`
- Configurable via environment variables: `GRPC_PORT`, `KAFKA_BROKERS`, `KAFKA_TOPIC`, `KAFKA_GROUP_ID`, `LOG_LEVEL`, `HEALTH_PORT`
- Structured JSON logging via `log/slog` (stdlib); fields: `level`, `time`, `msg`
- Graceful shutdown on `SIGINT`/`SIGTERM`: gRPC drain (10 s), consumer offset commit, producer flush, HTTP server stop

[Unreleased]: https://github.com/vlantonov/GoGrpcKafkaMicroservice/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/vlantonov/GoGrpcKafkaMicroservice/releases/tag/v0.1.0
