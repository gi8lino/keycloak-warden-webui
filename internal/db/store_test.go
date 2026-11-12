package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(t.Context(), filepath.Join(t.TempDir(), "warden-webui.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}

func TestInitialSetup(t *testing.T) {
	t.Parallel()

	t.Run("creates only one initial administrator", func(t *testing.T) {
		t.Parallel()
		store := testStore(t)
		required, err := store.SetupRequired(t.Context())
		require.NoError(t, err)
		assert.True(t, required)

		require.NoError(t, store.CreateInitialAdministrator(t.Context(), "admin", "bcrypt-hash"))
		require.ErrorIs(t, store.CreateInitialAdministrator(t.Context(), "second", "other-hash"), ErrSetupComplete)
		required, err = store.SetupRequired(t.Context())
		require.NoError(t, err)
		assert.False(t, required)

		hash, err := store.LookupCredential(t.Context(), "admin")
		require.NoError(t, err)
		assert.Equal(t, "bcrypt-hash", hash)
		_, err = store.LookupCredential(t.Context(), "second")
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("persists administrator after database reopen", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "warden-webui.db")
		store, err := Open(t.Context(), path)
		require.NoError(t, err)
		require.NoError(t, store.CreateInitialAdministrator(t.Context(), "operator", "hash"))
		require.NoError(t, store.Close())

		reopened, err := Open(t.Context(), path)
		require.NoError(t, err)
		defer func() { require.NoError(t, reopened.Close()) }()
		required, err := reopened.SetupRequired(t.Context())
		require.NoError(t, err)
		assert.False(t, required)
	})

	t.Run("refuses concurrent setup from two connections", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "warden-webui.db")
		first, err := Open(t.Context(), path)
		require.NoError(t, err)
		defer func() { require.NoError(t, first.Close()) }()
		second, err := Open(t.Context(), path)
		require.NoError(t, err)
		defer func() { require.NoError(t, second.Close()) }()

		started := make(chan struct{})
		results := make(chan error, 2)
		go func() { <-started; results <- first.CreateInitialAdministrator(t.Context(), "one", "first-hash") }()
		go func() { <-started; results <- second.CreateInitialAdministrator(t.Context(), "two", "second-hash") }()
		close(started)

		err1, err2 := <-results, <-results
		assert.True(t, (err1 == nil && err2 == ErrSetupComplete) || (err2 == nil && err1 == ErrSetupComplete), "setup must succeed exactly once: %v / %v", err1, err2)
	})
}

