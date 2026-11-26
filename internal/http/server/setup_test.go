package server

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appkitauth "github.com/containeroo/appkit/auth"
	appkitconfig "github.com/containeroo/appkit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/warden-webui/internal/cli"
	"github.com/gi8lino/warden-webui/internal/db"
	"github.com/gi8lino/warden-webui/internal/http/auth"
	"github.com/gi8lino/warden-webui/internal/http/endpoint"
	"github.com/gi8lino/warden-webui/web"
)

// bootstrapSecret authorizes first-run setup in HTTP integration tests.
const bootstrapSecret = "development-setup-token-0123456789abcdef"

// localHandlers builds the real HTML authentication routes backed by an isolated SQLite database.
func localHandlers(t *testing.T, prefix string) http.Handler {
	t.Helper()

	store, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "webui.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return newLocalApplication(t, prefix, store)
}

// newLocalApplication composes the HTTP adapter against an existing database.
func newLocalApplication(t *testing.T, prefix string, store *db.Store) http.Handler {
	t.Helper()

	return New(newLocalServerConfig(t, prefix, store)).Application
}

// newLocalServerConfig builds a complete route configuration from the current persisted state.
func newLocalServerConfig(t *testing.T, prefix string, store *db.Store) Config {
	t.Helper()

	cfg := cli.Flags{
		Server: appkitconfig.Server{
			PublicURL:   "http://127.0.0.1:8082" + prefix,
			RoutePrefix: prefix,
		},
		Authentication: appkitconfig.Authentication{SessionLifetime: time.Hour},
		SetupToken:     bootstrapSecret,
	}
	identity, err := auth.New(t.Context(), cfg, store)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pages, err := endpoint.NewPages(web.Assets, prefix, identity.Mode(), logger, store, "", "test")
	require.NoError(t, err)

	static, err := fs.Sub(web.Assets, "static")
	require.NoError(t, err)

	return Config{
		InfrastructureConfig: InfrastructureConfig{
			RoutePrefix: prefix,
			PublicURL:   cfg.Server.PublicURL,
			Static:      static,
			Pages:       pages,
			Logger:      logger,
			Ready:       store.Ping,
		},
		AuthenticationConfig: AuthenticationConfig{Identity: identity},
		ApplicationConfig:    ApplicationConfig{Warden: &fakeAPI{}},
	}
}

// localForm sends the exact origin required for browser write protection.
func localForm(path string, values url.Values) *http.Request {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://127.0.0.1:8082")
	return request
}

