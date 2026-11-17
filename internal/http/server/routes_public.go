package server

import (
	"net/http"

	"github.com/gi8lino/warden-webui/internal/http/endpoint"
)

// registerPublicRoutes exposes CSS, optional one-time setup, local login, and OIDC callback.
func registerPublicRoutes(mux *http.ServeMux, config Config) {
	mux.HandleFunc("GET /theme.css", config.Pages.ThemeStylesheet)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(config.Static))))

	// Do not register bootstrap endpoints for installations already initialized.
	// Once registered, their in-memory guards disable them after the first commit.
	if config.Identity.SetupRequired() {
		mux.Handle("GET /setup", endpoint.SetupPage(config.Identity, config.Pages))
		mux.Handle("POST /setup", endpoint.SetupLocal(config.Identity, config.Pages))
	}

	mux.Handle("GET /auth/local", endpoint.LocalLoginPage(config.Identity, config.Pages))
	mux.Handle("GET /login", endpoint.LoginPage(config.Identity, config.Pages))
	mux.Handle("POST /login", endpoint.LoginLocal(config.Identity, config.Pages))
	mux.Handle("GET /auth/callback", endpoint.OIDCCallback(config.Identity, config.Pages))
}
