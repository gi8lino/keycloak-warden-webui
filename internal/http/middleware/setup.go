package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// RedirectToSetup routes a fresh installation to initial administrator setup
// before an external authentication provider can reject an unmapped identity.
// Apply it only to the dashboard entry point; other routes retain their own access policy.
func RedirectToSetup(identity *auth.Service, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			required, err := identity.SetupRequired(r.Context())
			if err != nil {
				logger.Error("check setup state failed", "error", err)
				http.Error(w, "Authentication unavailable.", http.StatusInternalServerError)
				return
			}
			if required {
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
