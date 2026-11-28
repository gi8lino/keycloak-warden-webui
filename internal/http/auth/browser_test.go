package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/warden-webui/internal/db"
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
func localConfig() Config {
	return Config{
		PublicURL:       "http://127.0.0.1:8082",
		Mode:            appkitauth.ModeLocal,
		SessionLifetime: time.Hour,
		SetupToken:      testSetupToken,
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("rejects unauthenticated mode", func(t *testing.T) {
		t.Parallel()

		mode := appkitauth.ModeNone
		cfg := localConfig()
		cfg.Mode = mode
		_, err := New(t.Context(), cfg, authStore(t))

		require.ErrorContains(t, err, "unauthenticated mode is not supported")
	})

	t.Run("rejects unknown mode", func(t *testing.T) {
		t.Parallel()

		mode := appkitauth.Mode("unsupported")
		cfg := localConfig()
		cfg.Mode = mode
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

	t.Run("stale service synchronizes after another service completes setup", func(t *testing.T) {
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

	t.Run("simultaneous submissions create exactly one administrator", func(t *testing.T) {
		t.Parallel()

		store := authStore(t)
		service, err := New(t.Context(), localConfig(), store)
		require.NoError(t, err)

		start := make(chan struct{})
		results := make(chan error, 2)
		for _, name := range []string{"first", "second"} {
			go func() {
				<-start
				request := httptest.NewRequest(http.MethodPost, "/setup", nil).WithContext(t.Context())
				results <- service.SetupLocal(httptest.NewRecorder(), request, testSetupToken, name, "correct-password")
			}()
		}

		close(start)

		successes, conflicts := 0, 0
		for range 2 {
			err := <-results
			switch {
			case err == nil:
				successes++
			case errors.Is(err, db.ErrSetupComplete):
				conflicts++
			default:
				assert.NoError(t, err)
			}
		}

		assert.Equal(t, 1, successes)
		assert.Equal(t, 1, conflicts)
		assert.False(t, service.SetupRequired())

		users, err := store.LocalUsers(t.Context())
		require.NoError(t, err)
		require.Len(t, users, 1)
	})

	t.Run("session failure after commit leaves setup disabled and login available", func(t *testing.T) {
		t.Parallel()

		store := authStore(t)
		service, err := New(t.Context(), localConfig(), store)
		require.NoError(t, err)

		failure := errors.New("session persistence unavailable")
		service.sessions.Store = failingSessionStore{Store: service.sessions.Store, err: failure}

		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/setup", nil).WithContext(t.Context())
		err = service.SetupLocal(response, request, testSetupToken, "admin", "correct-password")

		require.ErrorIs(t, err, failure)
		assert.Empty(t, response.Result().Cookies())
		assert.False(t, service.SetupRequired())

		required, err := store.SetupRequired(t.Context())
		require.NoError(t, err)
		assert.False(t, required)
		require.ErrorIs(t, service.SetupLocal(httptest.NewRecorder(), request, testSetupToken, "other", "correct-password"), db.ErrSetupComplete)

		// The regular login manager still uses the healthy session store.
		login := httptest.NewRecorder()
		require.NoError(t, service.SignInLocal(login, request, "admin", "correct-password"))
		require.Len(t, login.Result().Cookies(), 1)

		authenticated := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(t.Context())
		authenticated.AddCookie(login.Result().Cookies()[0])
		user, err := service.Authenticate(authenticated)
		require.NoError(t, err)
		assert.Equal(t, "admin", user.Name)
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

// failingSessionStore injects a session-write failure while preserving reads and revocation.
type failingSessionStore struct {
	// Store delegates session lookup and revocation to the real persistence adapter.
	session.Store[User]
	// err is the failure returned when setup tries to persist its first session.
	err error
}

// Create simulates unavailable session persistence after the administrator has committed.
func (s failingSessionStore) Create(context.Context, string, User, time.Time) error {
	return s.err
}
