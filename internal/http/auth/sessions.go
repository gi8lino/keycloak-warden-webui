package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/containeroo/appkit/auth/session"

	"github.com/gi8lino/warden-webui/internal/db"
)

const (
	// cookieName identifies Warden Web UI's opaque browser session.
	cookieName = "warden_webui_session"
	// sessionPurpose separates Web UI browser hashes from other token uses.
	sessionPurpose = "warden-webui-browser"
)

// sessionStore adapts the application's durable SQLite store to Appkit.
type sessionStore struct {
	// repository persists sessions and their account bindings.
	repository *db.Store
}

// Create persists only the purpose-separated session hash.
func (s sessionStore) Create(ctx context.Context, hash string, user User, expires time.Time) error {
	return s.repository.CreateSession(ctx, hash, db.Session{
		Name:    user.Name,
		Local:   user.Local,
		Issuer:  user.Issuer,
		Subject: user.Subject,
	}, expires)
}

// Lookup returns only an unexpired, valid session.
func (s sessionStore) Lookup(ctx context.Context, hash string) (User, error) {
	user, err := s.repository.LookupSession(ctx, hash)
	if errors.Is(err, db.ErrNotFound) {
		return User{}, session.ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	return User{Name: user.Name, Local: user.Local, Issuer: user.Issuer, Subject: user.Subject}, nil
}

// Revoke invalidates the selected session hash.
func (s sessionStore) Revoke(ctx context.Context, hash string) error {
	return s.repository.RevokeSession(ctx, hash)
}

// newSessions configures durable session storage and path-scoped browser cookies.
func newSessions(repository *db.Store, lifetime time.Duration, publicURL, routePrefix string) (session.Manager[User], session.CookieConfig) {
	manager := session.Manager[User]{
		Store:    sessionStore{repository: repository},
		Lifetime: lifetime,
		Purpose:  sessionPurpose,
	}
	cookies := session.CookieConfig{
		Name:     cookieName,
		Path:     cookiePath(routePrefix),
		Lifetime: lifetime,
		Secure:   strings.HasPrefix(publicURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	}

	return manager, cookies
}

// cookiePath scopes Appkit sessions and OIDC state cookies to the application.
func cookiePath(prefix string) string {
	if prefix == "" {
		return "/"
	}

	return prefix
}

// Logout revokes the active browser session and clears its cookie.
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) error {
	if s.proxy != nil && !s.recovery {
		return errors.New("proxy identity must be logged out at the identity provider")
	}

	if cookie, err := r.Cookie(cookieName); err == nil {
		if err := s.sessions.End(r.Context(), cookie.Value); err != nil {
			return err
		}
	}

	http.SetCookie(w, s.cookies.Cookie("", true))

	return nil
}

// Verify that sessionStore implements Appkit session persistence.
var _ session.Store[User] = sessionStore{}
