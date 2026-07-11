// Package observability wires OpenTelemetry tracing for the platform.
//
// SetupTracing installs a global TracerProvider backed by a stdout exporter
// (suitable for local development). The returned shutdown function must be
// called on process exit to flush any pending spans. When the standard
// OTEL_EXPORTER_OTLP_ENDPOINT variable is set, spans are shipped over OTLP/HTTP
// to a collector instead.
package observability

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

// SetupTracing initialises a global tracer provider with a stdout exporter.
// It returns a shutdown function that must be called on shutdown to flush
// pending spans. If OTEL_EXPORTER_OTLP_ENDPOINT is set, an OTLP HTTP exporter
// is used instead so spans can be shipped to a collector.
func SetupTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	exporter, err := newExporter(ctx)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	slog.Info("OpenTelemetry tracing initialised", "service", serviceName, "exporter", exporterName())
	return tp.Shutdown, nil
}

// newExporter constructs the trace exporter selected by the environment.
func newExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		return otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(endpoint))
	}
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}

// exporterName is a human-friendly label for the configured exporter.
func exporterName() string {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		return "otlp"
	}
	return "stdout"
}
