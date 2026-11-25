package server

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appkitauth "github.com/containeroo/appkit/auth"
	appkitconfig "github.com/containeroo/appkit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/keycloak-warden/pkg/workerstate"
	"github.com/gi8lino/warden-webui/internal/db"
	"github.com/gi8lino/warden-webui/internal/flags"
	"github.com/gi8lino/warden-webui/internal/http/auth"
	"github.com/gi8lino/warden-webui/internal/http/endpoint"
	"github.com/gi8lino/warden-webui/web"
)

// fakeAPI supplies Warden records and tracks successful mutations.
type fakeAPI struct {
	// banCalls counts successful manual ban requests.
	banCalls atomic.Int32
	// banDuration captures the requested ban lifetime in nanoseconds.
	banDuration atomic.Int64
	// maintenanceCalls counts requested maintenance operations.
	maintenanceCalls atomic.Int32
	// workersError simulates an upstream worker status failure.
	workersError error
}

// List returns a deterministic active decision.
func (*fakeAPI) List(context.Context) ([]client.Record, error) {
	return []client.Record{{Decision: client.Decision{ID: "one", IP: "203.0.113.1", State: "banned"}}}, nil
}

// ListDecisionHistory supplies the audit view for routing tests.
func (*fakeAPI) ListDecisionHistory(context.Context, client.RevisionFilter) (client.RevisionPage, error) {
	return client.RevisionPage{Items: []client.Revision{}, Page: 1}, nil
}

// ListDeliveries supplies current progress for routing tests.
func (*fakeAPI) ListDeliveries(context.Context, client.DeliveryFilter) (client.DeliveryOverviewPage, error) {
	return client.DeliveryOverviewPage{Items: []client.DeliveryOverview{}, Page: 1}, nil
}

// Get returns an existing decision.
func (*fakeAPI) Get(context.Context, string) (client.Record, error) {
	return client.Record{Decision: client.Decision{ID: "one", IP: "203.0.113.1", State: "banned"}}, nil
}

// GetPageThroughRevision supplies decision history for browser pagination tests.
func (a *fakeAPI) GetPageThroughRevision(ctx context.Context, id string, page int, revision int64) (client.Record, error) {
	return a.Get(ctx, id)
}

// Ban records a successful mutation.
func (a *fakeAPI) Ban(_ context.Context, request client.ManualBanRequest) (client.Record, error) {
	a.banDuration.Store(int64(request.Duration))
	a.banCalls.Add(1)
	return client.Record{}, nil
}

// Release returns a completed release.
func (*fakeAPI) Unban(context.Context, string) (client.Record, error) {
	return client.Record{}, nil
}

// Maintenance returns an empty scheduler status.
func (*fakeAPI) MaintenanceStatus(context.Context) (client.Maintenance, error) {
	return client.Maintenance{}, nil
}

// PruneObservations records an observation-cleanup request.
func (a *fakeAPI) PruneObservations(context.Context) (client.Maintenance, error) {
	a.maintenanceCalls.Add(1)
	return client.Maintenance{}, nil
}

// OptimizeDatabase records a database optimization request.
func (a *fakeAPI) OptimizeDatabase(context.Context) (client.Maintenance, error) {
	a.maintenanceCalls.Add(1)
	return client.Maintenance{}, nil
}

// WorkerStatus returns worker status through the same Warden API as other operations.
func (a *fakeAPI) WorkerStatus(context.Context) (client.Workers, error) {
	if a.workersError != nil {
		return client.Workers{}, a.workersError
	}
	return client.Workers{
		Actions: client.ActionWorker{
			WorkerRuntime:     client.WorkerRuntime{State: workerstate.StateIdle},
			PendingDeliveries: 4,
		},
		Maintenance: client.MaintenanceWorker{
			WorkerRuntime: client.WorkerRuntime{State: workerstate.StateIdle},
		},
	}, nil
}

// testHandlers constructs the whole HTTP adapter using a trusted proxy administrator.
func testHandlers(t *testing.T, prefix string) (http.Handler, http.Handler, *fakeAPI) {
	t.Helper()
	return testHandlersWithAPIError(t, prefix, nil)
}

