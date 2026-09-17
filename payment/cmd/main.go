package main

import (
	"context"
	"log"

	"github.com/Fulim13/microservices-go/payment/config"
	"github.com/Fulim13/microservices-go/payment/internal/adapters/db"
	"github.com/Fulim13/microservices-go/payment/internal/adapters/grpc"
	"github.com/Fulim13/microservices-go/payment/internal/application/core/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func tracerProvider(ctx context.Context, endpoint, service string) (*tracesdk.TracerProvider, error) {
	// WithInsecure because there is no TLS inside the cluster.
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		// Buffers spans and flushes in batches rather than one call per span.
		tracesdk.WithBatcher(exp),
		// These attributes appear on every span, and are how you filter in the UI.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(service),
			attribute.String("environment", config.GetEnv()),
		)),
	)
	return tp, nil
}

func main() {
	ctx := context.Background()

	tp, err := tracerProvider(ctx, config.GetOtelEndpoint(), "payment")
	if err != nil {
		log.Fatalf("Failed to create tracer provider. Error: %v", err)
	}
	// Flush buffered spans on exit, or the last traces are silently lost.
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("tracer shutdown: %v", err)
		}
	}()

	otel.SetTracerProvider(tp)
	// Reads the trace ID out of the gRPC metadata sent by the order service,
	// so payment spans join that trace instead of starting a new one.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	dbAdapter, err := db.NewAdapter(config.GetDataSourceURL())
	if err != nil {
		log.Fatalf("Failed to connect to database. Error: %v", err)
	}

	application := api.NewApplication(dbAdapter)
	grpcAdapter := grpc.NewAdapter(application, config.GetApplicationPort())
	grpcAdapter.Run()
}
