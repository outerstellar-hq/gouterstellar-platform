package platform

import (
	"context"
	"fmt"
)

// BannerSeverity controls notice priority and presentation in the shared shell.
type BannerSeverity string

const (
	BannerCritical    BannerSeverity = "critical"
	BannerWarning     BannerSeverity = "warning"
	BannerInfo        BannerSeverity = "info"
	BannerMaintenance BannerSeverity = "maintenance"
)

// Banner is an authenticated shell notice contributed by an extension.
type Banner struct {
	ID          string
	Title       string
	Body        string
	Severity    BannerSeverity
	Dismissible bool
	DismissURL  string
}

// BannerProvider returns the notices visible to one authenticated user.
type BannerProvider interface {
	Banners(context.Context, RequestUser) ([]Banner, error)
}

// BannerProviderFunc adapts a function to BannerProvider.
type BannerProviderFunc func(context.Context, RequestUser) ([]Banner, error)

func (f BannerProviderFunc) Banners(ctx context.Context, user RequestUser) ([]Banner, error) {
	return f(ctx, user)
}

type bannerRegistration struct {
	owner    string
	provider BannerProvider
}

// BannerRegistry collects extension-owned providers during contribution.
type BannerRegistry struct {
	owner         string
	registrations []bannerRegistration
}

// Register adds a provider to the shared authenticated shell.
func (r *BannerRegistry) Register(provider BannerProvider) error {
	if r == nil || provider == nil {
		return fmt.Errorf("banner provider must not be nil")
	}
	r.registrations = append(r.registrations, bannerRegistration{owner: r.owner, provider: provider})
	return nil
}

func (r *BannerRegistry) all() []bannerRegistration {
	if r == nil {
		return nil
	}
	return r.registrations
}
