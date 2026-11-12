package auth

import (
	"path/filepath"
	"testing"
	"time"

	appkitauth "github.com/containeroo/appkit/auth"
	appkitconfig "github.com/containeroo/appkit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/warden-webui/internal/db"
	"github.com/gi8lino/warden-webui/internal/flags"
)

const testSetupToken = "long-test-setup-token-0123456789abcdef"

func authStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "warden-webui.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}

func localConfig() flags.Config {
	return flags.Config{
		Server:         appkitconfig.Server{PublicURL: "http://127.0.0.1:8092"},
		Authentication: appkitconfig.Authentication{SessionLifetime: time.Hour},
		SetupToken:     testSetupToken,
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("rejects unauthenticated mode", func(t *testing.T) {
		t.Parallel()
		mode := appkitauth.ModeNone
		cfg := localConfig()
		cfg.Authentication.ModeOverride = &mode
		_, err := New(t.Context(), cfg, authStore(t))
		require.ErrorContains(t, err, "unauthenticated mode is not supported")
	})

	t.Run("rejects unknown mode", func(t *testing.T) {
		t.Parallel()
		mode := appkitauth.Mode("unsupported")
		cfg := localConfig()
		cfg.Authentication.ModeOverride = &mode
		_, err := New(t.Context(), cfg, authStore(t))
		require.ErrorContains(t, err, "unsupported authentication mode")
	})

	t.Run("defaults to local mode", func(t *testing.T) {
		t.Parallel()
		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		assert.Equal(t, appkitauth.ModeLocal, service.Mode())
		required, err := service.SetupRequired(t.Context())
		require.NoError(t, err)
		assert.True(t, required)
	})

	t.Run("requires a bootstrap token before initial local setup", func(t *testing.T) {
		t.Parallel()
		cfg := localConfig()
		cfg.SetupToken = ""
		_, err := New(t.Context(), cfg, authStore(t))
		require.ErrorContains(t, err, "setup-token")
	})

	t.Run("permits an existing local administrator without a setup token", func(t *testing.T) {
		t.Parallel()
		store := authStore(t)
		require.NoError(t, store.CreateInitialAdministrator(t.Context(), "admin", "hash"))
		cfg := localConfig()
		cfg.SetupToken = ""
		_, err := New(t.Context(), cfg, store)
		require.NoError(t, err)
	})

	t.Run("requires repository", func(t *testing.T) {
		t.Parallel()
		_, err := New(t.Context(), localConfig(), nil)
		require.ErrorContains(t, err, "repository")
	})
}
