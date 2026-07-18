// Package observability constructs OpenTelemetry tracing without taking over
// application startup or deployment configuration.
package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// TracingConfig defines a service resource and OTLP/HTTP exporter. An empty
// endpoint delegates endpoint and transport configuration to standard OTel
// environment variables.
type TracingConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	EndpointURL    string
	Headers        map[string]string
	Insecure       bool
	SampleRatio    float64
}

// Tracing owns a tracer provider and its flush/shutdown lifecycle.
type Tracing struct {
	provider *sdktrace.TracerProvider
}

// NewTracing constructs, but does not globally install, an OTLP tracer
// provider. Consumers choose when process-global installation is appropriate.
func NewTracing(ctx context.Context, config TracingConfig) (*Tracing, error) {
	if config.ServiceName == "" {
		return nil, errors.New("tracing service name is required")
	}
	if config.SampleRatio == 0 {
		config.SampleRatio = 1
	}
	if config.SampleRatio < 0 || config.SampleRatio > 1 {
		return nil, errors.New("tracing sample ratio must be between zero and one")
	}

	options := make([]otlptracehttp.Option, 0, 3)
	if config.EndpointURL != "" {
		options = append(options, otlptracehttp.WithEndpointURL(config.EndpointURL))
	}
	if config.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	if len(config.Headers) > 0 {
		options = append(options, otlptracehttp.WithHeaders(config.Headers))
	}
	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	attributes := []attribute.KeyValue{semconv.ServiceName(config.ServiceName)}
	if config.ServiceVersion != "" {
		attributes = append(attributes, semconv.ServiceVersion(config.ServiceVersion))
	}
	if config.Environment != "" {
		attributes = append(attributes, attribute.String("deployment.environment.name", config.Environment))
	}
	serviceResource, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, attributes...))
	if err != nil {
		return nil, fmt.Errorf("create tracing resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(serviceResource),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(config.SampleRatio))),
	)
	return &Tracing{provider: provider}, nil
}

// Provider exposes the standard OTel provider for explicit consumer wiring.
func (t *Tracing) Provider() *sdktrace.TracerProvider {
	return t.provider
}

// InstallGlobal installs this provider and W3C trace-context propagation.
// This explicit call keeps process-global policy out of construction.
func (t *Tracing) InstallGlobal() {
	otel.SetTracerProvider(t.provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
}

// Shutdown flushes and closes the exporter.
func (t *Tracing) Shutdown(ctx context.Context) error {
	if err := t.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown tracing: %w", err)
	}
	return nil
}

// HTTP instruments a handler with standard server spans and propagation.
func HTTP(operation string, next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, operation)
}

// HTTPClient returns a shallow copy of base whose transport creates outbound
// HTTP spans. Nil uses http.DefaultClient as the source configuration.
func HTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	client := *base
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = otelhttp.NewTransport(transport)
	return &client
}
