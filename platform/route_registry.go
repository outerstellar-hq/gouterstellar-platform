package platform

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

// RouteGroup categorises a route so the wire root can apply the right
// middleware and so headless mode knows which groups to drop.
type RouteGroup string

const (
	GroupPublicUI    RouteGroup = "public-ui"
	GroupProtectedUI RouteGroup = "protected-ui"
	GroupAPI         RouteGroup = "api"
	GroupAdmin       RouteGroup = "admin"
	GroupAssets      RouteGroup = "assets"
)

// RouteRegistration is a single route collected from an extension's
// Contribute callback. The Owner field is stamped from the registry.
type RouteRegistration struct {
	Owner       string
	Method      string
	Pattern     string
	Group       RouteGroup
	Description string
	Handler     http.Handler
}

// RouteRegistry collects route registrations for a single extension owner.
// Each helper method stamps the owner so extensions never set it by hand.
type RouteRegistry struct {
	owner           string
	routes          []RouteRegistration
	staticDir       string
	assetMiddleware func(http.Handler) http.Handler
}

type assetHostOptions struct {
	staticDir       string
	assetMiddleware func(http.Handler) http.Handler
}

func newRouteRegistry(owner string, assets assetHostOptions) *RouteRegistry {
	return &RouteRegistry{
		owner:           owner,
		staticDir:       assets.staticDir,
		assetMiddleware: assets.assetMiddleware,
	}
}

func (r *RouteRegistry) Public(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupPublicUI)
}

func (r *RouteRegistry) Protected(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupProtectedUI)
}

func (r *RouteRegistry) API(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupAPI)
}

func (r *RouteRegistry) Admin(method, pattern, desc string, h http.Handler) {
	r.add(method, pattern, desc, h, GroupAdmin)
}

func (r *RouteRegistry) Assets(pattern string, h http.Handler) {
	if r.assetMiddleware != nil {
		h = r.assetMiddleware(h)
	}
	r.add(http.MethodGet, pattern, "static assets", h, GroupAssets)
}

// AssetSource identifies an extension-owned directory inside an fs.FS.
type AssetSource struct {
	FS        fs.FS
	Directory string
}

// StaticAssets mounts an extension-owned filesystem below pathPrefix. Files
// from the configured host override directory take precedence; absent files
// fall back to the extension's packaged filesystem.
func (r *RouteRegistry) StaticAssets(pathPrefix string, source AssetSource) error {
	assets, err := assetSub(source)
	if err != nil {
		return fmt.Errorf("extension %s static assets: %w", r.owner, err)
	}
	prefix := "/" + strings.Trim(strings.TrimSpace(pathPrefix), "/")
	if prefix == "/" {
		return fmt.Errorf("extension %s static asset path prefix must not be empty", r.owner)
	}
	handler := http.FileServer(http.FS(filesystemFirst(r.staticDir, assets)))
	r.Assets(prefix+"/*", http.StripPrefix(prefix+"/", handler))
	return nil
}

// StaticFile mounts one public file whose packaged name may differ from its
// URL. The external override uses the public URL name.
func (r *RouteRegistry) StaticFile(path string, source AssetSource, packagedName string) error {
	assets, err := assetSub(source)
	if err != nil {
		return fmt.Errorf("extension %s static file: %w", r.owner, err)
	}
	publicName := strings.TrimPrefix(strings.TrimSpace(path), "/")
	if !fs.ValidPath(publicName) || !fs.ValidPath(packagedName) {
		return fmt.Errorf("extension %s static file path is invalid", r.owner)
	}
	handler := http.FileServer(http.FS(&mappedAssetFS{
		primary:      optionalDirFS(r.staticDir),
		fallback:     assets,
		publicName:   publicName,
		packagedName: packagedName,
	}))
	r.Assets("/"+publicName, handler)
	return nil
}

func assetSub(source AssetSource) (fs.FS, error) {
	if source.FS == nil {
		return nil, fmt.Errorf("%w: filesystem is nil", fs.ErrInvalid)
	}
	directory := strings.TrimSpace(source.Directory)
	if directory == "" || directory == "." {
		return source.FS, nil
	}
	assets, err := fs.Sub(source.FS, directory)
	if err != nil {
		return nil, fmt.Errorf("open directory %q: %w", directory, err)
	}
	return assets, nil
}

type fallbackFS struct {
	primary  fs.FS
	fallback fs.FS
}

func filesystemFirst(directory string, fallback fs.FS) fs.FS {
	primary := optionalDirFS(directory)
	if primary == nil {
		return fallback
	}
	return &fallbackFS{primary: primary, fallback: fallback}
}

func optionalDirFS(directory string) fs.FS {
	if strings.TrimSpace(directory) == "" {
		return nil
	}
	return os.DirFS(directory)
}

func (f *fallbackFS) Open(name string) (fs.File, error) {
	file, err := f.primary.Open(name)
	if err == nil {
		return file, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return f.fallback.Open(name)
}

type mappedAssetFS struct {
	primary      fs.FS
	fallback     fs.FS
	publicName   string
	packagedName string
}

func (f *mappedAssetFS) Open(name string) (fs.File, error) {
	if name != f.publicName {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if f.primary != nil {
		file, err := f.primary.Open(name)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return f.fallback.Open(f.packagedName)
}

func (r *RouteRegistry) add(method, pattern, desc string, h http.Handler, group RouteGroup) {
	r.routes = append(r.routes, RouteRegistration{
		Owner:       r.owner,
		Method:      method,
		Pattern:     pattern,
		Group:       group,
		Description: desc,
		Handler:     h,
	})
}

// All returns every registration collected so far.
func (r *RouteRegistry) All() []RouteRegistration {
	return r.routes
}

// ByOwner filters the collected routes to those owned by the given id.
func (r *RouteRegistry) ByOwner(id string) []RouteRegistration {
	var result []RouteRegistration
	for _, r := range r.routes {
		if r.Owner == id {
			result = append(result, r)
		}
	}
	return result
}

// Find looks up a registration by method and pattern.
func (r *RouteRegistry) Find(method, pattern string) (RouteRegistration, bool) {
	for _, reg := range r.routes {
		if reg.Method == method && reg.Pattern == pattern {
			return reg, true
		}
	}
	return RouteRegistration{}, false
}
