package auth

import (
	"context"
	"net/http"
	"sync"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/browser"
	"github.com/containeroo/appkit/auth/local"
	"github.com/containeroo/appkit/auth/oidc"
	"github.com/containeroo/appkit/auth/proxy"
	"github.com/containeroo/appkit/auth/session"

	"github.com/gi8lino/warden-webui/internal/db"
)

// Service owns the selected authentication mode and its application-specific policy.
type Service struct {
	// mode is the configured browser authentication method.
	mode appkitauth.Mode
	// routePrefix scopes OIDC redirects to the application.
	routePrefix string
	// adminGroup is the exact group required for external identities.
	adminGroup string
	// repository owns persisted administrator credentials and session hashes.
	repository *db.Store
	// setupDigest authenticates first-run setup without retaining the plain-text token.
	setupDigest [32]byte
	// setupSubject is the deployment-configured initial external identity.
	setupSubject string
	// sessions issues and verifies Appkit opaque browser sessions.
	sessions session.Manager[User]
	// cookies configures the HTTP-only, path-scoped browser session cookie.
	cookies session.CookieConfig
	// local verifies the local administrator's stored credentials.
	local local.Service[User]
	// recovery permits local sessions alongside the configured external provider.
	recovery bool
	// issuer scopes identity links to the configured OIDC provider or proxy.
	issuer string
	// proxy authenticates identities asserted by a trusted ingress.
	proxy *proxy.Authenticator[User]
	// oidcMu serializes discovery and caches the first successful flow.
	oidcMu sync.Mutex
	// discoverOIDC initializes the configured external provider on demand.
	discoverOIDC func(context.Context) (*oidc.Flow, error)
	// oidc performs the verified OIDC authorization-code flow.
	oidc *oidc.Flow
}

// Mode reports the selected authentication method.
func (s *Service) Mode() appkitauth.Mode {
	return s.mode
}

// Authenticate validates the trusted proxy or an Appkit server-side browser session.
func (s *Service) Authenticate(r *http.Request) (User, error) {
	provider := browser.Authenticator[User]{Policy: s.policy(), Session: s.sessionOrigin}
	if s.proxy != nil {
		provider.Primary = s.proxy
	}
	return provider.Authenticate(r)
}

// policy returns the effective deployment authentication settings.
func (s *Service) policy() browser.Policy {
	return browser.Policy{Mode: s.mode, Issuer: s.issuer, Recovery: s.recovery}
}

// sessionOrigin reads the account and sign-in method persisted with its session.
func (s *Service) sessionOrigin(r *http.Request) (User, browser.Origin, error) {
	user, err := (session.Authenticator[User]{Manager: s.sessions, CookieName: cookieName}).Authenticate(r)
	origin := browser.Origin{
		Mode:   appkitauth.ModeOIDC,
		Issuer: user.Issuer,
	}
	if user.Local {
		origin = browser.Origin{Mode: appkitauth.ModeLocal}
	}
	return user, origin, err
}

// LocalLoginEnabled reports whether password authentication is available.
func (s *Service) LocalLoginEnabled() bool { return s.policy().LocalEnabled() }

// SetupRequired reports whether the local directory needs its first administrator.
func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	return s.repository.SetupRequired(ctx)
}

var _ appkitauth.Authenticator[User] = (*Service)(nil)
