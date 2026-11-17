package server

import (
	"context"
	"io/fs"
	"log/slog"

	"github.com/gi8lino/warden-webui/internal/http/auth"
	"github.com/gi8lino/warden-webui/internal/http/endpoint"
)

// InfrastructureConfig contains route, diagnostics, and presentation dependencies.
type InfrastructureConfig struct {
	// RoutePrefix is applied only to application routes.
	RoutePrefix string
	// PublicURL is the browser-visible address used for same-origin validation.
	PublicURL string
	// Static contains the embedded CSS files.
	Static fs.FS
	// Pages renders escaped HTML responses.
	Pages *endpoint.Pages
	// Logger records server errors and optional HTTP access logs.
	Logger *slog.Logger
	// AccessLog enables shared HTTPkit request logging.
	AccessLog bool
	// Ready probes the Web UI credential/session storage without contacting Warden.
	Ready func(context.Context) error
}

// AuthenticationConfig contains the administrator authentication boundary.
type AuthenticationConfig struct {
	// Identity handles Appkit local, OIDC, or trusted-proxy authentication.
	Identity *auth.Service
}

// ApplicationConfig contains the server-only Warden administration capability.
type ApplicationConfig struct {
	// Warden is the authenticated API used by administration pages.
	Warden endpoint.API
}

// Config composes the Web UI HTTP adapter's dependencies.
type Config struct {
	// InfrastructureConfig supplies routing, rendering, logging, and readiness.
	InfrastructureConfig
	// AuthenticationConfig supplies the administrator identity boundary.
	AuthenticationConfig
	// ApplicationConfig supplies upstream administrative operations.
	ApplicationConfig
}
