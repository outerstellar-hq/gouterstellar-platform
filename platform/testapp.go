package platform

import "net/http"

// TestOptions configures a test application.
type TestOptions struct {
	Mode            PlatformMode
	Extensions      []Extension
	MiddlewareChain []func(http.Handler) http.Handler
}

// TestApp is the result of NewTestApp. It exposes the assembled handler so
// Tier 2 in-memory HTTP tests can drive it with httptest without starting
// real servers or databases.
type TestApp struct {
	Handler http.Handler
}

// NewTestApp builds the handler through the same NewHandler assembly path as
// production. This means the same manifest validation, contribution, route
// validation, and router build steps run in tests as in the live wire root.
func NewTestApp(opts TestOptions) (*TestApp, error) {
	handler, err := NewHandler(Options{
		Mode:            opts.Mode,
		Extensions:      opts.Extensions,
		MiddlewareChain: opts.MiddlewareChain,
	})
	if err != nil {
		return nil, err
	}
	return &TestApp{Handler: handler}, nil
}

// Close releases any resources held by the test app. Currently a no-op because
// NewHandler does not open long-lived resources, but it is part of the surface
// so future test infrastructure can add teardown without breaking callers.
func (a *TestApp) Close() {}
