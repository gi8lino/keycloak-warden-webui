package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/oidc"

	"github.com/gi8lino/warden-webui/internal/db"
)

// New configures Appkit adapters around the application's durable credential and session store.
func New(ctx context.Context, cfg Config, repository *db.Store) (*Service, error) {
	if repository == nil {
		return nil, errors.New("authentication repository is required")
	}

	manager, cookies := newSessions(repository, cfg.SessionLifetime, cfg.PublicURL, cfg.RoutePrefix)
	service := &Service{
		mode:         cfg.Mode,
		recovery:     cfg.LocalLogin,
		local:        newLocal(repository, manager),
		routePrefix:  cfg.RoutePrefix,
		adminGroup:   cfg.AdminGroup,
		repository:   repository,
		setupSubject: cfg.SetupSubject,
		setupDigest:  sha256.Sum256([]byte(cfg.SetupToken)),
		sessions:     manager,
		cookies:      cookies,
	}

	switch service.mode {
	case appkitauth.ModeLocal:
		// Local credentials are also used by optional recovery login.

	case appkitauth.ModeOIDC:
		service.discoverOIDC = func(ctx context.Context) (*oidc.Flow, error) {
			return newOIDC(ctx, cfg.OIDC, cfg.PublicURL, cfg.RoutePrefix)
		}
		if !service.recovery {
			if _, err := service.oidcFlow(ctx); err != nil {
				return nil, err
			}
		}

		service.issuer = cfg.OIDC.Issuer

	case appkitauth.ModeProxy:
		authenticator, err := newTrustedProxy(cfg.Proxy, cfg.AdminGroup, service.resolveExternal)
		if err != nil {
			return nil, err
		}

		service.proxy = authenticator
		service.issuer = proxyIssuer

	case appkitauth.ModeNone:
		return nil, errors.New("unauthenticated mode is not supported")

	default:
		return nil, fmt.Errorf("unsupported authentication mode: %s", service.mode)
	}

	required, err := repository.SetupRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("check initial administrator: %w", err)
	}

	service.setupRequired.Store(required)

	if required && len(cfg.SetupToken) < 32 {
		return nil, errors.New("setup-token must be at least 32 characters until the initial administrator is created")
	}

	if required && service.needsSetupIdentity() {
		return nil, errors.New("initial external setup requires --setup-subject or --local-login")
	}

	return service, nil
}

// needsSetupIdentity identifies external-only deployments without a founder mapping.
func (s *Service) needsSetupIdentity() bool {
	return s.mode != appkitauth.ModeLocal && s.setupSubject == "" && !s.recovery
}
