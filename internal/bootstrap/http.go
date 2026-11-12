package bootstrap

import (
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/gi8lino/warden-webui/internal/flags"
	"github.com/gi8lino/warden-webui/internal/http/endpoint"
	"github.com/gi8lino/warden-webui/internal/http/server"
)

// NewHTTPConfig prepares presentation assets and wires the passive HTTP adapter.
func NewHTTPConfig(
	assets fs.FS,
	cfg flags.Config,
	infrastructure *Infrastructure,
	application *Application,
	logger *slog.Logger,
) (server.Config, error) {
	pages, err := endpoint.NewPages(
		assets,
		cfg.Server.RoutePrefix,
		application.identity.Mode(),
		logger.With("component", "pages"),
		infrastructure.store,
		cfg.ThemeDirectory,
	)
	if err != nil {
		return server.Config{}, fmt.Errorf("parse UI templates: %w", err)
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		return server.Config{}, fmt.Errorf("configure static files: %w", err)
	}

	return server.Config{
		InfrastructureConfig: server.InfrastructureConfig{
			RoutePrefix: cfg.Server.RoutePrefix,
			PublicURL:   cfg.Server.PublicURL,
			Static:      static,
			Pages:       pages,
			Logger:      logger.With("component", "http"),
			AccessLog:   cfg.Logging.AccessLog,
			Ready:       infrastructure.store.Ping,
		},
		AuthenticationConfig: server.AuthenticationConfig{Identity: application.identity},
		ApplicationConfig:    server.ApplicationConfig{Warden: infrastructure.warden},
	}, nil
}
