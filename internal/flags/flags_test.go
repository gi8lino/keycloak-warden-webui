package flags

import (
	"math"
	"strconv"
	"testing"
	"time"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/tinyflags"
	"github.com/gi8lino/keycloak-warden/pkg/client"
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
		assert.Contains(t, err.Error(), "--warden-request-timeout")
		assert.Contains(t, err.Error(), "--warden-maintenance-timeout")
		assert.Contains(t, err.Error(), "--warden-max-response-bytes")
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
			"--public-url=http://127.0.0.1:8082/ui",
		}, "dev")
		require.NoError(t, err)
		assert.Equal(t, ":9000", cfg.Server.ListenAddress)
		assert.Equal(t, ":9001", cfg.Server.ManagementListenAddress)
		assert.Equal(t, "/ui", cfg.Server.RoutePrefix)
		assert.Equal(t, "http://127.0.0.1:8082/ui", cfg.Server.PublicURL)
		assert.Equal(t, appkitauth.ModeLocal, cfg.Mode())
		assert.Equal(t, "warden-admins", cfg.AdminGroup)
		assert.Equal(t, "/data/test.db", cfg.Database)
	})

	t.Run("disallows non-loopback HTTP by default", func(t *testing.T) {
		t.Parallel()

		cfg, err := Parse(nil, "dev")

		require.NoError(t, err)
		assert.False(t, cfg.AllowInsecureWardenHTTP)
	})

	t.Run("enables non-loopback HTTP explicitly", func(t *testing.T) {
		t.Parallel()

		cfg, err := Parse([]string{"--allow-insecure-warden-http=true"}, "dev")

		require.NoError(t, err)
		assert.True(t, cfg.AllowInsecureWardenHTTP)
	})

	t.Run("allows explicitly disabling non-loopback HTTP", func(t *testing.T) {
		t.Parallel()

		cfg, err := Parse([]string{"--allow-insecure-warden-http=false"}, "dev")

		require.NoError(t, err)
		assert.False(t, cfg.AllowInsecureWardenHTTP)
	})

	t.Run("requires an explicit value for non-loopback HTTP", func(t *testing.T) {
		t.Parallel()

		_, err := Parse([]string{"--allow-insecure-warden-http"}, "dev")

		require.ErrorContains(t, err, "missing value")
	})

	t.Run("uses the Warden HTTP client defaults", func(t *testing.T) {
		t.Parallel()

		cfg, err := Parse(nil, "dev")

		require.NoError(t, err)
		assert.Equal(t, client.DefaultRequestTimeout, cfg.WardenRequestTimeout)
		assert.Equal(t, client.DefaultMaintenanceTimeout, cfg.WardenMaintenanceTimeout)
		assert.Equal(t, client.DefaultMaxResponseBytes, cfg.WardenMaxResponseBytes)
	})

	t.Run("accepts custom Warden HTTP client limits", func(t *testing.T) {
		t.Parallel()

		cfg, err := Parse([]string{
			"--warden-request-timeout=20s",
			"--warden-maintenance-timeout=60s",
			"--warden-max-response-bytes=4194304",
		}, "dev")

		require.NoError(t, err)
		assert.Equal(t, 20*time.Second, cfg.WardenRequestTimeout)
		assert.Equal(t, time.Minute, cfg.WardenMaintenanceTimeout)
		assert.Equal(t, 4194304, cfg.WardenMaxResponseBytes)
	})

	t.Run("rejects a zero Warden request timeout", func(t *testing.T) {
		t.Parallel()

		_, err := Parse([]string{"--warden-request-timeout=0s"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects a negative Warden maintenance timeout", func(t *testing.T) {
		t.Parallel()

		_, err := Parse([]string{"--warden-maintenance-timeout=-1s"}, "dev")

		require.Error(t, err)
	})

	t.Run("rejects a zero Warden response limit", func(t *testing.T) {
		t.Parallel()

		_, err := Parse([]string{"--warden-max-response-bytes=0"}, "dev")

		require.ErrorContains(t, err, "maximum response bytes")
	})

	t.Run("rejects a negative Warden response limit", func(t *testing.T) {
		t.Parallel()

		_, err := Parse([]string{"--warden-max-response-bytes=-1"}, "dev")

		require.ErrorContains(t, err, "maximum response bytes")
	})

	t.Run("rejects a response limit that overflows the client's sentinel", func(t *testing.T) {
		t.Parallel()

		if strconv.IntSize != 64 {
			t.Skip("MaxInt cannot overflow int64 on a 32-bit platform")
		}

		_, err := Parse([]string{"--warden-max-response-bytes=" + strconv.Itoa(math.MaxInt)}, "dev")

		require.ErrorContains(t, err, "maximum response bytes")
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
	t.Setenv("WARDEN_WEBUI__ALLOW_INSECURE_WARDEN_HTTP", "true")
	t.Setenv("WARDEN_WEBUI__WARDEN_REQUEST_TIMEOUT", "20s")
	t.Setenv("WARDEN_WEBUI__WARDEN_MAINTENANCE_TIMEOUT", "1m")
	t.Setenv("WARDEN_WEBUI__WARDEN_MAX_RESPONSE_BYTES", "4194304")

	cfg, err := Parse(nil, "dev")
	require.NoError(t, err)
	assert.Equal(t, "env-secret", cfg.WardenToken)
	assert.True(t, cfg.AllowInsecureWardenHTTP)
	assert.Equal(t, 20*time.Second, cfg.WardenRequestTimeout)
	assert.Equal(t, time.Minute, cfg.WardenMaintenanceTimeout)
	assert.Equal(t, 4194304, cfg.WardenMaxResponseBytes)
}
