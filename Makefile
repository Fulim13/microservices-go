# One-command local deployment of the order/payment microservices onto kind.
#
#   make up     -- create the cluster and everything in it, from nothing
#   make down   -- delete the cluster and everything in it
#
# Requires: docker, kind, kubectl, helm (grpcurl only for `make test`).

comma        := ,
CLUSTER      := ecommerce
CONTEXT      := kind-$(CLUSTER)
KIND_CONFIG  := k8s/cluster-ecommerce.yaml
SERVICES     := order payment

# Jaeger is installed from the upstream chart rather than vendored manifests.
# Pinned so `make up` deploys the same version next month as it does today.
JAEGER_CHART_VERSION := 4.13.1
JAEGER_NS            := observability

# Applied in this order: datastore first, so order/payment don't crash-loop
# waiting on a database that isn't up yet.
DB_MANIFESTS  := k8s/deployment-postgress.yaml k8s/svc-postgres.yaml
APP_MANIFESTS := k8s/deployment-payment.yaml k8s/svc-payment.yaml \
                 k8s/deployment-order.yaml k8s/svc-order.yaml \
                 k8s/ingress-nginx.yaml

# Every kubectl call is pinned to this cluster's context, so a stray
# `kubectl config use-context` can never point these at a real cluster.
KUBECTL := kubectl --context $(CONTEXT)

.DEFAULT_GOAL := help
.PHONY: help up down cluster ingress observability images deploy wait \
        redeploy status logs trace test clean

help: ## Show this help
	@echo "Usage: make <target>"
	@echo
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

## ---------------------------------------------------------------- lifecycle

up: cluster ingress observability images deploy wait ## Create the cluster and deploy everything
	@echo
	@echo "Ready. Try it:"
	@echo "  make test                       # place an order"
	@echo "  make trace                      # Jaeger UI on http://localhost:16686"

down: ## Delete the cluster and every resource in it
	@if kind get clusters 2>/dev/null | grep -qx $(CLUSTER); then \
		kind delete cluster --name $(CLUSTER); \
	else \
		echo "Cluster '$(CLUSTER)' does not exist, nothing to delete."; \
	fi

## ------------------------------------------------------------------- stages

cluster: ## Create the kind cluster (no-op if it exists)
	@if kind get clusters 2>/dev/null | grep -qx $(CLUSTER); then \
		echo "Cluster '$(CLUSTER)' already exists, reusing it."; \
	else \
		kind create cluster --config $(KIND_CONFIG); \
	fi

ingress: ## Install the ingress-nginx controller
	$(KUBECTL) apply -f k8s/ingress-controller.yaml
	@echo "Waiting for the ingress controller to become ready..."
	@$(KUBECTL) wait --namespace ingress-nginx \
		--for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller \
		--timeout=300s

observability: ## Install Jaeger (all-in-one, in-memory storage)
	@helm repo add jaegertracing https://jaegertracing.github.io/helm-charts >/dev/null
	@helm repo update jaegertracing >/dev/null
	helm --kube-context $(CONTEXT) upgrade --install jaeger jaegertracing/jaeger \
		--namespace $(JAEGER_NS) --create-namespace \
		--version $(JAEGER_CHART_VERSION) \
		--set provisionDataStore.cassandra=false \
		--set storage.type=memory \
		--set allInOne.enabled=true \
		--set agent.enabled=false \
		--set collector.enabled=false \
		--set query.enabled=false \
		--wait --timeout 5m

images: ## Build the service images and side-load them into kind
	@for svc in $(SERVICES); do \
		echo "==> building $$svc:latest"; \
		docker build -t $$svc:latest ./$$svc || exit 1; \
	done
	kind load docker-image $(addsuffix :latest,$(SERVICES)) --name $(CLUSTER)

deploy: ## Apply the application manifests
	$(KUBECTL) apply $(foreach m,$(DB_MANIFESTS),-f $(m))
	@echo "Waiting for postgres before starting the services..."
	@$(KUBECTL) rollout status deploy/postgres --timeout=300s
	$(KUBECTL) apply $(foreach m,$(APP_MANIFESTS),-f $(m))

wait: ## Block until order and payment are rolled out
	@for svc in $(SERVICES); do \
		$(KUBECTL) rollout status deploy/$$svc --timeout=300s || exit 1; \
	done

## ------------------------------------------------------------------ dev loop

redeploy: images ## Rebuild images and restart the services (skips cluster setup)
	$(KUBECTL) apply $(foreach m,$(APP_MANIFESTS),-f $(m))
	$(KUBECTL) rollout restart $(addprefix deploy/,$(SERVICES))
	@$(MAKE) --no-print-directory wait

status: ## Show what is running
	@$(KUBECTL) get pods,svc,ingress
	@$(KUBECTL) get pods -n $(JAEGER_NS)

logs: ## Tail the order and payment logs (SVC=order to pick one)
	$(KUBECTL) logs -l 'service in ($(subst $() ,$(comma),$(SERVICES)))' \
		--all-containers --tail=50 -f --max-log-requests=10

trace: ## Port-forward the Jaeger UI to http://localhost:16686
	@echo "Jaeger UI: http://localhost:16686  (Ctrl-C to stop)"
	$(KUBECTL) port-forward -n $(JAEGER_NS) svc/jaeger 16686:16686

test: ## Place one order through the order service
	@$(KUBECTL) port-forward svc/order 3000:3000 >/dev/null 2>&1 & \
	pf=$$!; \
	trap "kill $$pf 2>/dev/null" EXIT; \
	until nc -z localhost 3000 2>/dev/null; do sleep 1; done; \
	grpcurl -plaintext -d '{"user_id":123,"order_items":[{"product_code":"item1","unit_price":10,"quantity":2}]}' \
		localhost:3000 Order/Create

clean: ## Remove the application resources but keep the cluster
	-$(KUBECTL) delete $(foreach m,$(APP_MANIFESTS),-f $(m)) --ignore-not-found
	-$(KUBECTL) delete $(foreach m,$(DB_MANIFESTS),-f $(m)) --ignore-not-found
	-helm --kube-context $(CONTEXT) uninstall jaeger -n $(JAEGER_NS)
