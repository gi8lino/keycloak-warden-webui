package bootstrap

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/warden-webui/internal/flags"
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
	cfg, err := flags.Parse([]string{
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
	httpConfig, err := NewHTTPConfig(web.Assets, cfg, infrastructure, application, logger)
	require.NoError(t, err)

	// The configured upstream client keeps administrative credentials on the server.
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

	// Closing the infrastructure closes the same store used by HTTP readiness.
	require.NoError(t, infrastructure.Close())
	closed := httptest.NewRecorder()
	handlers.Management.ServeHTTP(closed, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	assert.Equal(t, http.StatusServiceUnavailable, closed.Code)
}
