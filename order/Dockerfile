FROM golang:1.27 AS builder
WORKDIR /usr/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o order ./cmd

FROM scratch
COPY --from=builder /usr/src/app/order /order

# Required at runtime, set via k8s/deployment-order.yaml (see README.md):
#   ENV, APPLICATION_PORT, DATA_SOURCE_URL, PAYMENT_SERVICE_URL,
#   OTEL_EXPORTER_OTLP_ENDPOINT
EXPOSE 3000
CMD ["/order"]

# docker build -t order .
