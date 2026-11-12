package endpoint

import (
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
		pages, err := NewPages(web.Assets, "", appkitauth.ModeProxy, logger, nil, "")
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		pages.fail(httptest.NewRequest("GET", "/", nil), recorder, auth.User{Name: "operator"}, errors.New("private upstream transport details"))
		assert.Equal(t, http.StatusBadGateway, recorder.Code)
		assert.NotContains(t, recorder.Body.String(), "private upstream transport details")
	})

	t.Run("not found remains not found", func(t *testing.T) {
		t.Parallel()
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		pages, err := NewPages(web.Assets, "", appkitauth.ModeProxy, logger, nil, "")
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
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, logger, nil, "")
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
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, logger, nil, "")
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
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, logger, nil, "")
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
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, logger, nil, "")
		require.NoError(t, err)

		failure := fmt.Errorf("wrapped: %w", &client.Error{Code: 600, Message: "private details"})
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		recorder := httptest.NewRecorder()

		pages.fail(request, recorder, auth.User{Name: "operator"}, failure)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
		assert.NotContains(t, recorder.Body.String(), "private details")
	})

}
