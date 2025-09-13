package telemetry

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func InitTracerProvider(ctx context.Context, serviceName, jaegerURL string) (func(), error) {
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
            semconv.ServiceVersion("1.0.0"),
            semconv.DeploymentEnvironmentName("development"),
		),
	)
	if err != nil {
		return nil, err
	}

    // should be replaced with secure credentials
	conn, err := grpc.NewClient(
		jaegerURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, err
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tracerProvider)

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := tracerProvider.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown tracer provider", "error", err)
		}
	}

	return shutdown, nil
}
