package endpoint

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gi8lino/keycloak-warden/pkg/model"
	"github.com/gi8lino/warden-webui/internal/http/auth"
	"github.com/gi8lino/warden-webui/web"
)

// retryAPI captures the browser request translated into Warden's administrative contract.
type retryAPI struct {
	// API embeds unused operations.
	API
	// request captures the exact retry request.
	request model.RetryRequest
}

// RetryDelivery records the request for assertions.
func (a *retryAPI) RetryDelivery(_ context.Context, request model.RetryRequest) error {
	a.request = request
	return nil
}

func TestRetryDelivery(t *testing.T) {
	t.Parallel()

	t.Run("uses authenticated actor instead of submitted identity", func(t *testing.T) {
		t.Parallel()
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "", "test")
		require.NoError(t, err)
		api := &retryAPI{}
		request := httptest.NewRequest(http.MethodPost, "/bans/one/retry", strings.NewReader("revision=3&destination=webhook.edge&actor=forged"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.SetPathValue("id", "one")
		request = auth.WithUser(request, auth.User{Name: "operator", Local: true})
		response := httptest.NewRecorder()
		RetryDelivery(api, pages)(response, request)
		assert.Equal(t, http.StatusSeeOther, response.Code)
		assert.Equal(t, "/deliveries?notice=retry-queued", response.Header().Get("Location"))
		assert.Equal(t, model.RetryRequest{DecisionID: "one", Revision: 3, Destination: "webhook.edge", Actor: "operator"}, api.request)
	})

	t.Run("rejects duplicate revision selector", func(t *testing.T) {
		t.Parallel()
		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "", "test")
		require.NoError(t, err)
		api := &retryAPI{}
		request := httptest.NewRequest(http.MethodPost, "/bans/one/retry", strings.NewReader("revision=3&revision=4&destination=webhook.edge"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.SetPathValue("id", "one")
		response := httptest.NewRecorder()
		RetryDelivery(api, pages)(response, request)
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Empty(t, api.request.DecisionID)
	})
}
