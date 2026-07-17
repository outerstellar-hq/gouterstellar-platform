package platform

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticAssetsPreferFilesystemAndFallBackToPackagedFiles(t *testing.T) {
	overrideDir := t.TempDir()
	extension := &stubExtension{
		manifest: Manifest{
			ID: "assets", Label: "Assets", Mode: FullPlatform,
			Ownership: RouteOwnership{Assets: []string{"/extensions/assets", "/site.css"}},
		},
		contrib: func(ctx *ContributionContext) error {
			source := AssetSource{
				FS: fstest.MapFS{
					"public/site.css":     &fstest.MapFile{Data: []byte("packaged extension")},
					"public/css/main.css": &fstest.MapFile{Data: []byte("packaged platform")},
				},
				Directory: "public",
			}
			if err := ctx.Routes.StaticAssets("/extensions/assets", source); err != nil {
				return err
			}
			return ctx.Routes.StaticFile("/site.css", source, "css/main.css")
		},
	}

	handler, err := NewHandler(Options{
		Mode:       FullPlatform,
		Extensions: []Extension{extension},
		StaticDir:  overrideDir,
		AssetMiddleware: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Asset-Middleware", "applied")
				next.ServeHTTP(w, r)
			})
		},
	})
	require.NoError(t, err)

	assertAsset(t, handler, "/extensions/assets/site.css", "packaged extension")
	assertAsset(t, handler, "/site.css", "packaged platform")

	require.NoError(t, os.WriteFile(filepath.Join(overrideDir, "site.css"), []byte("filesystem override"), 0o600))
	assertAsset(t, handler, "/extensions/assets/site.css", "filesystem override")
	assertAsset(t, handler, "/site.css", "filesystem override")
}

func TestStaticAssetsRejectNilFilesystem(t *testing.T) {
	registry := newRouteRegistry("broken", assetHostOptions{})

	err := registry.StaticAssets("/extensions/broken", AssetSource{})

	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrInvalid)
}

func assertAsset(t *testing.T, handler http.Handler, path, body string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, body, recorder.Body.String())
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/css")
	assert.Equal(t, "applied", recorder.Header().Get("X-Asset-Middleware"))
}
