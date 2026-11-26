// Package app composes runtime dependencies and starts Warden Web UI.
package app

import (
	"context"
	"fmt"
	"io"
	"io/fs"

	appkithttpserver "github.com/containeroo/appkit/httpserver"
	appkitlogging "github.com/containeroo/appkit/logging"
	"github.com/containeroo/httpkit/grace"
	"github.com/containeroo/tinyflags"
	"github.com/gi8lino/warden-webui/internal/bootstrap"
	"github.com/gi8lino/warden-webui/internal/cli"
	httpserver "github.com/gi8lino/warden-webui/internal/http/server"
)

// Run parses flags, initializes Appkit browser authentication, and serves both listeners.
func Run(ctx context.Context, args []string, assets fs.FS, version string, stdout, stderr io.Writer) error {
	flags, err := cli.Parse(args, version)
	if err != nil {
		if tinyflags.IsHelpRequested(err) || tinyflags.IsVersionRequested(err) {
			_, _ = fmt.Fprint(stdout, err.Error())
			return nil
		}
		_, _ = fmt.Fprintln(stderr, err)
		return err
	}

	// Configure process logging and record the effective application identity.
	logger := appkitlogging.Setup(flags.Logging.Format, flags.Logging.Debug, stdout)
	setupLogger := logger.With("component", "setup")
	setupLogger.Info(
		"starting Warden Web UI",
		"event", "app_starting",
		"version", version,
	)
	if len(flags.Overrides) > 0 {
		setupLogger.Info(
			"CLI Overrides",
			"event", "cli_overrides",
			"overrides", flags.Overrides.Values(),
		)
	}

	ctx, stop := grace.SignalContext(ctx)
	defer stop()

	// Construct infrastructure and retain ownership until both listeners stop.
	infrastructure, err := bootstrap.NewInfrastructure(ctx, flags)
	if err != nil {
		setupLogger.Error(
			"initialize infrastructure",
			"event", "infrastructure_init_failed",
			"error", err,
		)
		return err
	}
	defer infrastructure.Close() // nolint:errcheck

	application, err := bootstrap.NewApplication(ctx, flags, infrastructure)
	if err != nil {
		setupLogger.Error(
			"initialize application",
			"event", "application_init_failed",
			"error", err,
		)
		return err
	}

	serverConfig, err := bootstrap.NewHTTPConfig(assets, flags, infrastructure, application, logger, version)
	if err != nil {
		setupLogger.Error(
			"configure HTTP adapter",
			"event", "http_setup_failed",
			"error", err,
		)
		return err
	}

	handlers := httpserver.New(serverConfig)

	if err := appkithttpserver.Run(
		ctx,
		flags.Server.ListenAddress,
		flags.Server.ManagementListenAddress,
		handlers,
		logger.With("server", "application"),
		logger.With("server", "management"),
	); err != nil {
		setupLogger.Error(
			"run server error",
			"event", "server_run_failed",
			"error", err,
		)
		return err
	}

	return nil
}
