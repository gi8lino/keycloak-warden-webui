package server

import (
	"net/http"

	"github.com/gi8lino/warden-webui/internal/http/endpoint"
	"github.com/gi8lino/warden-webui/internal/http/middleware"
)

// registerAdminRoutes protects every Warden administration page and form.
func registerAdminRoutes(mux *http.ServeMux, config Config) {
	protected := middleware.Authenticate(config.Identity, config.Logger, config.Pages.Problem)

	mux.Handle("GET /admin/users", protected(endpoint.Users(config.Identity, config.Pages)))
	mux.Handle("POST /admin/users", protected(endpoint.Users(config.Identity, config.Pages)))

	mux.Handle("POST /preferences/theme", protected(http.HandlerFunc(config.Pages.SaveTheme)))
	mux.Handle("GET /preferences", protected(endpoint.Preferences(config.Pages)))

	mux.Handle("GET /account/password", protected(endpoint.PasswordPage(config.Identity, config.Pages)))
	mux.Handle("POST /account/password", protected(endpoint.ChangePassword(config.Identity, config.Pages)))
	mux.Handle("POST /logout", protected(endpoint.Logout(config.Identity, config.Pages)))

	// Only fresh installations need a dashboard setup gate. Initialized
	// installations use the protected route without an extra state check.
	dashboard := protected(endpoint.Dashboard(config.Warden, config.Pages))
	if config.Identity.SetupRequired() {
		dashboard = middleware.RedirectToSetup(config.Identity)(dashboard)
	}
	mux.Handle("GET /{$}", dashboard)

	mux.Handle("GET /bans/{id}", protected(endpoint.BanDetails(config.Warden, config.Pages)))
	mux.Handle("POST /bans", protected(endpoint.CreateBan(config.Warden, config.Pages)))
	mux.Handle("POST /bans/{id}/release", protected(endpoint.ReleaseBan(config.Warden, config.Pages)))

	mux.Handle("GET /maintenance", protected(endpoint.Maintenance(config.Warden, config.Pages)))
	mux.Handle("POST /maintenance/{operation}", protected(endpoint.RunMaintenance(config.Warden, config.Pages)))
}
