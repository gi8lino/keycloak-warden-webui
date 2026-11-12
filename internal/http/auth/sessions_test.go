package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/containeroo/appkit/auth/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/warden-webui/internal/db"
)

func TestDurableSessions(t *testing.T) {
	t.Parallel()

	t.Run("survives database reopen", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "sessions.db")
		store, err := db.Open(t.Context(), path)
		require.NoError(t, err)
		manager, _ := newSessions(store, time.Hour, "http://localhost", "")
		require.NoError(t, store.CreateFounder(t.Context(), "oidc-operator", "hash", &db.IdentityLink{Issuer: "issuer", Subject: "sub"}))
		token, err := manager.Start(t.Context(), User{Name: "oidc-operator", Issuer: "issuer", Subject: "sub"})
		require.NoError(t, err)
		require.NoError(t, store.Close())

		reopened, err := db.Open(t.Context(), path)
		require.NoError(t, err)
		defer reopened.Close()
		restartedManager, _ := newSessions(reopened, time.Hour, "http://localhost", "")
		user, err := restartedManager.Resolve(t.Context(), token)
		require.NoError(t, err)
		assert.Equal(t, "oidc-operator", user.Name)
	})

	t.Run("returns not found for expired token hash", func(t *testing.T) {
		t.Parallel()
		store := authStore(t)
		adapter := sessionStore{repository: store}
		require.NoError(t, adapter.Create(t.Context(), "expired", User{Name: "admin", Local: true}, time.Now().Add(-time.Second)))
		_, err := adapter.Lookup(t.Context(), "expired")
		require.ErrorIs(t, err, session.ErrNotFound)
	})

	t.Run("revokes hash", func(t *testing.T) {
		t.Parallel()
		store := authStore(t)
		adapter := sessionStore{repository: store}
		require.NoError(t, adapter.Create(t.Context(), "hash", User{Name: "admin", Local: true}, time.Now().Add(time.Hour)))
		require.NoError(t, adapter.Revoke(t.Context(), "hash"))
		_, err := adapter.Lookup(t.Context(), "hash")
		require.ErrorIs(t, err, session.ErrNotFound)
	})

	t.Run("logout clears browser cookie", func(t *testing.T) {
		t.Parallel()
		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		require.NoError(t, service.Logout(recorder, httptest.NewRequest(http.MethodPost, "/logout", nil)))
		cookies := recorder.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Less(t, cookies[0].MaxAge, 0)
	})
}

func TestCookiePath(t *testing.T) {
	t.Parallel()

	t.Run("uses root for unprefixed deployment", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "/", cookiePath(""))
	})

	t.Run("scopes cookies to application prefix", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "/ui", cookiePath("/ui"))
	})
}
