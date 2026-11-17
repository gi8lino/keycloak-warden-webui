package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// Authenticate verifies the administrator before passing the Appkit principal to an endpoint.
func Authenticate(identity *auth.Service, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := identity.Authenticate(r)
			if err == nil {
				next.ServeHTTP(w, auth.WithUser(r, user))
				return
			}

			switch {
			case errors.Is(err, appkitauth.ErrForbidden):
				http.Error(w, "Administrator access required.", http.StatusForbidden)
			case errors.Is(err, appkitauth.ErrUnauthenticated), errors.Is(err, appkitauth.ErrInvalidCredentials):
				if identity.SetupRequired() {
					http.Redirect(w, r, "/setup", http.StatusSeeOther)
					return
				}
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			default:
				logger.Error("authenticate administrator failed", "event", "authentication_failed", "error", err)
				http.Error(w, "Authentication unavailable.", http.StatusInternalServerError)
			}
		})
	}
}
