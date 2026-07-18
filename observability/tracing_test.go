package observability

import (
	"context"
	"net/http"
	"testing"
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
