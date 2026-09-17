FROM golang:1.27 AS builder
WORKDIR /usr/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o payment ./cmd

FROM scratch
COPY --from=builder /usr/src/app/payment /payment

# Required at runtime, set via k8s/deployment-payment.yaml (see README.md):
#   ENV, APPLICATION_PORT, DATA_SOURCE_URL, OTEL_EXPORTER_OTLP_ENDPOINT
EXPOSE 3001
CMD ["/payment"]

# docker build -t payment .
