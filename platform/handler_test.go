package platform

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubExtension is a minimal extension for testing.
type stubExtension struct {
	manifest Manifest
	contrib  func(*ContributionContext) error
}

func (e *stubExtension) Manifest() Manifest { return e.manifest }
func (e *stubExtension) Contribute(ctx *ContributionContext) error {
	if e.contrib != nil {
		return e.contrib(ctx)
	}
	return nil
}

func TestNewHandlerAssemblesRoutes(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "test-ext", Label: "Test", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/test"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/test", "test page",
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("hello from test-ext"))
				}))
			return nil
		},
	}

	handler, err := NewHandler(Options{
		Mode:       FullPlatform,
		Extensions: []Extension{ext},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "hello from test-ext")
}

func TestNewHandlerRejectsInvalidOwnership(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "bad-ext", Label: "Bad", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/allowed"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/not-allowed", "", stubHandler())
			return nil
		},
	}

	_, err := NewHandler(Options{
		Mode:       FullPlatform,
		Extensions: []Extension{ext},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside ownership")
}

func TestNewHandlerRejectsConflict(t *testing.T) {
	ext1 := &stubExtension{
		manifest: Manifest{
			ID: "ext-a", Label: "A", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/shared"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/shared", "", stubHandler())
			return nil
		},
	}
	ext2 := &stubExtension{
		manifest: Manifest{
			ID: "ext-b", Label: "B", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/shared"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/shared", "", stubHandler())
			return nil
		},
	}

	_, err := NewHandler(Options{
		Mode:       FullPlatform,
		Extensions: []Extension{ext1, ext2},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "route conflict: GET /shared")
	assert.Contains(t, err.Error(), "ext-a")
	assert.Contains(t, err.Error(), "ext-b")
}

func TestNewHandlerContributeError(t *testing.T) {
	ext := &stubExtension{
		manifest: Manifest{
			ID: "err-ext", Label: "Err", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/x"}},
		},
		contrib: func(ctx *ContributionContext) error {
			return errors.New("extension init failed")
		},
	}

	_, err := NewHandler(Options{
		Mode:       FullPlatform,
		Extensions: []Extension{ext},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extension err-ext")
}

func TestNewHandlerAppliesMiddlewareChain(t *testing.T) {
	called := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}

	ext := &stubExtension{
		manifest: Manifest{
			ID: "mw-ext", Label: "MW", Mode: FullPlatform,
			Ownership: RouteOwnership{UI: []string{"/mw"}},
		},
		contrib: func(ctx *ContributionContext) error {
			ctx.Routes.Protected(http.MethodGet, "/mw", "", stubHandler())
			return nil
		},
	}

	handler, err := NewHandler(Options{
		Mode:            FullPlatform,
		Extensions:      []Extension{ext},
		MiddlewareChain: []func(http.Handler) http.Handler{mw},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/mw", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called, "middleware should have been called")
}
