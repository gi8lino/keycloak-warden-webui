package endpoint

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReady(t *testing.T) {
	t.Parallel()

	t.Run("healthy database is ready", func(t *testing.T) {
		t.Parallel()
		response := httptest.NewRecorder()
		Ready(func(context.Context) error { return nil })(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		assert.Equal(t, http.StatusNoContent, response.Code)
	})

	t.Run("database outage fails readiness", func(t *testing.T) {
		t.Parallel()
		response := httptest.NewRecorder()
		Ready(func(context.Context) error { return errors.New("private database error") })(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.NotContains(t, response.Body.String(), "private database error")
	})
}
