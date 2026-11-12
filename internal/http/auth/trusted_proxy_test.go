package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appkitauth "github.com/containeroo/appkit/auth"
	appkitconfig "github.com/containeroo/appkit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/warden-webui/internal/db"
	"github.com/gi8lino/warden-webui/internal/flags"
)

// proxyService configures the same trusted ingress settings used by the HTTP adapter.
func proxyService(t *testing.T) *Service {
	t.Helper()

	mode := appkitauth.ModeProxy
	store := authStore(t)
	require.NoError(t, store.CreateFounder(t.Context(), "local-admin", "hash", &db.IdentityLink{Issuer: proxyIssuer, Subject: "operator"}))
	service, err := New(t.Context(), flags.Config{
		Authentication: appkitconfig.Authentication{
			ModeOverride: &mode,
		},
		Proxy: appkitconfig.Proxy{
			TrustedCIDRs: []string{"127.0.0.1/32"},
		},
		AdminGroup: "warden-admins",
	}, store)
	require.NoError(t, err)

	return service
}

func TestTrustedProxy(t *testing.T) {
	t.Parallel()

	t.Run("accepts an administrator asserted by a trusted ingress", func(t *testing.T) {
		t.Parallel()

		service := proxyService(t)
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "127.0.0.1:12345"
		request.Header.Set("X-Forwarded-User", "operator")
		request.Header.Set("X-Forwarded-Groups", "operators,warden-admins")

		user, err := service.Authenticate(request)
		require.NoError(t, err)
		assert.Equal(t, "local-admin", user.Name)
		request.Header.Set("X-Forwarded-User", "local-admin")
		_, err = service.Authenticate(request)
		require.ErrorIs(t, err, appkitauth.ErrForbidden)
	})

	t.Run("rejects an untrusted immediate peer", func(t *testing.T) {
		t.Parallel()

		service := proxyService(t)
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "198.51.100.10:12345"
		request.Header.Set("X-Forwarded-User", "operator")
		request.Header.Set("X-Forwarded-Groups", "warden-admins")

		_, err := service.Authenticate(request)
		require.ErrorIs(t, err, appkitauth.ErrUnauthenticated)
	})

	t.Run("requires exact administrator group membership", func(t *testing.T) {
		t.Parallel()

		service := proxyService(t)
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "127.0.0.1:12345"
		request.Header.Set("X-Forwarded-User", "operator")
		request.Header.Set("X-Forwarded-Groups", "warden-admins-extra")

		_, err := service.Authenticate(request)
		require.ErrorIs(t, err, appkitauth.ErrForbidden)
	})

	t.Run("refuses local sign-in and logout in proxy mode", func(t *testing.T) {
		t.Parallel()

		service := proxyService(t)
		request := httptest.NewRequest(http.MethodPost, "/login", nil)
		recorder := httptest.NewRecorder()

		require.ErrorIs(t, service.SignInLocal(recorder, request, "admin", "password"), appkitauth.ErrForbidden)
		require.ErrorContains(t, service.Logout(recorder, request), "identity provider")
	})

	t.Run("rejects proxy configurations without trusted peers", func(t *testing.T) {
		t.Parallel()

		mode := appkitauth.ModeProxy
		_, err := New(t.Context(), flags.Config{
			Authentication: appkitconfig.Authentication{
				ModeOverride: &mode,
			},
		}, authStore(t))

		require.Error(t, err)
	})
}

func TestExternalSetup(t *testing.T) {
	t.Parallel()
	store := authStore(t)
	mode := appkitauth.ModeProxy
	service, err := New(t.Context(), flags.Config{
		Authentication: appkitconfig.Authentication{ModeOverride: &mode},
		Proxy:          appkitconfig.Proxy{TrustedCIDRs: []string{"127.0.0.1/32"}},
		AdminGroup:     "admins", SetupToken: testSetupToken, SetupSubject: "external-owner",
	}, store)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/setup", nil)
	response := httptest.NewRecorder()
	require.NoError(t, service.SetupLocal(response, request, testSetupToken, "owner", "correct-password"))
	assert.Empty(t, response.Result().Cookies())
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-User", "external-owner")
	request.Header.Set("X-Forwarded-Groups", "admins")
	actor, err := service.Authenticate(request)
	require.NoError(t, err)
	assert.Equal(t, "owner", actor.Name)
	require.NoError(t, service.CreateLocalUser(t.Context(), actor, "second", "correct-password"))
	require.ErrorIs(t, service.UpdateIdentity(t.Context(), actor, "second", "external-second", true), db.ErrNotFound)
	request.Header.Set("X-Forwarded-User", "external-second")
	_, err = service.Authenticate(request)
	require.ErrorIs(t, err, appkitauth.ErrForbidden)
	pending, err := service.PendingIdentities(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"external-second"}, pending)
	require.NoError(t, service.UpdateIdentity(t.Context(), actor, "second", "external-second", true))
	pending, err = service.PendingIdentities(t.Context())
	require.NoError(t, err)
	assert.Empty(t, pending)
	second, err := service.Authenticate(request)
	require.NoError(t, err)
	assert.Equal(t, "second", second.Name)
	require.NoError(t, service.SetLocalAccess(t.Context(), actor, "second", false))
	_, err = service.Authenticate(request)
	require.ErrorIs(t, err, appkitauth.ErrForbidden)
}

func TestRecoveryDuringOIDCOutage(t *testing.T) {
	t.Parallel()
	cfg := localConfig()
	store := authStore(t)
	localService, err := New(t.Context(), cfg, store)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/auth/local", nil)
	require.NoError(t, localService.SetupLocal(httptest.NewRecorder(), request, testSetupToken, "owner", "correct-password"))
	mode := appkitauth.ModeOIDC
	cfg.Authentication.ModeOverride = &mode
	cfg.Authentication.LocalLogin = true
	cfg.OIDC.Issuer = "http://127.0.0.1:1"
	service, err := New(t.Context(), cfg, store)
	require.NoError(t, err)
	response := httptest.NewRecorder()
	require.NoError(t, service.SignInLocal(response, request, "owner", "correct-password"))
	request.AddCookie(response.Result().Cookies()[0])
	user, err := service.Authenticate(request)
	require.NoError(t, err)
	assert.True(t, user.Local)
	_, err = service.resolveExternal(t.Context(), cfg.OIDC.Issuer, "provider-subject")
	require.ErrorIs(t, err, appkitauth.ErrForbidden)
	require.NoError(t, service.UpdateIdentity(t.Context(), user, "owner", "provider-subject", true))
	links, err := service.IdentityLinks(t.Context())
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, cfg.OIDC.Issuer, links[0].Issuer)
	service.recovery = false
	_, err = service.Authenticate(request)
	require.ErrorIs(t, err, appkitauth.ErrUnauthenticated)
}
