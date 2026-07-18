package observability

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func TestNewTracingValidatesConfiguration(t *testing.T) {
	if _, err := NewTracing(context.Background(), TracingConfig{}); err == nil {
		t.Fatal("missing service name was accepted")
	}
	if _, err := NewTracing(context.Background(), TracingConfig{ServiceName: "example", SampleRatio: 2}); err == nil {
		t.Fatal("invalid sample ratio was accepted")
	}
}

func TestHTTPClientPreservesCallerConfiguration(t *testing.T) {
	base := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	client := HTTPClient(base)
	if client == base || client.Transport == nil || client.CheckRedirect == nil {
		t.Fatalf("client = %#v", client)
	}
}

func TestGRPCOptionsPropagateOneTraceWithoutGlobalInstallation(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	tracing := &Tracing{provider: provider}

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(tracing.GRPCServerOption())
	healthpb.RegisterHealthServer(server, health.NewServer())
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	connection, err := grpc.NewClient(
		"passthrough:///buffer",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		tracing.GRPCClientOption(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	ctx, parent := provider.Tracer("test").Start(context.Background(), "parent")
	parentTraceID := parent.SpanContext().TraceID()
	_, err = healthpb.NewHealthClient(connection).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	parent.End()

	var clientTraceID, serverTraceID trace.TraceID
	for _, span := range recorder.Ended() {
		switch span.SpanKind() {
		case trace.SpanKindClient:
			clientTraceID = span.SpanContext().TraceID()
		case trace.SpanKindServer:
			serverTraceID = span.SpanContext().TraceID()
		}
	}
	if !clientTraceID.IsValid() || !serverTraceID.IsValid() {
		t.Fatalf("missing client or server span: client=%s server=%s", clientTraceID, serverTraceID)
	}
	if clientTraceID != parentTraceID || serverTraceID != parentTraceID {
		t.Fatalf("trace propagation failed: parent=%s client=%s server=%s", parentTraceID, clientTraceID, serverTraceID)
	}
}

func TestPostgreSQLTracerOmitsStatementAndUsesOperationName(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	tracing := &Tracing{provider: provider}
	postgres := tracing.PostgreSQLTracer()

	ctx, parent := provider.Tracer("test").Start(context.Background(), "parent")
	queryContext := postgres.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{
		SQL:  "SELECT password_hash FROM users WHERE email = $1",
		Args: []any{"private@example.test"},
	})
	postgres.TraceQueryEnd(queryContext, nil, pgx.TraceQueryEndData{})
	parent.End()

	var querySpan sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.SpanKind() == trace.SpanKindClient {
			querySpan = span
			break
		}
	}
	if querySpan == nil {
		t.Fatal("PostgreSQL query span was not recorded")
	}
	if querySpan.Name() != "query SELECT" {
		t.Fatalf("span name = %q, want query SELECT", querySpan.Name())
	}
	for _, value := range querySpan.Attributes() {
		if value.Key == attribute.Key("db.query.text") || value.Key == attribute.Key("pgx.query.parameters") {
			t.Fatalf("sensitive query attribute recorded: %s", value.Key)
		}
		if rendered := value.Value.String(); strings.Contains(rendered, "password_hash") || strings.Contains(rendered, "private@example.test") {
			t.Fatalf("sensitive query value recorded by %s", value.Key)
		}
	}
}
