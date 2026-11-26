// Package flags parses Warden Web UI deployment flags and environment variables.
package cli

import (
	"net"
	"time"

	appkitauth "github.com/containeroo/appkit/auth"
	appkitconfig "github.com/containeroo/appkit/config"
	"github.com/containeroo/tinyflags"
	"github.com/gi8lino/keycloak-warden/pkg/client"
)

const (
	// sectionWarden groups upstream Warden API connection settings.
	sectionWarden = "Warden API"
	// sectionSetup groups first-administrator bootstrap settings.
	sectionSetup = "Initial setup"
	// sectionAppearance groups theme catalog configuration.
	sectionAppearance = "Appearance"
)

// Flags holds the deployment configuration for both listeners and browser authentication.
type Flags struct {
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
	// WardenRequestTimeout bounds ordinary administrative HTTP requests.
	WardenRequestTimeout time.Duration
	// WardenMaintenanceTimeout bounds maintenance requests independently.
	WardenMaintenanceTimeout time.Duration
	// WardenMaxResponseBytes limits successful administrative response bodies.
	WardenMaxResponseBytes int
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
func (c Flags) Mode() appkitauth.Mode {
	if c.Authentication.ModeOverride != nil {
		return *c.Authentication.ModeOverride
	}
	return appkitauth.ModeLocal
}

// Parse registers Appkit's shared flags alongside Warden Web UI's own settings.
func Parse(args []string, version string) (flags Flags, err error) {
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

	localhost := net.IPv4(127, 0, 0, 1)
	finalizeServer := appkitconfig.RegisterServer(tf, &flags.Server, appkitconfig.ServerOptions{
		ListenAddress:           &net.TCPAddr{IP: localhost, Port: 8082},
		ManagementListenAddress: &net.TCPAddr{IP: localhost, Port: 8083},
		PublicURL:               "http://127.0.0.1:8082",
	})

	finalizeAuthentication := RegisterAuthentication(tf, &flags)
	finalizeLogging := appkitconfig.RegisterLogging(tf, &flags.Logging)

	registerWardenFlags(tf, &flags)
	registerLocalFlags(tf, &flags)

	tf.StringVar(&flags.ThemeDirectory, "theme-directory", "", "Directory containing custom theme TOML files").
		Section(sectionAppearance).
		Value()

	if err := tf.Parse(args); err != nil {
		return Flags{}, err
	}
	finalizeServer()
	finalizeAuthentication()
	finalizeLogging()
	flags.Overrides = tf.Overrides()

	return flags, nil
}

// RegisterAuthentication configures
func RegisterAuthentication(tf *tinyflags.FlagSet, cfg *Flags) func() {
	finalizeAuthentication := appkitconfig.RegisterAuthentication(tf, &cfg.Authentication, appkitconfig.AuthenticationOptions{
		RegisterSessionLifetime: true,
		DefaultSessionLifetime:  12 * time.Hour,
	})

	appkitconfig.RegisterProxy(tf, &cfg.Proxy)
	appkitconfig.RegisterOIDC(tf, &cfg.OIDC)

	return finalizeAuthentication
}

// registerWardenFlags configures access to the Warden administration API.
func registerWardenFlags(tf *tinyflags.FlagSet, cfg *Flags) {
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
	tf.DurationVar(&cfg.WardenRequestTimeout, "warden-request-timeout", client.DefaultRequestTimeout,
		"Timeout for ordinary Warden administrative requests").
		Section(sectionWarden).
		Validate(tinyflags.Positive[time.Duration]()).
		Value()
	tf.DurationVar(&cfg.WardenMaintenanceTimeout, "warden-maintenance-timeout", client.DefaultMaintenanceTimeout,
		"Timeout for Warden maintenance requests").
		Section(sectionWarden).
		Validate(tinyflags.Positive[time.Duration]()).
		Value()
	tf.IntVar(&cfg.WardenMaxResponseBytes, "warden-max-response-bytes", client.DefaultMaxResponseBytes,
		"Maximum successful Warden administrative response size in bytes").
		Section(sectionWarden).
		Validate(client.ValidateMaxResponseBytes).
		Value()
}

// registerLocalFlags configures the private credential database and first-run bootstrap secret.
func registerLocalFlags(tf *tinyflags.FlagSet, cfg *Flags) {
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
