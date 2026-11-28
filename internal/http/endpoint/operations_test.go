package endpoint

import (
	"context"
	"github.com/gi8lino/keycloak-warden/pkg/client"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// incidentAPI captures cursor-based requests without calling the removed observations API.
type incidentAPI struct {
	API
	filter client.IncidentFilter
	calls  int
	page   model.IncidentPage
}

func (a *incidentAPI) ListIncidents(_ context.Context, filter client.IncidentFilter) (model.IncidentPage, error) {
	a.calls++
	a.filter = filter

	return a.page, nil
}

func TestIncidents(t *testing.T) {
	t.Parallel()

	pages, err := NewPages(web.Assets, "/warden", appkitauth.ModeLocal, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "", "test")
	require.NoError(t, err)

	until := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

	for _, cursor := range []string{"0", "-1", "", "abc", "9223372036854775808", "1&before-id=2"} {
		t.Run("invalid "+cursor, func(t *testing.T) {
			api := &incidentAPI{}
			request := httptest.NewRequest(http.MethodGet, "/warden/incidents?before-id="+cursor, nil)
			response := httptest.NewRecorder()
			Incidents(api, pages)(response, request)

			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.Zero(t, api.calls)
		})
	}

	api := &incidentAPI{page: model.IncidentPage{NextBeforeID: 7, Items: []model.Incident{
		{Rule: "<script>rule</script>", EventKind: "user", EventType: "LOGIN_ERROR", GroupBy: "ip", GroupKey: "192.0.2.1", At: until, Count: 5, Threshold: 5, BanUntil: &until, ObserveOnly: true},
		{Rule: "admin-audit", EventKind: "admin", OperationType: "DELETE", ResourceType: "USER", At: until},
	}}}
	request := httptest.NewRequest(http.MethodGet, "/warden/incidents?before-id=42", nil)
	request = auth.WithUser(request, auth.User{Name: "operator", Local: true})
	response := httptest.NewRecorder()
	Incidents(api, pages)(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, int64(42), api.filter.BeforeID)

	body := response.Body.String()
	for _, expected := range []string{"&lt;script&gt;rule&lt;/script&gt;", "LOGIN_ERROR", "DELETE USER", "Hypothetical ban", "No ban requested", "/warden/incidents?before-id=7"} {
		assert.Contains(t, body, expected)
	}

	assert.NotContains(t, body, "<script>rule</script>")
}
