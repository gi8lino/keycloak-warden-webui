package auth

import (
	"context"
	"net/http"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/oidc"
	appkitconfig "github.com/containeroo/appkit/config"
)

// oidcStateCookieName isolates the signed OIDC authorization state cookie.
const oidcStateCookieName = "warden_webui_oidc_state"

// newOIDC configures the verified authorization-code flow with Appkit's state and PKCE handling.
func newOIDC(ctx context.Context, cfg appkitconfig.OIDC, publicURL, routePrefix string) (*oidc.Flow, error) {
	return oidc.New(ctx, oidc.Config{
		Issuer:       cfg.Issuer,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		StateSecret:  cfg.SessionSecret,
		CallbackURL:  publicURL + "/auth/callback",
		CookieName:   oidcStateCookieName,
		CookiePath:   cookiePath(routePrefix),
		GroupClaim:   cfg.GroupClaim,
	})
}

// oidcFlow discovers the provider on first use and retries discovery after an outage.
func (s *Service) oidcFlow(ctx context.Context) (*oidc.Flow, error) {
	if s.discoverOIDC == nil {
		return nil, appkitauth.ErrForbidden
	}

	s.oidcMu.Lock()
	defer s.oidcMu.Unlock()

	if s.oidc != nil {
		return s.oidc, nil
	}

	flow, err := s.discoverOIDC(ctx)
	if err == nil {
		s.oidc = flow
	}

	return flow, err
}

// BeginOIDC redirects to the configured provider with signed state and PKCE.
func (s *Service) BeginOIDC(w http.ResponseWriter, r *http.Request) error {
	flow, err := s.oidcFlow(r.Context())
	if err != nil {
		return err
	}

	return flow.Begin(w, r, s.routePrefix+"/")
}

// CompleteOIDC checks verified administrator group membership and creates a browser session.
func (s *Service) CompleteOIDC(w http.ResponseWriter, r *http.Request) error {
	flow, err := s.oidcFlow(r.Context())
	if err != nil {
		return err
	}

	identity, _, err := flow.Complete(w, r)
	if err != nil {
		return err
	}

	if !hasGroup(identity.Groups, s.adminGroup) {
		return appkitauth.ErrForbidden
	}

	user, err := s.resolveExternal(r.Context(), identity.Issuer, identity.Subject)
	if err != nil {
		return err
	}

	token, err := s.sessions.Start(r.Context(), user)
	if err != nil {
		return err
	}

	http.SetCookie(w, s.cookies.Cookie(token, false))

	return nil
}
