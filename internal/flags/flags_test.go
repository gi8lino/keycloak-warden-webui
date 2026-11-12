package flags

import (
	"testing"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/tinyflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("help documents the one flagset", func(t *testing.T) {
		t.Parallel()
		_, err := Parse([]string{"--help"}, "v0.1.0")
		require.Error(t, err)
		assert.True(t, tinyflags.IsHelpRequested(err))
		assert.Contains(t, err.Error(), "Usage: warden-webui [flags]")
		assert.Contains(t, err.Error(), "--management-listen-address")
	})

	t.Run("flags configure listeners and route prefix", func(t *testing.T) {
		t.Parallel()
		cfg, err := Parse([]string{
			"--warden-admin-token=secret",
			"--database=/data/test.db",
			"--setup-token=abcdefghijklmnopqrstuvwxyz012345",
			"--listen-address=:9000",
			"--management-listen-address=:9001",
			"--route-prefix=/ui",
			"--public-url=http://127.0.0.1:8092/ui",
		}, "dev")
		require.NoError(t, err)
		assert.Equal(t, ":9000", cfg.Server.ListenAddress)
		assert.Equal(t, ":9001", cfg.Server.ManagementListenAddress)
		assert.Equal(t, "/ui", cfg.Server.RoutePrefix)
		assert.Equal(t, "http://127.0.0.1:8092/ui", cfg.Server.PublicURL)
		assert.Equal(t, appkitauth.ModeLocal, cfg.Mode())
		assert.Equal(t, "warden-admins", cfg.AdminGroup)
		assert.Equal(t, "/data/test.db", cfg.Database)
	})

	t.Run("no subcommands", func(t *testing.T) {
		t.Parallel()
		_, err := Parse([]string{"serve", "--warden-admin-token=secret"}, "dev")
		require.ErrorContains(t, err, "accepts no positional arguments")
	})

	t.Run("positional arguments after flags are also rejected", func(t *testing.T) {
		t.Parallel()

		_, err := Parse([]string{"--warden-admin-token=secret", "unexpected"}, "dev")
		require.ErrorContains(t, err, "accepts no positional arguments")
	})
}

func TestParseEnvironment(t *testing.T) {
	// Environment changes cannot share parallel execution with other tests.
	t.Setenv("WARDEN_WEBUI__WARDEN_ADMIN_TOKEN", "env-secret")
	t.Setenv("WARDEN_WEBUI__SETUP_TOKEN", "abcdefghijklmnopqrstuvwxyz012345")

	cfg, err := Parse(nil, "dev")
	require.NoError(t, err)
	assert.Equal(t, "env-secret", cfg.WardenToken)
}
