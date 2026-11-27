package auth

import (
	"time"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/proxy"
	appkitconfig "github.com/containeroo/appkit/config"
)

// Config contains resolved browser authentication policy, independent of CLI parsing.
type Config struct {
	// Mode selects the primary sign-in method.
	Mode appkitauth.Mode
	// PublicURL is the browser-visible application URL.
	PublicURL string
	// RoutePrefix scopes session cookies and redirects.
	RoutePrefix string
	// SessionLifetime bounds local browser sessions.
	SessionLifetime time.Duration
	// LocalLogin enables password recovery alongside external authentication.
	LocalLogin bool
	// AdminGroup is the exact group required from an external identity.
	AdminGroup string
	// SetupToken authorizes creation of the first administrator.
	SetupToken string
	// SetupSubject links the initial administrator to an external subject.
	SetupSubject string
	// OIDC configures the external authorization-code provider.
	OIDC appkitconfig.OIDC
	// Proxy contains effective peer trust and identity headers.
	Proxy proxy.Config
}
