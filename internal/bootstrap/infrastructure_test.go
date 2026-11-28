package bootstrap

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/warden-webui/internal/cli"
	"github.com/gi8lino/warden-webui/internal/http/server"
	"github.com/gi8lino/warden-webui/web"
)

func TestBootstrapComposesRuntime(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bans", r.URL.Path)
		assert.Equal(t, "Bearer private-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	t.Cleanup(upstream.Close)
	cfg, err := cli.Parse([]string{
		"--database", filepath.Join(t.TempDir(), "webui.db"),
		"--warden-url", upstream.URL,
		"--warden-admin-token", "private-token",
		"--setup-token", "long-test-setup-token-0123456789abcdef",
	}, "test")
	require.NoError(t, err)

	infrastructure, err := NewInfrastructure(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = infrastructure.Close() })
	application, err := NewApplication(t.Context(), cfg, infrastructure)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	httpConfig, err := NewHTTPConfig(web.Assets, cfg, infrastructure, application, logger, "test")
	require.NoError(t, err)

	_, err = httpConfig.Warden.List(t.Context())
	require.NoError(t, err)

	handlers := server.New(httpConfig)
	setup := httptest.NewRecorder()
	handlers.Application.ServeHTTP(setup, httptest.NewRequest(http.MethodGet, "/setup", nil))

	assert.Equal(t, http.StatusOK, setup.Code)
	assert.Contains(t, setup.Body.String(), "Create administrator")
	assert.NotContains(t, setup.Body.String(), "private-token")

	ready := httptest.NewRecorder()
	handlers.Management.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assert.Equal(t, http.StatusNoContent, ready.Code)

	require.NoError(t, infrastructure.Close())

	closed := httptest.NewRecorder()
	handlers.Management.ServeHTTP(closed, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assert.Equal(t, http.StatusServiceUnavailable, closed.Code)
}

// TestNewInfrastructureHTTPPolicy verifies that WebUI passes its URL policy to Warden's public client.
func TestNewInfrastructureHTTPPolicy(t *testing.T) {
	t.Parallel()

	t.Run("rejects remote HTTP by default", func(t *testing.T) {
		t.Parallel()

		cfg, err := cli.Parse([]string{
			"--database=" + filepath.Join(t.TempDir(), "webui.db"),
			"--warden-url=http://keycloak-warden.keycloak.svc.cluster.local:8080",
			"--warden-admin-token=private-token",
		}, "test")
		require.NoError(t, err)

		infrastructure, err := NewInfrastructure(t.Context(), cfg)

		require.ErrorContains(t, err, "requires HTTPS")
		assert.Nil(t, infrastructure)
	})

	t.Run("accepts remote HTTP with opt-in", func(t *testing.T) {
		t.Parallel()

		cfg, err := cli.Parse([]string{
			"--database=" + filepath.Join(t.TempDir(), "webui.db"),
			"--warden-url=http://keycloak-warden.keycloak.svc.cluster.local:8080",
			"--warden-admin-token=private-token",
			"--allow-insecure-warden-http=true",
		}, "test")
		require.NoError(t, err)

		infrastructure, err := NewInfrastructure(t.Context(), cfg)

		require.NoError(t, err)
		require.NotNil(t, infrastructure)
		t.Cleanup(func() { require.NoError(t, infrastructure.Close()) })
	})

	t.Run("still rejects unsupported URL schemes with opt-in", func(t *testing.T) {
		t.Parallel()

		cfg, err := cli.Parse([]string{
			"--database=" + filepath.Join(t.TempDir(), "webui.db"),
			"--warden-url=ftp://keycloak-warden.keycloak.svc.cluster.local:8080",
			"--warden-admin-token=private-token",
			"--allow-insecure-warden-http=true",
		}, "test")
		require.NoError(t, err)

		infrastructure, err := NewInfrastructure(t.Context(), cfg)

		require.ErrorContains(t, err, "requires HTTPS")
		assert.Nil(t, infrastructure)
	})

	t.Run("accepts loopback HTTP without opt-in", func(t *testing.T) {
		t.Parallel()

		cfg, err := cli.Parse([]string{
			"--database=" + filepath.Join(t.TempDir(), "webui.db"),
			"--warden-url=http://127.0.0.1:8080",
			"--warden-admin-token=private-token",
		}, "test")
		require.NoError(t, err)

		infrastructure, err := NewInfrastructure(t.Context(), cfg)

		require.NoError(t, err)
		require.NotNil(t, infrastructure)
		t.Cleanup(func() { require.NoError(t, infrastructure.Close()) })
	})
}

// TestNewInfrastructureClientLimits verifies the parsed flags affect actual Warden API calls.
func TestNewInfrastructureClientLimits(t *testing.T) {
	t.Parallel()

	t.Run("enforces the configured maximum response size", func(t *testing.T) {
		t.Parallel()

		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[]`)
		}))
		t.Cleanup(upstream.Close)

		cfg, err := cli.Parse([]string{
			"--database=" + filepath.Join(t.TempDir(), "webui.db"),
			"--warden-url=" + upstream.URL,
			"--warden-admin-token=private-token",
			"--warden-max-response-bytes=1",
		}, "test")
		require.NoError(t, err)

		infrastructure, err := NewInfrastructure(t.Context(), cfg)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, infrastructure.Close()) })

		_, err = infrastructure.warden.List(t.Context())

		require.ErrorContains(t, err, "application response exceeds 1 bytes")
	})

	t.Run("enforces the ordinary request timeout", func(t *testing.T) {
		t.Parallel()

		upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		t.Cleanup(upstream.Close)

		cfg, err := cli.Parse([]string{
			"--database=" + filepath.Join(t.TempDir(), "webui.db"),
			"--warden-url=" + upstream.URL,
			"--warden-admin-token=private-token",
			"--warden-request-timeout=2ms",
		}, "test")
		require.NoError(t, err)

		infrastructure, err := NewInfrastructure(t.Context(), cfg)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, infrastructure.Close()) })

		_, err = infrastructure.warden.List(t.Context())

		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("enforces the separate maintenance timeout", func(t *testing.T) {
		t.Parallel()

		upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		t.Cleanup(upstream.Close)

		cfg, err := cli.Parse([]string{
			"--database=" + filepath.Join(t.TempDir(), "webui.db"),
			"--warden-url=" + upstream.URL,
			"--warden-admin-token=private-token",
			"--warden-request-timeout=5s",
			"--warden-maintenance-timeout=2ms",
		}, "test")
		require.NoError(t, err)

		infrastructure, err := NewInfrastructure(t.Context(), cfg)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, infrastructure.Close()) })

		_, err = infrastructure.warden.MaintenanceStatus(t.Context())

		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
