# Microservices in Go — Order & Payment

Two gRPC services in Go, built on a hexagonal (ports & adapters) architecture,
running on a local `kind` Kubernetes cluster with distributed tracing into
Jaeger. Placing an order calls the payment service over gRPC, and the whole
request shows up in Jaeger as a single trace across both services.

```
grpcurl ──▶ order.Create ──▶ payment.Create ──▶ postgres
                  └──────────── traces ───────────▶ jaeger
```

## Features

| Feature                                                                                                                                                                              | Where it lives                                                                  |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- |
| **Hexagonal architecture** — the application core depends only on port interfaces; gRPC, GORM and the payment client are all swappable adapters                                      | `internal/ports`, `internal/application/core`, `internal/adapters`              |
| **gRPC APIs** generated from a shared protobuf repo, consumed as a Go module by both services                                                                                        | [`Fulim13/microservices-proto`](https://github.com/Fulim13/microservices-proto) |
| **Order placement flow** — persist the order, then charge it through the payment service, failing the order if the charge is rejected                                                | `order/internal/application/core/api/api.go`                                    |
| **Rich validation errors** — payment returns field-level violations as `google.rpc.BadRequest` details, which order unpacks and re-reports against its own `payment` field           | `payment/internal/adapters/grpc/grpc.go`, `order/.../api/api.go`                |
| **Circuit breaker** — a unary client interceptor trips the order→payment circuit once the failure ratio crosses 60%, so requests fail fast instead of piling up on a sick dependency | `order/internal/adapters/payment/payment.go`                                    |
| **Distributed tracing** — both services export OTLP spans to Jaeger; W3C trace context rides in gRPC metadata so payment spans join the order trace rather than starting their own   | `*/cmd/main.go`, both gRPC adapters                                             |
| **Database per service** — `order` and `payment` are separate logical databases, with GORM auto-migration on startup                                                                 | `*/internal/adapters/db/db.go`                                                  |
| **Minimal images** — multi-stage builds producing static binaries on `scratch`                                                                                                       | `*/Dockerfile`                                                                  |
| **Reproducible cluster** — a 4-node `kind` cluster, ingress-nginx configured for gRPC, and Jaeger, all from one command                                                              | `Makefile`, `k8s/`                                                              |

## Architecture

### Runtime topology

```mermaid
flowchart TB
    client["grpcurl / gRPC client"]

    subgraph cluster["kind cluster: ecommerce"]
        subgraph ingressns["namespace: ingress-nginx"]
            ing["ingress-nginx controller<br/>hostPort 80 / 443<br/>backend-protocol: GRPC"]
        end

        subgraph appns["namespace: default"]
            osvc["Service: order<br/>:3000"]
            order["Deployment: order<br/>3 replicas"]
            psvc["Service: payment<br/>:3001"]
            payment["Deployment: payment<br/>3 replicas"]
            pgsvc["Service: postgres<br/>:5432"]
            pg["Deployment: postgres<br/>postgres:16-alpine<br/>databases: order, payment"]
        end

        subgraph obs["namespace: observability"]
            jaeger["Jaeger all-in-one<br/>OTLP :4318 · UI :16686<br/>in-memory storage"]
        end
    end

    client -->|"Order/Create"| ing
    ing --> osvc --> order
    order -->|"Payment/Create<br/>gRPC + traceparent"| psvc --> payment
    order -->|SQL| pgsvc
    payment -->|SQL| pgsvc
    pgsvc --> pg
    order -.->|"OTLP/HTTP spans"| jaeger
    payment -.->|"OTLP/HTTP spans"| jaeger
```

### Hexagonal layout (both services follow this shape)

The core never imports an adapter. It only knows the interfaces in `ports`,
which is what lets the gRPC server, the database and the payment client be
replaced without touching business logic.

```mermaid
flowchart LR
    subgraph driving["Driving adapters"]
        grpcin["gRPC server<br/>adapters/grpc"]
    end

    subgraph core["Application core"]
        apiport(["ports.APIPort"])
        app["api.Application<br/>business logic<br/>+ domain"]
        dbport(["ports.DBPort"])
        payport(["ports.PaymentPort"])
    end

    subgraph driven["Driven adapters"]
        dbad["GORM + Postgres<br/>adapters/db"]
        payad["payment gRPC client<br/>+ circuit breaker<br/>adapters/payment"]
    end

    grpcin --> apiport --> app
    app --> dbport --> dbad
    app --> payport --> payad
```

### Order placement sequence

```mermaid
sequenceDiagram
    participant C as Client
    participant O as order
    participant P as payment
    participant DB as postgres
    participant J as jaeger

    C->>O: Order/Create(user_id, order_items)
    Note over O: span Order/Create starts
    O->>DB: INSERT order + order_items
    DB-->>O: order id
    O->>P: Payment/Create(user_id, order_id, total_price)<br/>traceparent in gRPC metadata
    Note over P: span joins the same trace
    P->>P: validate user_id, order_id, total_price
    alt validation fails
        P-->>O: InvalidArgument + BadRequest details
        O-->>C: InvalidArgument("order creation failed")<br/>with field violations
    else ok
        P->>DB: INSERT payment
        P-->>O: payment id
        O-->>C: order_id
    end
    O-->>J: export spans (batched, OTLP/HTTP)
    P-->>J: export spans (batched, OTLP/HTTP)
```

## Frameworks and tools

**Language & runtime**

- **Go 1.27** — both services, standard `cmd/` + `internal/` layout

**Service communication**

- **gRPC** (`google.golang.org/grpc`) — server and client, with a stats
  handler and a unary client interceptor
- **Protocol Buffers** (`google.golang.org/protobuf`) — contracts published as
  a separate Go module
- **`google.golang.org/genproto/.../errdetails`** — typed `BadRequest` error
  details on the wire

**Persistence**

- **PostgreSQL 16** (`postgres:16-alpine`)
- **GORM** (`gorm.io/gorm`) with the **pgx**-backed `gorm.io/driver/postgres`,
  including `AutoMigrate` on startup

**Resilience**

- **`sony/gobreaker`** — circuit breaker around the order→payment call

**Observability**

- **OpenTelemetry Go SDK** — tracer provider, batching span processor,
  resource attributes (`service.name`, `environment`)
- **`otelgrpc`** — automatic server and client spans for gRPC
- **OTLP/HTTP exporter** (`otlptracehttp`) — ships spans to the collector
- **Jaeger 2.x** (all-in-one, in-memory) — installed from the official Helm
  chart, pinned to `4.13.1`

**Build & deployment**

- **Docker** — multi-stage builds, `scratch` final images
- **kind** — 4-node local cluster (1 control-plane + 3 workers)
- **kubectl** + **Helm** — manifests in `k8s/`, Jaeger via chart
- **ingress-nginx 1.15.1** — with `backend-protocol: GRPC`
- **GNU Make** — one-command lifecycle
- **grpcurl** — manual testing

## How to run the application with make commands

This is the fast path. `make up` takes you from nothing to a running system:
it creates the `kind` cluster, installs ingress-nginx and Jaeger, builds and
side-loads both images, and applies the manifests datastore-first.

```sh
make up     # create the cluster and deploy everything
make test   # place one order through the order service
make trace  # port-forward the Jaeger UI to http://localhost:16686
make down   # delete the cluster and every resource in it
```

Run `make` on its own to list every target.

### All targets

| Target               | What it does                                                                       |
| -------------------- | ---------------------------------------------------------------------------------- |
| `make up`            | Full setup: `cluster` → `ingress` → `observability` → `images` → `deploy` → `wait` |
| `make down`          | Delete the whole `kind` cluster                                                    |
| `make cluster`       | Create the `kind` cluster (no-op if it already exists)                             |
| `make ingress`       | Install the ingress-nginx controller and wait for it                               |
| `make observability` | Install Jaeger via Helm into the `observability` namespace                         |
| `make images`        | `docker build` both services and `kind load` them                                  |
| `make deploy`        | Apply postgres, wait for it, then apply order, payment and the ingress             |
| `make wait`          | Block until both service deployments have rolled out                               |
| `make redeploy`      | Rebuild images and restart the services — the inner dev loop                       |
| `make status`        | Show pods, services and ingress                                                    |
| `make logs`          | Follow the order and payment logs                                                  |
| `make trace`         | Port-forward the Jaeger UI                                                         |
| `make test`          | Place one order with `grpcurl`                                                     |
| `make clean`         | Remove the application resources but keep the cluster                              |

After a code change you don't need a full teardown — `make redeploy` rebuilds
the images and restarts the deployments in place.

**Requirements:** `docker`, `kind`, `kubectl`, `helm`, and `make`. `grpcurl` is
only needed for `make test`. Ports 80 and 443 on the host must be free, since
the cluster maps them to the ingress controller.

### Seeing a trace

```sh
make test
make trace   # then open http://localhost:16686 and pick "order" in Service
```

One order produces a single trace with three spans: `Order/Create` on order,
the client-side `Payment/Create` on order, and the server-side `Payment/Create`
on payment.

## How to run in local

```sh
# Start PostgreSQL
docker run --name postgres \
  -p 5432:5432 \
  -e POSTGRES_PASSWORD=verysecretpass \
  -e POSTGRES_DB=order \
  -d postgres

# Create payment database
docker exec -it postgres \
  psql -U postgres \
  -c "CREATE DATABASE payment;"
```

Both services require `OTEL_EXPORTER_OTLP_ENDPOINT` and will exit at startup
without it, so run a local Jaeger to export to:

```sh
docker run -d --name jaeger \
  -p 16686:16686 -p 4318:4318 \
  jaegertracing/all-in-one:1.60
```

Then, from `payment/`:

```sh
DATA_SOURCE_URL="postgres://postgres:verysecretpass@127.0.0.1:5432/payment?sslmode=disable" \
	APPLICATION_PORT=3001 \
	ENV=development \
	OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318 \
	go run cmd/main.go
```

and from `order/`:

```sh
DATA_SOURCE_URL="postgres://postgres:verysecretpass@127.0.0.1:5432/order?sslmode=disable" \
	APPLICATION_PORT=3000 \
	ENV=development \
	PAYMENT_SERVICE_URL=localhost:3001 \
	OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318 \
	go run cmd/main.go
```

```sh
grpcurl \
	-d '{"user_id": 123, "order_items": [{"product_code": "prod", "quantity": 4, "unit_price": 12}]}' \
	-plaintext \
	localhost:3000 \
	Order/Create
```

To see the validation path, send values the payment service rejects:

```sh
grpcurl \
	-d '{"user_id": -1, "order_items": [{"product_code": "prod", "quantity": 4, "unit_price": -1}]}' \
	-plaintext \
	localhost:3000 \
	Order/Create
```

## How to run in Kubernetes

`make up` does all of this. The steps below are the same thing by hand, for
when you want to see them individually.

### Create Cluster

```sh
kind create cluster --config k8s/cluster-ecommerce.yaml
```

### Build the images (from the repo root)

```sh
docker build -t order:latest ./order
docker build -t payment:latest ./payment
```

### Load Image to Cluster

```sh
kind load docker-image order:latest --name ecommerce
kind load docker-image payment:latest --name ecommerce
```

### Label the control-plane node

Only this node has the hostPort 80/443 mappings, so the ingress controller has
to land there. `k8s/cluster-ecommerce.yaml` now applies this label at cluster
creation, so this step is only needed for a cluster created before that.

```sh
kubectl label nodes ecommerce-control-plane ingress-ready=true
```

### Apply ingress-controller

```sh
kubectl apply -f k8s/ingress-controller.yaml

kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s
```

### Apply Postgres

Apply this first: order and payment resolve `postgres` by name.

```sh
kubectl apply -f k8s/deployment-postgress.yaml
kubectl apply -f k8s/svc-postgres.yaml
```

### Apply deployment/service of order and payment

```sh
kubectl apply -f k8s/deployment-order.yaml
kubectl apply -f k8s/deployment-payment.yaml
kubectl apply -f k8s/svc-order.yaml
kubectl apply -f k8s/svc-payment.yaml
```

### Apply the ingress

```sh
kubectl apply -f k8s/ingress-nginx.yaml
```

### Install Jaeger

Both services export OTLP traces over HTTP to this release. The all-in-one
chart puts the collector and the query UI behind a single `jaeger` Service.

```sh
helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
helm upgrade --install jaeger jaegertracing/jaeger \
  --namespace observability --create-namespace \
  --version 4.13.1 \
  --set provisionDataStore.cassandra=false \
  --set storage.type=memory \
  --set allInOne.enabled=true \
  --set agent.enabled=false \
  --set collector.enabled=false \
  --set query.enabled=false

kubectl port-forward -n observability svc/jaeger 16686:16686
```

### Check everything is wired up

```sh
kubectl get pods

# None of these may show <none>
kubectl get endpoints order payment postgres
```

### Test with grpcurl

gRPC needs HTTP/2, and ingress-nginx only speaks HTTP/2 on its TLS (443)
listener, so plaintext gRPC will not traverse the ingress. Port-forward
straight to the order Service instead.

```sh
kubectl port-forward svc/order 3000:3000

grpcurl \
	-d '{"user_id": 123, "order_items": [{"product_code": "prod", "quantity": 4, "unit_price": 12}]}' \
	-plaintext \
	localhost:3000 \
	Order/Create
```
