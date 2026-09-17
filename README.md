# Run Microservice

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

DATA_SOURCE_URL="postgres://postgres:verysecretpass@127.0.0.1:5432/payment?sslmode=disable" \
	APPLICATION_PORT=3001 \
	ENV=development \
	go run cmd/main.go

DATA_SOURCE_URL="postgres://postgres:verysecretpass@127.0.0.1:5432/order?sslmode=disable" \
	APPLICATION_PORT=3000 \
	ENV=development \
	PAYMENT_SERVICE_URL=localhost:3001 \
	go run cmd/main.go

grpcurl \
	-d '{"user_id": 123, "order_items": [{"product_code": "prod", "quantity": 4, "unit_price": 12}]}' \
	-plaintext \
	localhost:3000 \
	Order/Create
```

## How to run in Kubernetes

### The short version

```sh
make up     # create the cluster, install ingress-nginx + Jaeger, build and deploy
make test   # place one order
make trace  # Jaeger UI on http://localhost:16686
make down   # delete the cluster and everything in it
```

`make` on its own lists every target. The rest of this section is the same
thing done by hand, for when you want to see the individual steps.

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