// testHandlersWithAPIError composes an API that can simulate a worker status failure.
func testHandlersWithAPIError(t *testing.T, prefix string, workersError error) (http.Handler, http.Handler, *fakeAPI) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mode := appkitauth.ModeProxy
	cfg := flags.Config{
		Server: appkitconfig.Server{
			PublicURL:   "http://127.0.0.1:8082" + prefix,
			RoutePrefix: prefix,
		},
		Authentication: appkitconfig.Authentication{ModeOverride: &mode},
		Proxy:          appkitconfig.Proxy{TrustedCIDRs: []string{"127.0.0.1/32"}},
		AdminGroup:     "warden-admins",
	}
	repository, err := db.Open(t.Context(), t.TempDir()+"/webui.db")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repository.Close()) })
	require.NoError(t, repository.CreateFounder(t.Context(), "operator", "hash", &db.IdentityLink{Issuer: "warden-webui-proxy", Subject: "operator"}))
	identity, err := auth.New(t.Context(), cfg, repository)
	require.NoError(t, err)
	pages, err := endpoint.NewPages(web.Assets, prefix, identity.Mode(), logger, repository, "", "test")
	require.NoError(t, err)
	static, err := fs.Sub(web.Assets, "static")
	require.NoError(t, err)
	api := &fakeAPI{workersError: workersError}
	handlers := New(Config{
		InfrastructureConfig: InfrastructureConfig{
			RoutePrefix: prefix,
			PublicURL:   cfg.Server.PublicURL,
			Static:      static,
			Pages:       pages,
			Logger:      logger,
		},
		AuthenticationConfig: AuthenticationConfig{Identity: identity},
		ApplicationConfig: ApplicationConfig{
			Warden: api,
		},
	})
	return handlers.Application, handlers.Management, api
}

// authenticatedRequest simulates the asserted identity of a trusted ingress peer.
func authenticatedRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-User", "operator")
	request.Header.Set("X-Forwarded-Groups", "operators,warden-admins")
	return request
}

func TestPages(t *testing.T) {
	t.Parallel()

	t.Run("dashboard", func(t *testing.T) {
		t.Parallel()
		app, _, _ := testHandlers(t, "")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, "/", ""))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "203.0.113.1")
		assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	})

	t.Run("decision details", func(t *testing.T) {
		t.Parallel()
		app, _, _ := testHandlers(t, "")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, "/bans/one", ""))
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "203.0.113.1")
	})

	t.Run("maintenance", func(t *testing.T) {
		t.Parallel()
		app, _, _ := testHandlers(t, "")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, "/maintenance", ""))
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Database maintenance")
		assert.Contains(t, recorder.Body.String(), "Background workers")
		assert.Contains(t, recorder.Body.String(), "Pending deliveries")
	})

	t.Run("maintenance remains usable with an older Warden", func(t *testing.T) {
		t.Parallel()

		app, _, _ := testHandlersWithAPIError(t, "", &client.Error{Code: http.StatusNotFound})
		response := httptest.NewRecorder()
		app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/maintenance", ""))

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "Worker status requires a newer Warden release")
		assert.Contains(t, response.Body.String(), "Observation cleanup")
	})

	t.Run("static assets", func(t *testing.T) {
		t.Parallel()
		app, _, _ := testHandlers(t, "")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil))
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.NotEmpty(t, recorder.Body.String())
	})
}

