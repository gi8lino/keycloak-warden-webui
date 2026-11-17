package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/warden-webui/internal/db"
)

func TestLocalAuthentication(t *testing.T) {
	t.Parallel()

	t.Run("setup signs in with a secure scoped cookie", func(t *testing.T) {
		t.Parallel()
		cfg := localConfig()
		cfg.Server.RoutePrefix = "/ui"
		cfg.Server.PublicURL = "https://warden.example/ui"
		service, err := New(t.Context(), cfg, authStore(t))
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/ui/setup", nil)
		require.NoError(t, service.SetupLocal(recorder, request, testSetupToken, "admin", "correct-password"))
		cookies := recorder.Result().Cookies()
		require.Len(t, cookies, 1)
		cookie := cookies[0]
		assert.Equal(t, cookieName, cookie.Name)
		assert.Equal(t, "/ui", cookie.Path)
		assert.True(t, cookie.HttpOnly)
		assert.True(t, cookie.Secure)
		assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)

		authenticated := httptest.NewRequest(http.MethodGet, "/ui/", nil)
		authenticated.AddCookie(cookie)
		user, err := service.Authenticate(authenticated)
		require.NoError(t, err)
		assert.Equal(t, "admin", user.Name)
		assert.True(t, user.Local)
	})

	t.Run("incorrect bootstrap token cannot create an administrator", func(t *testing.T) {
		t.Parallel()
		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		err = service.SetupLocal(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/setup", nil), "wrong", "admin", "correct-password")
		require.ErrorIs(t, err, appkitauth.ErrForbidden)
		assert.True(t, service.SetupRequired())
	})

	t.Run("setup cannot be repeated", func(t *testing.T) {
		t.Parallel()
		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodPost, "/setup", nil)
		require.NoError(t, service.SetupLocal(httptest.NewRecorder(), request, testSetupToken, "admin", "correct-password"))
		err = service.SetupLocal(httptest.NewRecorder(), request, testSetupToken, "second", "other-password")
		require.ErrorIs(t, err, db.ErrSetupComplete)
	})

	t.Run("wrong password is rejected without a cookie", func(t *testing.T) {
		t.Parallel()
		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodPost, "/setup", nil)
		require.NoError(t, service.SetupLocal(httptest.NewRecorder(), request, testSetupToken, "admin", "correct-password"))
		recorder := httptest.NewRecorder()
		err = service.SignInLocal(recorder, request, "admin", "incorrect")
		require.ErrorIs(t, err, appkitauth.ErrInvalidCredentials)
		assert.Empty(t, recorder.Result().Cookies())
	})

	t.Run("password change revokes all existing local sessions", func(t *testing.T) {
		t.Parallel()
		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodPost, "/setup", nil)
		recorder := httptest.NewRecorder()
		require.NoError(t, service.SetupLocal(recorder, request, testSetupToken, "admin", "correct-password"))
		oldCookie := recorder.Result().Cookies()[0]

		require.NoError(t, service.ChangeLocalPassword(t.Context(), User{Name: "admin", Local: true}, "correct-password", "new-password-2026"))
		oldRequest := httptest.NewRequest(http.MethodGet, "/", nil)
		oldRequest.AddCookie(oldCookie)
		_, err = service.Authenticate(oldRequest)
		require.ErrorIs(t, err, appkitauth.ErrUnauthenticated)
		require.ErrorIs(t, service.SignInLocal(httptest.NewRecorder(), request, "admin", "correct-password"), appkitauth.ErrInvalidCredentials)
		require.NoError(t, service.SignInLocal(httptest.NewRecorder(), request, "admin", "new-password-2026"))
	})

	t.Run("rejects short password", func(t *testing.T) {
		t.Parallel()
		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		err = service.SetupLocal(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/setup", nil), testSetupToken, "admin", "short")
		require.ErrorIs(t, err, ErrInvalidLocalInput)
	})

	t.Run("cannot begin OIDC in local mode", func(t *testing.T) {
		t.Parallel()
		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/login", nil)
		require.ErrorIs(t, service.BeginOIDC(recorder, request), appkitauth.ErrForbidden)
		require.ErrorIs(t, service.CompleteOIDC(recorder, request), appkitauth.ErrForbidden)
	})

	t.Run("rotated local cookie is revoked on logout", func(t *testing.T) {
		t.Parallel()
		service, err := New(t.Context(), localConfig(), authStore(t))
		require.NoError(t, err)
		setup := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/setup", nil)
		require.NoError(t, service.SetupLocal(setup, request, testSetupToken, "admin", "correct-password"))
		authenticated := httptest.NewRequest(http.MethodGet, "/", nil)
		authenticated.AddCookie(setup.Result().Cookies()[0])
		require.NoError(t, service.Logout(httptest.NewRecorder(), authenticated))
		_, err = service.Authenticate(authenticated)
		require.ErrorIs(t, err, appkitauth.ErrUnauthenticated)
	})
}
