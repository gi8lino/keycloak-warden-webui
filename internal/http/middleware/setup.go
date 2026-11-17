package middleware

import (
	"net/http"

	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// RedirectToSetup routes fresh installations to the bootstrap page before
// proxy or OIDC authentication, using only the startup-loaded in-memory state.
func RedirectToSetup(identity *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if identity.SetupRequired() {
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
