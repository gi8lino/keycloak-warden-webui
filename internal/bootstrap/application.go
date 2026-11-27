package bootstrap

import (
	"context"
	"fmt"
	"slices"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/proxy"

	"github.com/gi8lino/warden-webui/internal/cli"
	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// Application contains the configured browser authentication service.
type Application struct {
	// identity owns local, OIDC, or trusted-proxy authentication.
	identity *auth.Service
}

// NewApplication initializes browser authentication.
func NewApplication(ctx context.Context, cfg cli.Flags, infrastructure *Infrastructure) (*Application, error) {
	settings, err := authenticationConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve authentication settings: %w", err)
	}
	identity, err := auth.New(ctx, settings, infrastructure.store)
	if err != nil {
		return nil, fmt.Errorf("configure browser authentication: %w", err)
	}
	return &Application{identity: identity}, nil
}

// authenticationConfig translates parsed deployment settings into effective authentication policy.
func authenticationConfig(cfg cli.Flags) (auth.Config, error) {
	settings := auth.Config{
		Mode:            cfg.Mode(),
		PublicURL:       cfg.Server.PublicURL,
		RoutePrefix:     cfg.Server.RoutePrefix,
		SessionLifetime: cfg.Authentication.SessionLifetime,
		LocalLogin:      cfg.Authentication.LocalLogin,
		AdminGroup:      cfg.AdminGroup,
		SetupToken:      cfg.SetupToken,
		SetupSubject:    cfg.SetupSubject,
		OIDC:            cfg.OIDC,
	}
	if settings.Mode != appkitauth.ModeProxy {
		return settings, nil
	}

	cidrs := slices.Clone(cfg.Proxy.TrustedCIDRs)
	overrides := cfg.Proxy.DeploymentOverrides(cfg.Overrides)
	if overrides.TrustedCIDRs == nil {
		overrides.TrustedCIDRs = &cidrs
	}
	resolved, err := proxy.Resolve(proxy.Options{Overrides: overrides})
	if err != nil {
		return auth.Config{}, err
	}
	settings.Proxy = resolved
	return settings, nil
}
