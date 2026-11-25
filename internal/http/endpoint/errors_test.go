package endpoint

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/stretchr/testify/assert"
)

func TestUpstreamErrorStatus(t *testing.T) {
	t.Parallel()

	t.Run("preserves wrapped upstream HTTP status", func(t *testing.T) {
		t.Parallel()

		err := errors.Join(errors.New("backend"), &client.Error{Code: http.StatusConflict})

		assert.Equal(t, http.StatusConflict, upstreamErrorStatus(err))
	})

	t.Run("recognizes a context deadline", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusGatewayTimeout, upstreamErrorStatus(context.DeadlineExceeded))
	})

	t.Run("uses bad gateway for unrelated errors", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusBadGateway, upstreamErrorStatus(errors.New("unavailable")))
	})
}

func TestBackendTimedOut(t *testing.T) {
	t.Parallel()

	t.Run("recognizes wrapped context deadlines", func(t *testing.T) {
		t.Parallel()

		err := errors.Join(errors.New("backend"), context.DeadlineExceeded)

		assert.True(t, backendTimedOut(err))
	})

	t.Run("recognizes timeout-capable network errors", func(t *testing.T) {
		t.Parallel()

		err := errors.Join(errors.New("transport"), &net.DNSError{IsTimeout: true})

		assert.True(t, backendTimedOut(err))
	})

	t.Run("rejects unrelated errors", func(t *testing.T) {
		t.Parallel()

		assert.False(t, backendTimedOut(errors.New("unavailable")))
	})
}