func TestBrowserSetup(t *testing.T) {
	t.Parallel()

	t.Run("new local installation redirects to setup", func(t *testing.T) {
		t.Parallel()
		app := localHandlers(t, "")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/setup", recorder.Header().Get("Location"))

		recorder = httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Create administrator")
	})

	t.Run("fresh proxy installation redirects before authorization and disables setup after commit", func(t *testing.T) {
		t.Parallel()

		store, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "webui-proxy.db"))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, store.Close()) })

		mode := appkitauth.ModeProxy
		cfg := cli.Flags{
			Server: appkitconfig.Server{PublicURL: "http://127.0.0.1:8082"},
			Authentication: appkitconfig.Authentication{
				ModeOverride:    &mode,
				SessionLifetime: time.Hour,
			},
			Proxy:        appkitconfig.Proxy{TrustedCIDRs: []string{"127.0.0.1/32"}},
			AdminGroup:   "warden-admins",
			SetupToken:   bootstrapSecret,
			SetupSubject: "operator",
		}
		identity, err := auth.New(t.Context(), cfg, store)
		require.NoError(t, err)

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		pages, err := endpoint.NewPages(web.Assets, "", identity.Mode(), logger, store, "", "test")
		require.NoError(t, err)
		static, err := fs.Sub(web.Assets, "static")
		require.NoError(t, err)

		app := New(Config{
			InfrastructureConfig: InfrastructureConfig{
				PublicURL: cfg.Server.PublicURL,
				Static:    static,
				Pages:     pages,
				Logger:    logger,
			},
			AuthenticationConfig: AuthenticationConfig{Identity: identity},
			ApplicationConfig:    ApplicationConfig{Warden: &fakeAPI{}},
		}).Application

		before := httptest.NewRecorder()
		app.ServeHTTP(before, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusSeeOther, before.Code)
		assert.Equal(t, "/setup", before.Header().Get("Location"))

		setup := httptest.NewRecorder()
		app.ServeHTTP(setup, localForm("/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"owner"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		require.Equal(t, http.StatusSeeOther, setup.Code)
		assert.False(t, identity.SetupRequired())

		after := httptest.NewRecorder()
		app.ServeHTTP(after, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusNotFound, after.Code)

		unauthorized := httptest.NewRecorder()
		app.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusSeeOther, unauthorized.Code)
		assert.Equal(t, "/login", unauthorized.Header().Get("Location"))
	})

	t.Run("invalid token cannot provision administrator", func(t *testing.T) {
		t.Parallel()
		app := localHandlers(t, "")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, localForm("/setup", url.Values{
			"setupToken":      {"incorrect"},
			"username":        {"admin"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		assert.Equal(t, http.StatusForbidden, recorder.Code)
		recorder = httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("setup logs in and cannot run twice", func(t *testing.T) {
		t.Parallel()
		app := localHandlers(t, "")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, localForm("/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"admin"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		require.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/", recorder.Header().Get("Location"))
		cookies := recorder.Result().Cookies()
		require.Len(t, cookies, 1)

		recorder = httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusNotFound, recorder.Code)

		recorder = httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/account/password", nil)
		request.AddCookie(cookies[0])
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Change password")
	})

	t.Run("completing setup disables both setup methods immediately", func(t *testing.T) {
		t.Parallel()

		app := localHandlers(t, "")
		setup := httptest.NewRecorder()
		app.ServeHTTP(setup, localForm("/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"admin"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		require.Equal(t, http.StatusSeeOther, setup.Code)

		get := httptest.NewRecorder()
		app.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusNotFound, get.Code)

		post := httptest.NewRecorder()
		app.ServeHTTP(post, localForm("/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"other"},
			"password":        {"other-password"},
			"confirmPassword": {"other-password"},
		}))
		assert.Equal(t, http.StatusNotFound, post.Code)

		root := httptest.NewRecorder()
		app.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusSeeOther, root.Code)
		assert.Equal(t, "/login", root.Header().Get("Location"))
	})

	t.Run("password rotation revokes session and requires new credentials", func(t *testing.T) {
		t.Parallel()
		app := localHandlers(t, "")
		setup := httptest.NewRecorder()
		app.ServeHTTP(setup, localForm("/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"admin"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		require.Equal(t, http.StatusSeeOther, setup.Code)
		oldCookies := setup.Result().Cookies()
		require.Len(t, oldCookies, 1)
		oldCookie := oldCookies[0]

		change := localForm("/account/password", url.Values{
			"currentPassword": {"correct-password"},
			"newPassword":     {"new-password-2026"},
			"confirmPassword": {"new-password-2026"},
		})
		change.AddCookie(oldCookie)
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, change)
		require.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/login?changed=1", recorder.Header().Get("Location"))

		recorder = httptest.NewRecorder()
		protected := httptest.NewRequest(http.MethodGet, "/", nil)
		protected.AddCookie(oldCookie)
		app.ServeHTTP(recorder, protected)
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/login", recorder.Header().Get("Location"))

		recorder = httptest.NewRecorder()
		app.ServeHTTP(recorder, localForm("/login", url.Values{
			"username": {"admin"},
			"password": {"new-password-2026"},
		}))
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
	})

	t.Run("same-origin protection applies to setup", func(t *testing.T) {
		t.Parallel()
		app := localHandlers(t, "")
		request := localForm("/setup", url.Values{"setupToken": {bootstrapSecret}})
		request.Header.Set("Origin", "https://attacker.example")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("setup redirects and cookies respect route prefix", func(t *testing.T) {
		t.Parallel()

		app := localHandlers(t, "/ui")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ui/", nil))

		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/ui/setup", recorder.Header().Get("Location"))
	})

	t.Run("prefixed setup completes with one prefix and scoped session cookie", func(t *testing.T) {
		t.Parallel()

		app := localHandlers(t, "/ui")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, localForm("/ui/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"admin"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))

		require.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/ui/", recorder.Header().Get("Location"))

		cookies := recorder.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "/ui", cookies[0].Path)

		dashboard := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/ui/", nil)
		request.AddCookie(cookies[0])
		app.ServeHTTP(dashboard, request)

		assert.Equal(t, http.StatusOK, dashboard.Code)
	})

	t.Run("prefixed local login redirects only once", func(t *testing.T) {
		t.Parallel()

		app := localHandlers(t, "/ui")
		setup := httptest.NewRecorder()
		app.ServeHTTP(setup, localForm("/ui/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"admin"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		require.Equal(t, http.StatusSeeOther, setup.Code)

		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, localForm("/ui/login", url.Values{
			"username": {"admin"},
			"password": {"correct-password"},
		}))

		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/ui/", recorder.Header().Get("Location"))
	})

	t.Run("prefixed password change redirects only once", func(t *testing.T) {
		t.Parallel()

		app := localHandlers(t, "/ui")
		setup := httptest.NewRecorder()
		app.ServeHTTP(setup, localForm("/ui/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"admin"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		require.Equal(t, http.StatusSeeOther, setup.Code)

		cookies := setup.Result().Cookies()
		require.Len(t, cookies, 1)

		request := localForm("/ui/account/password", url.Values{
			"currentPassword": {"correct-password"},
			"newPassword":     {"new-password-2026"},
			"confirmPassword": {"new-password-2026"},
		})
		request.AddCookie(cookies[0])

		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/ui/login?changed=1", recorder.Header().Get("Location"))
	})
}

// postAccountForm exercises a protected or public account form with browser origin checks.
func postAccountForm(t *testing.T, app http.Handler, path string, values url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	r := localForm(path, values)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	result := httptest.NewRecorder()
	app.ServeHTTP(result, r)
	return result
}

func TestAdministratorDirectory(t *testing.T) {
	t.Parallel()
	app := localHandlers(t, "/ui")
	path := "/ui/admin/users"
	setup := postAccountForm(t, app, "/ui/setup", url.Values{
		"username":        {"owner"},
		"password":        {"owner-password"},
		"confirmPassword": {"owner-password"},
		"setupToken":      {bootstrapSecret},
	}, nil)
	require.Equal(t, http.StatusSeeOther, setup.Code)
	owner := setup.Result().Cookies()[0]
	create := url.Values{
		"operation": {"create"},
		"username":  {"operator"},
		"password":  {"operator-password"},
	}
	result := postAccountForm(t, app, path, create, owner)
	require.Equal(t, http.StatusSeeOther, result.Code)
	assert.Equal(t, path, result.Header().Get("Location"))
	require.Equal(t, http.StatusConflict, postAccountForm(t, app, path, create, owner).Code)

	login := url.Values{
		"username": {"operator"},
		"password": {"operator-password"},
	}
	signedIn := postAccountForm(t, app, "/ui/login", login, nil)
	require.Equal(t, http.StatusSeeOther, signedIn.Code)
	operator := signedIn.Result().Cookies()[0]
	request := httptest.NewRequest(http.MethodGet, "/ui/preferences", nil)
	request.AddCookie(operator)
	page := httptest.NewRecorder()
	app.ServeHTTP(page, request)
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), "Change password")
	assert.Contains(t, page.Body.String(), "/ui/admin/users")

	disable := url.Values{
		"operation": {"disable"},
		"username":  {"operator"},
	}
	require.Equal(t, http.StatusConflict, postAccountForm(t, app, path, disable, operator).Code)
	require.Equal(t, http.StatusSeeOther, postAccountForm(t, app, path, disable, owner).Code)
	enable := url.Values{
		"operation": {"enable"},
		"username":  {"operator"},
	}
	require.Equal(t, http.StatusSeeOther, postAccountForm(t, app, path, enable, owner).Code)
	page = httptest.NewRecorder()
	app.ServeHTTP(page, request)
	assert.Equal(t, "/ui/login", page.Header().Get("Location"))
	require.Equal(t, http.StatusSeeOther, postAccountForm(t, app, "/ui/login", login, nil).Code)
}

// freshExternalHandlers builds an uninitialized Web UI in proxy or OIDC mode.
func freshExternalHandlers(t *testing.T, mode appkitauth.Mode, prefix string, localLogin bool) http.Handler {
	t.Helper()

	store, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "webui.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	cfg := cli.Flags{
		Server: appkitconfig.Server{
			PublicURL:   "http://127.0.0.1:8082" + prefix,
			RoutePrefix: prefix,
		},
		Authentication: appkitconfig.Authentication{
			ModeOverride:    &mode,
			LocalLogin:      localLogin,
			SessionLifetime: time.Hour,
		},
		AdminGroup:   "warden-admins",
		SetupSubject: "operator",
		SetupToken:   bootstrapSecret,
	}
	if mode == appkitauth.ModeProxy {
		cfg.Proxy.TrustedCIDRs = []string{"127.0.0.1/32"}
	}
	if mode == appkitauth.ModeOIDC {
		// Allow local recovery to avoid OIDC network discovery in this route test.
		cfg.OIDC.Issuer = "https://identity.example/realms/test"
	}

	identity, err := auth.New(t.Context(), cfg, store)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pages, err := endpoint.NewPages(web.Assets, prefix, identity.Mode(), logger, store, "", "test")
	require.NoError(t, err)
	static, err := fs.Sub(web.Assets, "static")
	require.NoError(t, err)

	handlers := New(Config{
		InfrastructureConfig: InfrastructureConfig{
			RoutePrefix: prefix,
			PublicURL:   cfg.Server.PublicURL,
			Static:      static,
			Pages:       pages,
			Logger:      logger,
		},
		AuthenticationConfig: AuthenticationConfig{Identity: identity},
		ApplicationConfig:    ApplicationConfig{Warden: &fakeAPI{}},
	})
	return handlers.Application
}

func TestExternalFirstRunSetup(t *testing.T) {
	t.Parallel()

	t.Run("proxy redirects to setup before rejecting an unmapped administrator", func(t *testing.T) {
		t.Parallel()

		app := freshExternalHandlers(t, appkitauth.ModeProxy, "", false)
		request := authenticatedRequest(http.MethodGet, "/", "")
		request.Header.Set("X-Forwarded-Groups", "other-group")
		response := httptest.NewRecorder()

		app.ServeHTTP(response, request)

		assert.Equal(t, http.StatusSeeOther, response.Code)
		assert.Equal(t, "/setup", response.Header().Get("Location"))
		assert.Empty(t, response.Result().Cookies())

		setup := httptest.NewRecorder()
		app.ServeHTTP(setup, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusOK, setup.Code)
	})

	t.Run("other protected routes still require proxy authorization", func(t *testing.T) {
		t.Parallel()

		app := freshExternalHandlers(t, appkitauth.ModeProxy, "", false)
		request := authenticatedRequest(http.MethodGet, "/bans/one", "")
		request.Header.Set("X-Forwarded-Groups", "other-group")
		response := httptest.NewRecorder()

		app.ServeHTTP(response, request)

		assert.Equal(t, http.StatusForbidden, response.Code)
	})

	t.Run("proxy redirect respects the application prefix", func(t *testing.T) {
		t.Parallel()

		app := freshExternalHandlers(t, appkitauth.ModeProxy, "/ui", false)
		response := httptest.NewRecorder()

		app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/ui/", ""))

		assert.Equal(t, http.StatusSeeOther, response.Code)
		assert.Equal(t, "/ui/setup", response.Header().Get("Location"))
	})

	t.Run("proxy requires an approved identity after setup", func(t *testing.T) {
		t.Parallel()

		app := freshExternalHandlers(t, appkitauth.ModeProxy, "", false)
		setup := httptest.NewRecorder()
		app.ServeHTTP(setup, localForm("/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"owner"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		require.Equal(t, http.StatusSeeOther, setup.Code)
		assert.Empty(t, setup.Result().Cookies())

		approved := httptest.NewRecorder()
		app.ServeHTTP(approved, authenticatedRequest(http.MethodGet, "/", ""))
		assert.Equal(t, http.StatusOK, approved.Code)

		unapproved := authenticatedRequest(http.MethodGet, "/", "")
		unapproved.Header.Set("X-Forwarded-Groups", "other-group")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, unapproved)
		assert.Equal(t, http.StatusForbidden, response.Code)

		completed := httptest.NewRecorder()
		app.ServeHTTP(completed, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusNotFound, completed.Code)
	})

	t.Run("OIDC redirects to setup before starting the sign-in flow", func(t *testing.T) {
		t.Parallel()

		app := freshExternalHandlers(t, appkitauth.ModeOIDC, "", true)
		response := httptest.NewRecorder()

		app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, http.StatusSeeOther, response.Code)
		assert.Equal(t, "/setup", response.Header().Get("Location"))
	})

	t.Run("OIDC keeps authentication after initial setup", func(t *testing.T) {
		t.Parallel()

		app := freshExternalHandlers(t, appkitauth.ModeOIDC, "", true)
		setup := httptest.NewRecorder()
		app.ServeHTTP(setup, localForm("/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"owner"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		require.Equal(t, http.StatusSeeOther, setup.Code)
		assert.Empty(t, setup.Result().Cookies())

		response := httptest.NewRecorder()
		app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, http.StatusSeeOther, response.Code)
		assert.Equal(t, "/login", response.Header().Get("Location"))
	})
}

func TestRecoveryPasswordChange(t *testing.T) {
	t.Parallel()
	t.Run("proxy keeps recovery login", func(t *testing.T) {
		t.Parallel()
		assertRecoveryPasswordChange(t, appkitauth.ModeProxy)
	})
	t.Run("OIDC keeps recovery login", func(t *testing.T) {
		t.Parallel()
		assertRecoveryPasswordChange(t, appkitauth.ModeOIDC)
	})
}

// assertRecoveryPasswordChange verifies password rotation keeps external-mode users on local recovery.
func assertRecoveryPasswordChange(t *testing.T, mode appkitauth.Mode) {
	t.Helper()
	app := freshExternalHandlers(t, mode, "/ui", true)
	setup := postAccountForm(t, app, "/ui/setup", url.Values{
		"setupToken": {bootstrapSecret}, "username": {"owner"},
		"password": {"correct-password"}, "confirmPassword": {"correct-password"},
	}, nil)
	require.Equal(t, http.StatusSeeOther, setup.Code)

	login := postAccountForm(t, app, "/ui/login", url.Values{
		"username": {"owner"}, "password": {"correct-password"},
	}, nil)
	require.Equal(t, http.StatusSeeOther, login.Code)
	require.Len(t, login.Result().Cookies(), 1)
	cookie := login.Result().Cookies()[0]
	changed := postAccountForm(t, app, "/ui/account/password", url.Values{
		"currentPassword": {"correct-password"}, "newPassword": {"new-password-2026"},
		"confirmPassword": {"new-password-2026"},
	}, cookie)
	require.Equal(t, http.StatusSeeOther, changed.Code)
	require.Equal(t, "/ui/auth/local?changed=1", changed.Header().Get("Location"))

	page := httptest.NewRecorder()
	app.ServeHTTP(page, httptest.NewRequest(http.MethodGet, changed.Header().Get("Location"), nil))
	assert.Equal(t, http.StatusOK, page.Code)
	login = postAccountForm(t, app, "/ui/login", url.Values{
		"username": {"owner"}, "password": {"new-password-2026"},
	}, nil)
	assert.Equal(t, http.StatusSeeOther, login.Code)
}

// TestSetupRouteRegistration checks initial registration and restart behavior.
func TestSetupRouteRegistration(t *testing.T) {
	t.Parallel()

	t.Run("fresh installation registers both setup routes", func(t *testing.T) {
		t.Parallel()

		store, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "fresh.db"))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, store.Close()) })

		config := newLocalServerConfig(t, "", store)
		mux := http.NewServeMux()
		registerPublicRoutes(mux, config)

		_, getPattern := mux.Handler(httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, "GET /setup", getPattern)

		_, postPattern := mux.Handler(httptest.NewRequest(http.MethodPost, "/setup", nil))
		assert.Equal(t, "POST /setup", postPattern)
	})

	t.Run("existing administrator prevents setup route registration", func(t *testing.T) {
		t.Parallel()

		store, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "initialized.db"))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		require.NoError(t, store.CreateInitialAdministrator(t.Context(), "admin", "hash"))

		config := newLocalServerConfig(t, "", store)
		mux := http.NewServeMux()
		registerPublicRoutes(mux, config)

		_, getPattern := mux.Handler(httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Empty(t, getPattern)

		_, postPattern := mux.Handler(httptest.NewRequest(http.MethodPost, "/setup", nil))
		assert.Empty(t, postPattern)

		app := New(config).Application
		get := httptest.NewRecorder()
		app.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusNotFound, get.Code)

		post := httptest.NewRecorder()
		app.ServeHTTP(post, localForm("/setup", url.Values{}))
		assert.Equal(t, http.StatusNotFound, post.Code)
	})

	t.Run("restart loads completed setup from persisted database", func(t *testing.T) {
		t.Parallel()

		database := filepath.Join(t.TempDir(), "restarted.db")
		first, err := db.Open(t.Context(), database)
		require.NoError(t, err)

		app := newLocalApplication(t, "", first)
		setup := httptest.NewRecorder()
		app.ServeHTTP(setup, localForm("/setup", url.Values{
			"setupToken":      {bootstrapSecret},
			"username":        {"admin"},
			"password":        {"correct-password"},
			"confirmPassword": {"correct-password"},
		}))
		require.Equal(t, http.StatusSeeOther, setup.Code)
		require.NoError(t, first.Close())

		reopened, err := db.Open(t.Context(), database)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, reopened.Close()) })

		config := newLocalServerConfig(t, "", reopened)
		require.False(t, config.Identity.SetupRequired())
		mux := http.NewServeMux()
		registerPublicRoutes(mux, config)

		_, getPattern := mux.Handler(httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Empty(t, getPattern)
		_, postPattern := mux.Handler(httptest.NewRequest(http.MethodPost, "/setup", nil))
		assert.Empty(t, postPattern)

		restarted := New(config).Application
		get := httptest.NewRecorder()
		restarted.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusNotFound, get.Code)

		root := httptest.NewRecorder()
		restarted.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusSeeOther, root.Code)
		assert.Equal(t, "/login", root.Header().Get("Location"))
	})
}

func TestCustomNotFound(t *testing.T) {
	t.Parallel()

	app := localHandlers(t, "/ui")
	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ui/missing", nil))

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Contains(t, response.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, response.Body.String(), "Page not found")
	assert.Contains(t, response.Body.String(), `href="/ui/"`)
	assert.Contains(t, response.Body.String(), "Version test")
}