func TestAuthentication(t *testing.T) {
	t.Parallel()

	t.Run("untrusted proxy redirects to login", func(t *testing.T) {
		t.Parallel()
		app, _, _ := testHandlers(t, "")
		request := authenticatedRequest(http.MethodGet, "/", "")
		request.RemoteAddr = "198.51.100.2:23232"
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/login", recorder.Header().Get("Location"))
	})

	t.Run("setup route unavailable in trusted-proxy mode", func(t *testing.T) {
		t.Parallel()
		app, _, _ := testHandlers(t, "")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/setup", nil))
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("missing administrator group is forbidden", func(t *testing.T) {
		t.Parallel()
		app, _, _ := testHandlers(t, "")
		request := authenticatedRequest(http.MethodGet, "/", "")
		request.Header.Set("X-Forwarded-Groups", "unprivileged")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("wrong form origin is rejected", func(t *testing.T) {
		t.Parallel()
		app, _, api := testHandlers(t, "")
		request := authenticatedRequest(http.MethodPost, "/maintenance/prune", "")
		request.Header.Set("Origin", "https://attacker.example")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusForbidden, recorder.Code)
		assert.Zero(t, api.maintenanceCalls.Load())
	})

	t.Run("missing form origin is rejected", func(t *testing.T) {
		t.Parallel()
		app, _, api := testHandlers(t, "")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, authenticatedRequest(http.MethodPost, "/maintenance/prune", ""))
		assert.Equal(t, http.StatusForbidden, recorder.Code)
		assert.Zero(t, api.maintenanceCalls.Load())
	})

	t.Run("ban form sends a duration to the shared Warden client", func(t *testing.T) {
		t.Parallel()
		app, _, api := testHandlers(t, "")
		request := authenticatedRequest(http.MethodPost, "/bans", "ip=203.0.113.1&duration=15m&reason=incident")
		request.Header.Set("Origin", "http://127.0.0.1:8082")
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.EqualValues(t, 1, api.banCalls.Load())
		assert.Equal(t, int64(15*time.Minute), api.banDuration.Load())
	})

	t.Run("invalid ban duration never calls Warden", func(t *testing.T) {
		t.Parallel()
		app, _, api := testHandlers(t, "")
		request := authenticatedRequest(http.MethodPost, "/bans", "ip=203.0.113.1&duration=wrong&reason=incident")
		request.Header.Set("Origin", "http://127.0.0.1:8082")
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Zero(t, api.banCalls.Load())
	})

	t.Run("same origin form succeeds", func(t *testing.T) {
		t.Parallel()
		app, _, api := testHandlers(t, "")
		request := authenticatedRequest(http.MethodPost, "/maintenance/prune", "")
		request.Header.Set("Origin", "http://127.0.0.1:8082")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, int32(1), api.maintenanceCalls.Load())
	})

	t.Run("invalid maintenance operation is not routed upstream", func(t *testing.T) {
		t.Parallel()
		app, _, api := testHandlers(t, "")
		request := authenticatedRequest(http.MethodPost, "/maintenance/vacuum", "")
		request.Header.Set("Origin", "http://127.0.0.1:8082")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusNotFound, recorder.Code)
		assert.Zero(t, api.maintenanceCalls.Load())
	})
}

