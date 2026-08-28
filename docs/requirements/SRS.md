# Software Requirements Specification

**Project:** GoGrpcKafkaMicroservice  
**Version:** 1.0.0  
**Date:** 2026-08-28  
**License:** MIT  
**Status:** Draft — awaiting System Architect review

---

## 1. Overview / Purpose

This project is a portfolio-quality Go microservice that demonstrates two integration patterns common in production systems:

1. **Synchronous front-end API** — a gRPC server that allows clients to create and query *Order* resources.
2. **Asynchronous back-end messaging** — a Kafka producer that publishes domain events when orders change state, and a Kafka consumer that reacts to those events.

The primary audience is a technical hiring reviewer. The service must be self-contained and runnable on a developer laptop via `docker compose up`, requiring no external infrastructure beyond Docker.

---

## 2. Domain Model

A single domain entity, **Order**, is used throughout:

| Field        | Type    | Notes                                         |
|--------------|---------|-----------------------------------------------|
| `id`         | string  | UUID v4, assigned by the service              |
| `item`       | string  | Human-readable item description, 1–200 chars  |
| `quantity`   | uint32  | Positive integer, minimum 1                   |
| `status`     | enum    | `PENDING`, `CONFIRMED`, `CANCELLED`           |
| `created_at` | int64   | Unix epoch seconds, set at creation time      |

---

## 3. Functional Requirements

### 3.1 gRPC API

**FR-01** The service shall expose a gRPC server on a configurable TCP port (default `50051`).

**FR-02** The gRPC API shall define a `OrderService` with the following RPCs:

| RPC              | Request message    | Response message   | Behaviour                                                                   |
|------------------|--------------------|--------------------|-----------------------------------------------------------------------------|
| `CreateOrder`    | `CreateOrderRequest`  | `CreateOrderResponse` | Validates input, persists the order in memory, returns the created Order.  |
| `GetOrder`       | `GetOrderRequest`     | `GetOrderResponse`    | Returns the Order for the given `id`; returns gRPC `NOT_FOUND` if absent. |
| `ListOrders`     | `ListOrdersRequest`   | `ListOrdersResponse`  | Returns all orders; supports an optional `status` filter.                  |
| `UpdateOrderStatus` | `UpdateOrderStatusRequest` | `UpdateOrderStatusResponse` | Transitions an order's `status`; returns `NOT_FOUND` or `INVALID_ARGUMENT` on illegal transitions. |

**FR-03** `CreateOrder` shall validate that `item` is non-empty and `quantity ≥ 1`; it shall return gRPC status `INVALID_ARGUMENT` with a descriptive message on validation failure.

**FR-04** `UpdateOrderStatus` shall enforce the following legal state transitions only:

```
PENDING → CONFIRMED
PENDING → CANCELLED
CONFIRMED → CANCELLED
```

Any other transition shall return gRPC status `INVALID_ARGUMENT`.

**FR-05** The service shall return gRPC status `INTERNAL` (with no sensitive details) for unexpected storage errors.

### 3.2 Kafka Producer

**FR-06** After a successful `CreateOrder` call the service shall publish an `order.created` event to the Kafka topic `orders`.

**FR-07** After a successful `UpdateOrderStatus` call the service shall publish an `order.status_updated` event to the topic `orders`.

**FR-08** Each Kafka message shall use the Order `id` as the message key (for partition locality) and carry a JSON payload conforming to the following schema:

```json
{
  "event_type": "<order.created | order.status_updated>",
  "order_id":   "<uuid>",
  "status":     "<PENDING | CONFIRMED | CANCELLED>",
  "timestamp":  1234567890
}
```

**FR-09** Kafka production shall be fire-and-forget from the gRPC handler's perspective; a production failure shall be logged but shall not fail the gRPC response.

### 3.3 Kafka Consumer

**FR-10** The service shall run a background Kafka consumer that subscribes to the `orders` topic and logs each received event (event type, order id, status, timestamp) at INFO level.

**FR-11** The consumer shall use a consumer group id of `order-processor`.

**FR-12** On startup, the consumer shall begin consuming from the earliest available offset if no committed offset exists for the group.

