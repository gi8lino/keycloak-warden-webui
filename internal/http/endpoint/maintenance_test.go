package endpoint

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/keycloak-warden/pkg/client"
)

// TestIsWorkerStatusUnsupported classifies only missing worker status routes as unsupported.
func TestIsWorkerStatusUnsupported(t *testing.T) {
	t.Parallel()

	t.Run("recognizes an older Warden release", func(t *testing.T) {
		t.Parallel()

		err := &client.Error{Code: http.StatusNotFound}

		assert.True(t, isWorkerStatusUnsupported(err))
	})

	t.Run("recognizes wrapped upstream not found", func(t *testing.T) {
		t.Parallel()

		err := errors.Join(errors.New("worker status"), &client.Error{Code: http.StatusNotFound})

		assert.True(t, isWorkerStatusUnsupported(err))
	})

	t.Run("does not misclassify authorization failures", func(t *testing.T) {
		t.Parallel()

		err := &client.Error{Code: http.StatusForbidden}

		assert.False(t, isWorkerStatusUnsupported(err))
	})

	t.Run("does not misclassify network errors", func(t *testing.T) {
		t.Parallel()

		err := errors.New("connection refused")

		require.Error(t, err)
		assert.False(t, isWorkerStatusUnsupported(err))
	})
}
