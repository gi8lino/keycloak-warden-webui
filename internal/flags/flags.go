// Package flags parses Warden Web UI deployment flags and environment variables.
package flags

import (
	"net"
	"time"

	appkitauth "github.com/containeroo/appkit/auth"
	appkitconfig "github.com/containeroo/appkit/config"
	"github.com/containeroo/tinyflags"
)

const (
	sectionWarden     = "Warden API"
	sectionSetup      = "Initial setup"
	sectionAppearance = "Appearance"
)

// Config holds the deployment configuration for both listeners and browser authentication.
type Config struct {
	// Server contains the application and management listeners, route prefix and public URL.
	Server appkitconfig.Server
	// Authentication contains the selected mode and session lifetime.
	Authentication appkitconfig.Authentication
	// Proxy contains trusted peer and identity header settings.
	Proxy appkitconfig.Proxy
	// OIDC contains the identity provider and browser-flow settings.
	OIDC appkitconfig.OIDC
	// Logging contains diagnostic and access logging settings.
	Logging appkitconfig.Logging
	// WardenURL is the private administrative API base URL.
	WardenURL string
	// WardenToken is the server-only Warden administrator bearer token.
	WardenToken string
	// AllowInsecureWardenHTTP permits non-loopback HTTP for trusted upstream connections.
	AllowInsecureWardenHTTP bool
	// Database is the private SQLite database for Web UI accounts and sessions.
	Database string
	// ThemeDirectory overlays embedded palettes with deployment-specific TOML files.
	ThemeDirectory string
	// SetupToken authorizes creation of the initial local administrator.
	SetupToken string
	// SetupSubject optionally links the initial administrator to the configured external provider.
	SetupSubject string
	// AdminGroup is required for OIDC and trusted-proxy administrators.
	AdminGroup string
	// Overrides describes values explicitly supplied by flags or environment variables.
	Overrides tinyflags.Overrides
}

// Mode returns the effective browser authentication mode; unauthenticated mode is never enabled.
func (c Config) Mode() appkitauth.Mode {
	if c.Authentication.ModeOverride != nil {
		return *c.Authentication.ModeOverride
	}
	return appkitauth.ModeLocal
}

// Parse registers Appkit's shared flags alongside Warden Web UI's own settings.
func Parse(args []string, version string) (Config, error) {
	cfg := Config{}
	tf := appkitconfig.NewFlagSet("warden-webui", "WARDEN_WEBUI", version)
	tf.SectionOrder(
		appkitconfig.SectionServer,
		sectionWarden,
		appkitconfig.SectionDatabase,
		sectionSetup,
		appkitconfig.SectionAuthentication,
		appkitconfig.SectionProxy,
		appkitconfig.SectionOIDC,
		sectionAppearance,
		appkitconfig.SectionLogging,
	)

	// This CLI has flags only; silently accepting positional arguments hides typos.
	tf.NoPositional()
	tf.ExactPositional(0)

	finalizeServer := appkitconfig.RegisterServer(tf, &cfg.Server, appkitconfig.ServerOptions{
		ListenAddress:           &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8092},
		ManagementListenAddress: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8093},
		PublicURL:               "http://127.0.0.1:8092",
	})
	finalizeAuthentication := appkitconfig.RegisterAuthentication(tf, &cfg.Authentication, appkitconfig.AuthenticationOptions{
		RegisterSessionLifetime: true,
		DefaultSessionLifetime:  12 * time.Hour,
	})
	appkitconfig.RegisterProxy(tf, &cfg.Proxy)
	appkitconfig.RegisterOIDC(tf, &cfg.OIDC)
	finalizeLogging := appkitconfig.RegisterLogging(tf, &cfg.Logging)

	registerWardenFlags(tf, &cfg)
	registerLocalFlags(tf, &cfg)
	tf.StringVar(&cfg.ThemeDirectory, "theme-directory", "", "Directory containing custom theme TOML files").
		Section(sectionAppearance).
		Value()

	if err := tf.Parse(args); err != nil {
		return Config{}, err
	}
	finalizeServer()
	finalizeAuthentication()
	finalizeLogging()
	cfg.Overrides = tf.Overrides()

	return cfg, nil
}

// registerWardenFlags configures access to the Warden administration API.
func registerWardenFlags(tf *tinyflags.FlagSet, cfg *Config) {
	tf.StringVar(&cfg.WardenURL, "warden-url", "http://127.0.0.1:8080", "Warden application API URL").
		Section(sectionWarden).
		NotBlank().
		Value()
	tf.StringVar(&cfg.WardenToken, "warden-admin-token", "", "Bearer token for Warden administration").
		Section(sectionWarden).
		OverriddenValueMaskFn(tinyflags.MaskFirstLastLimit(5)).
		Value()
	// Allow HTTP to a remote Warden Service only when explicitly requested.
	tf.BoolVar(&cfg.AllowInsecureWardenHTTP, "allow-insecure-warden-http", false,
		"Allow unencrypted HTTP to a non-loopback Warden URL (admin token is sent in cleartext)").
		Section(sectionWarden).
		Strict().
		Value()
}

// registerLocalFlags configures the private credential database and first-run bootstrap secret.
func registerLocalFlags(tf *tinyflags.FlagSet, cfg *Config) {
	tf.StringVar(&cfg.Database, "database", "warden-webui.db", "Private Web UI SQLite database path").
		Section(appkitconfig.SectionDatabase).
		NotBlank().
		Value()
	tf.StringVar(&cfg.AdminGroup, "admin-group", "warden-admins", "Required group for OIDC and trusted-proxy administrators").
		Section(appkitconfig.SectionAuthentication).
		NotBlank().
		Value()
	tf.StringVar(&cfg.SetupSubject, "setup-subject", "", "External subject for the initial administrator (first setup only)").
		Section(sectionSetup).
		Value()
	tf.StringVar(&cfg.SetupToken, "setup-token", "", "One-time setup authorization token (at least 32 characters)").
		Section(sectionSetup).
		OverriddenValueMaskFn(tinyflags.MaskFirstLastLimit(5)).
		Value()
}
