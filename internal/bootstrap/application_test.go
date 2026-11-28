package bootstrap

import (
	"testing"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/warden-webui/internal/cli"
)

func TestAuthenticationConfig(t *testing.T) {
	t.Parallel()

	t.Run("local mode does not require proxy configuration", func(t *testing.T) {
		t.Parallel()

		cfg, err := authenticationConfig(cli.Flags{SetupToken: "bootstrap", SetupSubject: "owner"})
		require.NoError(t, err)
		assert.Equal(t, appkitauth.ModeLocal, cfg.Mode)
		assert.Equal(t, "bootstrap", cfg.SetupToken)
		assert.Equal(t, "owner", cfg.SetupSubject)
	})

	t.Run("proxy mode preserves explicit header overrides", func(t *testing.T) {
		t.Parallel()

		flags, err := cli.Parse([]string{
			"--auth-mode=proxy",
			"--trusted-proxy-cidrs=127.0.0.1/32",
			"--trusted-username-headers=X-Verified-User",
			"--trusted-group-headers=X-Verified-Groups",
		}, "test")
		require.NoError(t, err)

		cfg, err := authenticationConfig(flags)
		require.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1/32"}, cfg.Proxy.TrustedCIDRs)
		assert.Equal(t, []string{"X-Verified-User"}, cfg.Proxy.UsernameHeaders)
		assert.Equal(t, []string{"X-Verified-Groups"}, cfg.Proxy.GroupHeaders)
		assert.Equal(t, proxy.Defaults().EmailHeaders, cfg.Proxy.EmailHeaders)
	})

	t.Run("proxy mode rejects missing trust boundary", func(t *testing.T) {
		t.Parallel()

		mode := appkitauth.ModeProxy
		flags := cli.Flags{}
		flags.Authentication.ModeOverride = &mode

		_, err := authenticationConfig(flags)

		require.Error(t, err)
	})
}
