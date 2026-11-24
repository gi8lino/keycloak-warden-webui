package endpoint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/warden-webui/internal/http/auth"
	"github.com/gi8lino/warden-webui/web"
)

func TestPagesFail(t *testing.T) {
	t.Parallel()

	t.Run("upstream details are not disclosed", func(t *testing.T) {
		t.Parallel()
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		pages, err := NewPages(web.Assets, "", appkitauth.ModeProxy, logger, nil, "", "test")
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		pages.fail(httptest.NewRequest("GET", "/", nil), recorder, auth.User{Name: "operator"}, errors.New("private upstream transport details"))
		assert.Equal(t, http.StatusBadGateway, recorder.Code)
		assert.NotContains(t, recorder.Body.String(), "private upstream transport details")
	})

	t.Run("not found remains not found", func(t *testing.T) {
		t.Parallel()
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		pages, err := NewPages(web.Assets, "", appkitauth.ModeProxy, logger, nil, "", "test")
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		pages.fail(httptest.NewRequest("GET", "/", nil), recorder, auth.User{Name: "operator"}, &client.Error{Code: http.StatusNotFound, Message: "private details"})
		assert.Equal(t, http.StatusNotFound, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "The requested decision was not found.")
		assert.NotContains(t, recorder.Body.String(), "private details")
	})
}

func TestValidID(t *testing.T) {
	t.Parallel()

	t.Run("simple identifier", func(t *testing.T) {
		t.Parallel()
		assert.True(t, validID("decision-123"))
	})

	t.Run("path traversal", func(t *testing.T) {
		t.Parallel()
		assert.False(t, validID(".."))
	})

	t.Run("encoded traversal", func(t *testing.T) {
		t.Parallel()
		assert.False(t, validID("a%2fb"))
	})
}

func TestPagesFailUpstreamStatusBounds(t *testing.T) {
	t.Parallel()

	t.Run("399", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, logger, nil, "", "test")
		require.NoError(t, err)

		failure := fmt.Errorf("wrapped: %w", &client.Error{Code: 399, Message: "private details"})
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		recorder := httptest.NewRecorder()

		pages.fail(request, recorder, auth.User{Name: "operator"}, failure)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
		assert.NotContains(t, recorder.Body.String(), "private details")
	})

	t.Run("400", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, logger, nil, "", "test")
		require.NoError(t, err)

		failure := fmt.Errorf("wrapped: %w", &client.Error{Code: http.StatusBadRequest, Message: "private details"})
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		recorder := httptest.NewRecorder()

		pages.fail(request, recorder, auth.User{Name: "operator"}, failure)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.NotContains(t, recorder.Body.String(), "private details")
	})

	t.Run("599", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, logger, nil, "", "test")
		require.NoError(t, err)

		failure := fmt.Errorf("wrapped: %w", &client.Error{Code: 599, Message: "private details"})
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		recorder := httptest.NewRecorder()

		pages.fail(request, recorder, auth.User{Name: "operator"}, failure)

		assert.Equal(t, 599, recorder.Code)
		assert.NotContains(t, recorder.Body.String(), "private details")
	})

	t.Run("600", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, logger, nil, "", "test")
		require.NoError(t, err)

		failure := fmt.Errorf("wrapped: %w", &client.Error{Code: 600, Message: "private details"})
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		recorder := httptest.NewRecorder()

		pages.fail(request, recorder, auth.User{Name: "operator"}, failure)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
		assert.NotContains(t, recorder.Body.String(), "private details")
	})

}

func TestBackendErrorPresentation(t *testing.T) {
	t.Parallel()
	t.Run("unavailable backend keeps shell and hides backend details", func(t *testing.T) {
		t.Parallel()
		assertBackendErrorPage(t, &client.Error{Code: http.StatusServiceUnavailable, Message: "secret backend response"}, http.StatusServiceUnavailable, "currently unavailable")
	})
	t.Run("timeout warns against duplicate actions", func(t *testing.T) {
		t.Parallel()
		assertBackendErrorPage(t, context.DeadlineExceeded, http.StatusGatewayTimeout, "may already have completed")
	})
	t.Run("backend authorization failure explains server configuration", func(t *testing.T) {
		t.Parallel()
		assertBackendErrorPage(t, &client.Error{Code: http.StatusForbidden, Message: "secret backend response"}, http.StatusForbidden, "configured Warden API credentials")
	})
	t.Run("conflict asks to review current state", func(t *testing.T) {
		t.Parallel()
		assertBackendErrorPage(t, &client.Error{Code: http.StatusConflict, Message: "secret backend response"}, http.StatusConflict, "Review the latest data")
	})
	t.Run("validation error gives safe input guidance", func(t *testing.T) {
		t.Parallel()
		assertBackendErrorPage(t, &client.Error{Code: http.StatusBadRequest, Message: "secret backend response"}, http.StatusBadRequest, "Check your input")
	})
	t.Run("connection failure is a bad gateway", func(t *testing.T) {
		t.Parallel()
		assertBackendErrorPage(t, errors.New("secret backend response"), http.StatusBadGateway, "currently unavailable")
	})
}

// assertBackendErrorPage verifies status, safe guidance, and prefix-aware navigation together.
func assertBackendErrorPage(t *testing.T, failure error, status int, message string) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pages, err := NewPages(web.Assets, "/ui", appkitauth.ModeLocal, logger, nil, "", "test")
	require.NoError(t, err)
	response := httptest.NewRecorder()
	pages.fail(httptest.NewRequest(http.MethodGet, "/ui/", nil), response, auth.User{Name: "operator"}, failure)
	assert.Equal(t, status, response.Code)
	assert.Contains(t, response.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, response.Body.String(), message)
	assert.Contains(t, response.Body.String(), `href="/ui/"`)
	assert.Contains(t, response.Body.String(), "Version test")
	assert.Contains(t, response.Body.String(), `role="alert"`)
	assert.NotContains(t, response.Body.String(), "secret backend response")
}

// themeStoreStub verifies rendering requires only theme preference access.
type themeStoreStub struct {
	name string
}

func (s *themeStoreStub) Theme(_ context.Context, name string) (string, error) {
	s.name = name
	return "", nil
}

func (s *themeStoreStub) SaveTheme(context.Context, string, string) error {
	return nil
}

func TestPagesThemeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("accepts a narrow theme preference implementation", func(t *testing.T) {
		t.Parallel()
		store := &themeStoreStub{}
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, slog.New(slog.NewTextHandler(io.Discard, nil)), store, "", "test")
		require.NoError(t, err)
		theme := pages.accountTheme(t.Context(), "operator")
		assert.Equal(t, "operator", store.name)
		assert.NotEmpty(t, theme.Title)
	})
}