**FR-13** The consumer goroutine shall shut down gracefully when the service receives `SIGINT` or `SIGTERM`, committing its current offsets before exiting.

### 3.4 In-Memory Storage

**FR-14** Order state shall be held in an in-memory map for the lifetime of the process; persistence across restarts is explicitly out of scope (see Section 6).

**FR-15** The in-memory store shall be safe for concurrent access from the gRPC server goroutines and the consumer goroutine.

### 3.5 Configuration

**FR-16** All tunable parameters shall be readable from environment variables with sensible defaults:

| Variable          | Default            | Description                  |
|-------------------|--------------------|------------------------------|
| `GRPC_PORT`       | `50051`            | gRPC server listen port      |
| `KAFKA_BROKERS`   | `localhost:9092`   | Comma-separated broker list  |
| `KAFKA_TOPIC`     | `orders`           | Kafka topic name             |
| `KAFKA_GROUP_ID`  | `order-processor`  | Consumer group id            |
| `LOG_LEVEL`       | `info`             | Logging level (info/debug)   |

### 3.6 Observability

**FR-17** The service shall emit structured JSON logs to stdout with at minimum the fields: `level`, `timestamp`, `message`.

**FR-18** The service shall expose an HTTP health-check endpoint `GET /healthz` on port `8080` that returns `200 OK` with body `{"status":"ok"}` when the gRPC server and Kafka producer are ready.

---

## 4. Non-Functional Requirements

**NFR-01 Language & Runtime:** The service shall be written in Go ≥ 1.22. The `go.mod` minimum version directive shall reflect this.

**NFR-02 Protobuf tooling:** The gRPC service contract shall be defined in `.proto` files under `proto/`. Generated Go stubs shall be committed to the repository under `gen/`.

**NFR-03 Dependencies:** Direct dependencies shall be limited to:
- `google.golang.org/grpc` — gRPC runtime
- `google.golang.org/protobuf` — Protobuf runtime
- A single Kafka client library (e.g. `github.com/segmentio/kafka-go` or `github.com/confluentinc/confluent-kafka-go`)
- A structured logger (e.g. `log/slog` from the standard library, or `go.uber.org/zap`)
- `github.com/google/uuid` — UUID generation
- `github.com/stretchr/testify` — test assertions

No ORM, no web framework, no service mesh sidecar.

**NFR-04 Build:** `go build ./...` shall succeed with zero warnings. A `Makefile` shall provide targets: `build`, `test`, `lint`, `proto`, and `docker-up`.

**NFR-05 Test coverage:** Unit tests shall achieve ≥ 80 % statement coverage on business-logic packages (store, domain validation, event marshalling). Integration tests using a real Kafka broker (via Docker Compose) are welcome but not required for the coverage threshold.

**NFR-06 Linting:** The codebase shall pass `golangci-lint run` with the default ruleset and zero reported issues.

**NFR-07 Concurrency safety:** The data-race detector (`go test -race ./...`) shall report no races.

**NFR-08 Containerisation:** A `Dockerfile` using a multi-stage build (builder image → `gcr.io/distroless/static` or `alpine`) shall produce an image ≤ 50 MB.

**NFR-09 Startup time:** The service shall be ready to serve gRPC requests within 5 seconds of container start on a standard developer laptop (4-core CPU, 8 GB RAM).

**NFR-10 Graceful shutdown:** On `SIGINT`/`SIGTERM`, the service shall stop accepting new gRPC connections, drain in-flight RPCs (up to 10 s timeout), commit Kafka consumer offsets, and then exit with code 0.

---

## 5. Docker Compose Scope

**DC-01** The `docker-compose.yml` shall include the following services:

| Service         | Image                                  | Exposed host port | Purpose                         |
|-----------------|----------------------------------------|-------------------|---------------------------------|
| `zookeeper`     | `confluentinc/cp-zookeeper:7.6`        | `2181`            | Kafka coordination (internal)   |
| `kafka`         | `confluentinc/cp-kafka:7.6`            | `9092`            | Kafka broker                    |
| `kafka-ui`      | `provectuslabs/kafka-ui:latest`        | `8090`            | Browser-based topic inspection  |
| `app`           | Built from local `Dockerfile`          | `50051`, `8080`   | The Go microservice itself      |

