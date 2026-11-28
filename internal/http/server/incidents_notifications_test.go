package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncidentAndNotificationRoutes(t *testing.T) {
	t.Parallel()

	t.Run("incidents retain filters and prefix", func(t *testing.T) {
		t.Parallel()

		app, _, api := testHandlers(t, "/ui")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/ui/incidents?rule=brute-force&kind=user&before-id=90", ""))

		assert.Equal(t, int64(90), api.incidentFilter.BeforeID)
		assert.Equal(t, "brute-force", api.incidentFilter.Rule)
		assert.Equal(t, "user", api.incidentFilter.Kind)
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "LOGIN_ERROR")
		assert.Contains(t, response.Body.String(), "/ui/incidents?before-id=42&amp;kind=user&amp;rule=brute-force")
	})

	t.Run("rejects duplicate cursor", func(t *testing.T) {
		t.Parallel()

		app, _, _ := testHandlers(t, "")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/incidents?before-id=1&before-id=2", ""))

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("rejects invalid kind", func(t *testing.T) {
		t.Parallel()

		app, _, _ := testHandlers(t, "")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/incidents?kind=unknown", ""))

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("shows exhausted notifications and escapes provider errors", func(t *testing.T) {
		t.Parallel()

		app, _, api := testHandlers(t, "/ui")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/ui/notifications?before-id=10", ""))

		assert.Equal(t, int64(10), api.notificationBeforeID)
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "slack-security")
		assert.Contains(t, response.Body.String(), "&lt;script&gt;alert(1)&lt;/script&gt;")
		assert.NotContains(t, response.Body.String(), "<script>alert(1)</script>")
		assert.Contains(t, response.Body.String(), "/ui/notifications/7/retry")
		assert.Contains(t, response.Body.String(), "/ui/notifications?before-id=7")
	})

	t.Run("shows notification attempts", func(t *testing.T) {
		t.Parallel()

		app, _, api := testHandlers(t, "/ui")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/ui/notifications/7/attempts?before-number=4", ""))

		assert.Equal(t, int64(7), api.notificationID.Load())
		assert.Equal(t, int64(4), api.attemptBeforeNumber)
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "receiver unavailable")
		assert.Contains(t, response.Body.String(), "/ui/notifications/7/attempts?before-number=3")
	})

	t.Run("retry uses authenticated identity", func(t *testing.T) {
		t.Parallel()

		app, _, api := testHandlers(t, "/ui")
		request := authenticatedRequest(http.MethodPost, "/ui/notifications/7/retry", "actor=forged")
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Origin", "http://127.0.0.1:8082")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, request)

		require.Equal(t, http.StatusSeeOther, response.Code)
		assert.Equal(t, "/ui/notifications?notice=retry-queued", response.Header().Get("Location"))
		assert.Equal(t, int64(7), api.notificationID.Load())
		assert.Equal(t, "operator", api.notificationActor)
	})

	t.Run("rejects cross-site retry", func(t *testing.T) {
		t.Parallel()

		app, _, api := testHandlers(t, "")
		request := authenticatedRequest(http.MethodPost, "/notifications/7/retry", "")
		request.Header.Set("Origin", "https://attacker.example")
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, request)

		assert.Equal(t, http.StatusForbidden, response.Code)
		assert.Zero(t, api.notificationID.Load())
	})

	t.Run("rejects invalid delivery ID", func(t *testing.T) {
		t.Parallel()

		app, _, api := testHandlers(t, "")
		request := authenticatedRequest(http.MethodPost, "/notifications/0/retry", "")
		request.Header.Set("Origin", "http://127.0.0.1:8082")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, request)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Zero(t, api.notificationID.Load())
	})

	t.Run("requires authentication", func(t *testing.T) {
		t.Parallel()

		app, _, _ := testHandlers(t, "")
		response := httptest.NewRecorder()
		request := authenticatedRequest(http.MethodGet, "/notifications", "")
		request.RemoteAddr = "198.51.100.2:23232"
		app.ServeHTTP(response, request)

		assert.Equal(t, http.StatusSeeOther, response.Code)
	})

	t.Run("observation bookmark opens incidents", func(t *testing.T) {
		t.Parallel()

		app, _, _ := testHandlers(t, "")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/observations", ""))

		assert.Equal(t, http.StatusOK, response.Code)
	})
}