func TestStoredSessions(t *testing.T) {
	t.Parallel()

	t.Run("rejects expired and missing sessions", func(t *testing.T) {
		t.Parallel()
		store := testStore(t)
		require.NoError(t, store.CreateSession(t.Context(), "expired", Session{Name: "admin", Local: true}, time.Now().Add(-time.Second)))
		_, err := store.LookupSession(t.Context(), "expired")
		require.ErrorIs(t, err, ErrNotFound)
		_, err = store.LookupSession(t.Context(), "unknown")
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("persists and revokes externally verified session", func(t *testing.T) {
		t.Parallel()
		store := testStore(t)
		require.NoError(t, store.CreateFounder(t.Context(), "oidc-admin", "hash", &IdentityLink{Issuer: "issuer", Subject: "sub"}))
		require.NoError(t, store.CreateSession(t.Context(), "hash", Session{Name: "oidc-admin", Issuer: "issuer", Subject: "sub"}, time.Now().Add(time.Hour)))
		identity, err := store.LookupSession(t.Context(), "hash")
		require.NoError(t, err)
		assert.Equal(t, "oidc-admin", identity.Name)
		assert.False(t, identity.Local)

		require.NoError(t, store.RevokeSession(t.Context(), "hash"))
		_, err = store.LookupSession(t.Context(), "hash")
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("password update atomically revokes only local sessions", func(t *testing.T) {
		t.Parallel()
		store := testStore(t)
		require.NoError(t, store.CreateInitialAdministrator(t.Context(), "admin", "old-hash"))
		require.NoError(t, store.CreateSession(t.Context(), "local", Session{Name: "admin", Local: true}, time.Now().Add(time.Hour)))
		require.NoError(t, store.LinkIdentity(t.Context(), IdentityLink{Username: "admin", Issuer: "issuer", Subject: "sub"}))
		require.NoError(t, store.CreateSession(t.Context(), "oidc", Session{Name: "admin", Issuer: "issuer", Subject: "sub"}, time.Now().Add(time.Hour)))

		require.NoError(t, store.ChangePassword(t.Context(), "admin", "old-hash", "new-hash"))
		hash, err := store.LookupCredential(t.Context(), "admin")
		require.NoError(t, err)
		assert.Equal(t, "new-hash", hash)
		_, err = store.LookupSession(t.Context(), "local")
		require.ErrorIs(t, err, ErrNotFound)
		_, err = store.LookupSession(t.Context(), "oidc")
		require.NoError(t, err)
	})

	t.Run("stale password update does not revoke valid sessions", func(t *testing.T) {
		t.Parallel()
		store := testStore(t)
		require.NoError(t, store.CreateInitialAdministrator(t.Context(), "admin", "old-hash"))
		require.NoError(t, store.CreateSession(t.Context(), "live", Session{Name: "admin", Local: true}, time.Now().Add(time.Hour)))
		require.ErrorIs(t, store.ChangePassword(t.Context(), "admin", "wrong-hash", "new-hash"), ErrNotFound)
		hash, err := store.LookupCredential(t.Context(), "admin")
		require.NoError(t, err)
		assert.Equal(t, "old-hash", hash)
		_, err = store.LookupSession(t.Context(), "live")
		require.NoError(t, err)
	})

	t.Run("local session requires a stored administrator", func(t *testing.T) {
		t.Parallel()
		store := testStore(t)
		require.NoError(t, store.CreateSession(t.Context(), "orphan", Session{Name: "admin", Local: true}, time.Now().Add(time.Hour)))
		_, err := store.LookupSession(t.Context(), "orphan")
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func TestMigrateInitialAdministrator(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "legacy.db")
	old, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = old.Exec(`
		CREATE TABLE administrators (
		    id INTEGER PRIMARY KEY CHECK (id = 1),
		    username TEXT UNIQUE NOT NULL,
		    password_hash TEXT NOT NULL
		);

		INSERT INTO administrators
		VALUES (1, 'owner', 'original hash')
	`)
	require.NoError(t, err)
	require.NoError(t, old.Close())
	store, err := Open(t.Context(), path)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	hash, err := store.LookupCredential(t.Context(), "owner")
	require.NoError(t, err)
	assert.Equal(t, "original hash", hash)
	require.NoError(t, store.CreateLocalUser(t.Context(), "second", "another hash"))
	require.ErrorIs(t, store.SetLocalAccess(t.Context(), "second", "owner", false), ErrNotFound)
	require.ErrorIs(t, store.CreateInitialAdministrator(t.Context(), "replacement", "hash"), ErrSetupComplete)
}

// TestIdentityOwnership checks issuer isolation, uniqueness and revocation across relinking.
func TestIdentityOwnership(t *testing.T) {
	t.Parallel()
	store := testStore(t)
	link := IdentityLink{Issuer: "https://identity.example", Subject: "123", Username: "admin"}
	require.NoError(t, store.CreateFounder(t.Context(), "admin", "hash", &link))
	require.ErrorIs(t, store.LinkIdentity(t.Context(), link), ErrIdentityLinked)
	_, err := store.ResolveIdentity(t.Context(), "https://other.example", "123")
	require.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, store.CreateSession(t.Context(), "token", Session{Name: "admin", Issuer: link.Issuer, Subject: link.Subject}, time.Now().Add(time.Hour)))
	require.NoError(t, store.UnlinkIdentity(t.Context(), link))
	require.NoError(t, store.LinkIdentity(t.Context(), link))
	_, err = store.LookupSession(t.Context(), "token")
	require.ErrorIs(t, err, ErrNotFound)
}