**DC-02** The `kafka` service shall define a health-check using `kafka-broker-api-versions` (or equivalent) so dependent services wait for broker readiness.

**DC-03** The `app` service shall declare `depends_on: kafka: condition: service_healthy`.

**DC-04** The Compose file shall create the `orders` topic automatically via a one-shot init container or Kafka `KAFKA_CREATE_TOPICS` environment variable before the `app` starts.

**DC-05** All inter-service communication shall use Docker bridge networking; no host-network mode.

**DC-06** Persistent Kafka data volumes are not required (dev-only environment).

---

## 6. Out of Scope

The following items are explicitly excluded from this iteration:

1. **Persistent storage** — no database, no disk-backed store.
2. **Authentication / authorisation** — no TLS, no mTLS, no JWT on gRPC calls.
3. **Schema Registry** — Kafka payloads are plain JSON; Avro/Protobuf serialisation via a registry is not required.
4. **Kubernetes manifests** — Helm charts, Kustomize overlays, or any k8s YAML.
5. **CI/CD pipeline** — GitHub Actions workflows or any other pipeline configuration.
6. **Distributed tracing / metrics** — OpenTelemetry, Prometheus, Grafana.
7. **Multiple services** — this is a single-binary monolith for demonstration; no service mesh.
8. **gRPC streaming RPCs** — all four RPCs are unary; server-streaming `WatchOrders` is a potential future enhancement.
9. **Dead-letter queue / retry logic** — Kafka consumer failures are logged and skipped.

---

## 7. Acceptance Criteria

| ID    | Criterion                                                                                                              |
|-------|------------------------------------------------------------------------------------------------------------------------|
| AC-01 | `docker compose up --build` starts all four services with no errors; Kafka UI is reachable at `http://localhost:8090`. |
| AC-02 | A `grpcurl` or generated client calling `CreateOrder` returns a populated `Order` with a UUID `id`.                    |
| AC-03 | The Kafka UI shows a message on topic `orders` after each `CreateOrder` or `UpdateOrderStatus` call.                   |
| AC-04 | Calling `GetOrder` with the returned UUID returns the same order.                                                      |
| AC-05 | Calling `UpdateOrderStatus` with an illegal transition returns gRPC `INVALID_ARGUMENT`.                                |
| AC-06 | `go test -race -cover ./...` reports ≥ 80 % coverage and zero races.                                                  |
| AC-07 | `golangci-lint run` exits 0.                                                                                           |
| AC-08 | Sending `SIGTERM` to the `app` container results in a clean exit (code 0) within 15 seconds.                           |
| AC-09 | `docker build .` produces an image ≤ 50 MB.                                                                            |

---

## 8. Glossary

| Term             | Definition                                                                                          |
|------------------|-----------------------------------------------------------------------------------------------------|
| **gRPC**         | Remote Procedure Call framework using HTTP/2 and Protocol Buffers, developed by Google.             |
| **Kafka**        | Distributed event-streaming platform; messages are organised into *topics* and *partitions*.        |
| **Topic**        | A named, append-only log in Kafka; producers write to it, consumers read from it.                   |
| **Consumer group** | A set of consumers that cooperatively read a topic; each partition is assigned to one member.     |
| **Offset**       | The sequential position of a message within a Kafka partition.                                      |
| **Protobuf**     | Protocol Buffers — Google's language-neutral binary serialisation format used to define gRPC contracts. |
| **Order**        | The sole domain entity in this service; represents a purchase request with a lifecycle status.      |
| **SRS**          | Software Requirements Specification — this document.                                                |
| **FR**           | Functional Requirement — a statement of what the system must do.                                    |
| **NFR**          | Non-Functional Requirement — a quality attribute the system must exhibit.                           |
| **DC**           | Docker Compose requirement — a constraint on the local development environment definition.          |
| **AC**           | Acceptance Criterion — a verifiable condition that confirms a requirement is satisfied.             |
| **UUID v4**      | Universally Unique Identifier, version 4 (random); used as the Order primary key.                  |
| **distroless**   | A minimal base container image from Google containing only the application runtime, no shell/tools. |
