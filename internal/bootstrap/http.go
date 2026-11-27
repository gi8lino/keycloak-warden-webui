package bootstrap

import (
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/gi8lino/warden-webui/internal/cli"
	"github.com/gi8lino/warden-webui/internal/http/endpoint"
	"github.com/gi8lino/warden-webui/internal/http/server"
)

// NewHTTPConfig loads presentation assets and configures the HTTP handlers.
func NewHTTPConfig(
	assets fs.FS,
	cfg cli.Flags,
	infrastructure *Infrastructure,
	application *Application,
	logger *slog.Logger,
	version string,
) (server.Config, error) {
	pages, err := endpoint.NewPages(
		assets,
		cfg.Server.RoutePrefix,
		application.identity.Mode(),
		logger.With("component", "pages"),
		infrastructure.store,
		cfg.ThemeDirectory,
		version,
	)
	if err != nil {
		return server.Config{}, fmt.Errorf("parse UI templates: %w", err)
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		return server.Config{}, fmt.Errorf("configure static files: %w", err)
	}

	return server.Config{
		RoutePrefix: cfg.Server.RoutePrefix,
		PublicURL:   cfg.Server.PublicURL,
		Static:      static,
		Pages:       pages,
		Logger:      logger.With("component", "http"),
		AccessLog:   cfg.Logging.AccessLog,
		Ready:       infrastructure.store.Ping,
		Identity:    application.identity,
		Warden:      infrastructure.warden,
	}, nil
}