func TestListenerIsolation(t *testing.T) {
	t.Parallel()

	t.Run("health remains outside application prefix", func(t *testing.T) {
		t.Parallel()
		app, management, _ := testHandlers(t, "/ui")
		recorder := httptest.NewRecorder()
		management.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		assert.Equal(t, http.StatusNoContent, recorder.Code)

		recorder = httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("readiness remains outside application prefix", func(t *testing.T) {
		t.Parallel()
		app, management, _ := testHandlers(t, "/ui")
		recorder := httptest.NewRecorder()
		management.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		assert.Equal(t, http.StatusNoContent, recorder.Code)

		recorder = httptest.NewRecorder()
		app.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("prefixed dashboard is served", func(t *testing.T) {
		t.Parallel()
		app, _, _ := testHandlers(t, "/ui")
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, "/ui/", ""))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("prefixed browser redirects preserve the prefix", func(t *testing.T) {
		t.Parallel()
		app, _, _ := testHandlers(t, "/ui")
		request := authenticatedRequest(http.MethodGet, "/ui/", "")
		request.RemoteAddr = "198.51.100.2:23232"
		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/ui/login", recorder.Header().Get("Location"))
	})

	t.Run("prefixed maintenance redirect preserves the prefix", func(t *testing.T) {
		t.Parallel()

		app, _, api := testHandlers(t, "/ui")
		request := authenticatedRequest(http.MethodPost, "/ui/maintenance/prune", "")
		request.Header.Set("Origin", "http://127.0.0.1:8082")

		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/ui/maintenance?done=prune", recorder.Header().Get("Location"))
		assert.EqualValues(t, 1, api.maintenanceCalls.Load())
	})

	t.Run("prefixed manual ban redirect preserves the prefix", func(t *testing.T) {
		t.Parallel()

		app, _, api := testHandlers(t, "/ui")
		request := authenticatedRequest(http.MethodPost, "/ui/bans", "ip=203.0.113.1&duration=15m&reason=incident")
		request.Header.Set("Origin", "http://127.0.0.1:8082")
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		recorder := httptest.NewRecorder()
		app.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusSeeOther, recorder.Code)
		assert.Equal(t, "/ui/?notice=ban-created", recorder.Header().Get("Location"))
		assert.EqualValues(t, 1, api.banCalls.Load())
	})
}

func TestThemePreference(t *testing.T) {
	t.Parallel()
	app, _, _ := testHandlers(t, "/ui")
	request := authenticatedRequest(http.MethodPost, "/ui/preferences/theme", "theme=Nord")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://127.0.0.1:8082")
	saved := httptest.NewRecorder()
	app.ServeHTTP(saved, request)
	require.Equal(t, http.StatusSeeOther, saved.Code)
	assert.Equal(t, "/ui/preferences", saved.Header().Get("Location"))

	page := httptest.NewRecorder()
	app.ServeHTTP(page, authenticatedRequest(http.MethodGet, "/ui/preferences", ""))
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), "/ui/theme.css?name=Nord")
	assert.Regexp(t, `value="Nord"\s+selected`, page.Body.String())

	sheet := httptest.NewRecorder()
	app.ServeHTTP(sheet, httptest.NewRequest(http.MethodGet, "/ui/theme.css?name=Nord", nil))
	require.Equal(t, http.StatusOK, sheet.Code)
	assert.Contains(t, sheet.Body.String(), "color-scheme: dark")
	assert.Contains(t, sheet.Header().Get("Content-Type"), "text/css")
}

// TestBranding verifies that brand assets are linked and publicly served under the application prefix.
func TestBranding(t *testing.T) {
	t.Parallel()

	t.Run("renders favicon links and the logo under a route prefix", func(t *testing.T) {
		t.Parallel()

		application, _, _ := testHandlers(t, "/ui")
		response := httptest.NewRecorder()
		application.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/ui/", ""))

		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `<title>Active bans · Warden WebUI</title>`)
		assert.Contains(t, body, `href="/ui/static/assets/favicon-16x16.png"`)
		assert.Contains(t, body, `href="/ui/static/assets/favicon-32x32.png"`)
		assert.Contains(t, body, `href="/ui/static/assets/warden-webui.svg"`)
		assert.Contains(t, body, `href="/ui/static/assets/apple-touch-icon.png"`)
		assert.Contains(t, body, `src="/ui/static/assets/warden-webui.svg"`)
	})

	t.Run("serves the SVG logo without authentication", func(t *testing.T) {
		t.Parallel()

		application, _, _ := testHandlers(t, "/ui")
		response := httptest.NewRecorder()
		application.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ui/static/assets/warden-webui.svg", nil))

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Header().Get("Content-Type"), "image/svg+xml")
		assert.Contains(t, response.Body.String(), "<svg")
	})

	t.Run("serves the 16 pixel favicon without authentication", func(t *testing.T) {
		t.Parallel()

		application, _, _ := testHandlers(t, "/ui")
		response := httptest.NewRecorder()
		application.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ui/static/assets/favicon-16x16.png", nil))

		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "image/png", response.Header().Get("Content-Type"))
		assert.NotEmpty(t, response.Body.Bytes())
	})

	t.Run("serves the 32 pixel favicon without authentication", func(t *testing.T) {
		t.Parallel()

		application, _, _ := testHandlers(t, "/ui")
		response := httptest.NewRecorder()
		application.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ui/static/assets/favicon-32x32.png", nil))

		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "image/png", response.Header().Get("Content-Type"))
		assert.NotEmpty(t, response.Body.Bytes())
	})

	t.Run("serves the Apple touch icon without authentication", func(t *testing.T) {
		t.Parallel()

		application, _, _ := testHandlers(t, "/ui")
		response := httptest.NewRecorder()
		application.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ui/static/assets/apple-touch-icon.png", nil))

		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "image/png", response.Header().Get("Content-Type"))
		assert.NotEmpty(t, response.Body.Bytes())
	})
}
