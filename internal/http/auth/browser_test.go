package auth

import (
	"net/http"
	"net/http/httptest"
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

// testSetupToken authorizes bootstrap requests in authentication tests.
const testSetupToken = "long-test-setup-token-0123456789abcdef"

// authStore creates an isolated administrator store for authentication tests.
func authStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "warden-webui.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}

// localConfig supplies the deployment settings for password authentication tests.
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
		assert.True(t, service.SetupRequired())
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

// TestSetupState verifies that request-time state is cached while initial writes stay atomic.
func TestSetupState(t *testing.T) {
	t.Parallel()

	t.Run("fresh installation starts with setup enabled", func(t *testing.T) {
		t.Parallel()

		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		assert.True(t, service.SetupRequired())
	})

	t.Run("setup state remains available without a database query", func(t *testing.T) {
		t.Parallel()

		store, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "cached.db"))
		require.NoError(t, err)

		service, err := New(t.Context(), localConfig(), store)
		require.NoError(t, err)
		require.NoError(t, store.Close())

		assert.True(t, service.SetupRequired())
	})

	t.Run("successful database commit immediately disables setup", func(t *testing.T) {
		t.Parallel()

		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)

		request := httptest.NewRequest(http.MethodPost, "/setup", nil)
		require.NoError(t, service.SetupLocal(httptest.NewRecorder(), request, testSetupToken, "admin", "correct-password"))
		assert.False(t, service.SetupRequired())
		require.ErrorIs(t, service.SetupLocal(httptest.NewRecorder(), request, testSetupToken, "other", "other-password"), db.ErrSetupComplete)
	})

	t.Run("concurrent service with stale state synchronizes after losing database insert", func(t *testing.T) {
		t.Parallel()

		store := authStore(t)
		first, err := New(t.Context(), localConfig(), store)
		require.NoError(t, err)
		second, err := New(t.Context(), localConfig(), store)
		require.NoError(t, err)
		require.True(t, second.SetupRequired())

		request := httptest.NewRequest(http.MethodPost, "/setup", nil)
		require.NoError(t, first.SetupLocal(httptest.NewRecorder(), request, testSetupToken, "admin", "correct-password"))
		assert.False(t, first.SetupRequired())
		assert.True(t, second.SetupRequired(), "other instances cache their own startup state")

		require.ErrorIs(t, second.SetupLocal(httptest.NewRecorder(), request, testSetupToken, "other", "other-password"), db.ErrSetupComplete)
		assert.False(t, second.SetupRequired())
	})

	t.Run("initialized installation starts disabled without a setup token", func(t *testing.T) {
		t.Parallel()

		store := authStore(t)
		require.NoError(t, store.CreateInitialAdministrator(t.Context(), "admin", "hash"))
		cfg := localConfig()
		cfg.SetupToken = ""

		service, err := New(t.Context(), cfg, store)
		require.NoError(t, err)
		assert.False(t, service.SetupRequired())
	})
}
