package endpoint

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/keycloak-warden/pkg/deliverystate"
	"github.com/gi8lino/warden-webui/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryQueries(t *testing.T) {
	t.Parallel()
	t.Run("default page", func(t *testing.T) {
		t.Parallel()
		page, revision, err := parseHistoryQuery("")
		require.NoError(t, err)
		assert.Equal(t, 1, page)
		assert.Zero(t, revision)
	})
	t.Run("pinned history page", func(t *testing.T) {
		t.Parallel()
		page, revision, err := parseHistoryQuery("history-page=2&history-through-revision=9")
		require.NoError(t, err)
		assert.Equal(t, 2, page)
		assert.EqualValues(t, 9, revision)
	})
	t.Run("rejects duplicate selectors", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseHistoryQuery("history-page=1&history-page=2")
		require.Error(t, err)
	})
	t.Run("rejects nonpositive page", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseHistoryQuery("history-page=0")
		require.Error(t, err)
	})
	t.Run("rejects invalid revision", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseHistoryQuery("history-through-revision=-1")
		require.Error(t, err)
	})
	t.Run("rejects overflowing page", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseHistoryQuery("history-page=99999999999999999999999999")
		require.Error(t, err)
	})
	t.Run("rejects unknown selectors", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseHistoryQuery("other=1")
		require.Error(t, err)
	})
}

func TestHistoryNavigation(t *testing.T) {
	t.Parallel()

	t.Run("pins both directions to the original revision", func(t *testing.T) {
		t.Parallel()

		view := historyView("/ui", "one", 2, 9, 3, nil, nil)

		assert.Equal(t, "/ui/bans/one?history-page=1&history-through-revision=9#delivery-history", view.Previous)
		assert.Equal(t, "/ui/bans/one?history-page=3&history-through-revision=9#delivery-history", view.Next)
	})

	t.Run("refreshes the latest page at the history section", func(t *testing.T) {
		t.Parallel()

		view := historyView("/ui", "one", 2, 9, 0, nil, nil)

		assert.Equal(t, "/ui/bans/one#delivery-history", view.Latest)
		assert.Empty(t, view.Next)
	})

	t.Run("later pages still navigate within the pinned revision", func(t *testing.T) {
		t.Parallel()

		view := historyView("/ui", "one", 3, 9, 4, nil, nil)

		assert.Equal(t, "/ui/bans/one?history-page=2&history-through-revision=9#delivery-history", view.Previous)
		assert.Equal(t, "/ui/bans/one?history-page=4&history-through-revision=9#delivery-history", view.Next)
	})
}

func TestDeliveryHistoryPage(t *testing.T) {
	t.Parallel()
	record := client.Record{
		Decision:          client.Decision{ID: "one", Revision: 9},
		Deliveries:        []client.Acknowledgement{{Destination: "webhook.receiver", Revision: 8}},
		CurrentDeliveries: []client.Delivery{{Destination: "webhook.receiver", Revision: 9, Status: deliverystate.StatusRetrying}},
		DeliveryHistory:   []client.Delivery{{Destination: "webhook.receiver", Revision: 9, Status: deliverystate.StatusRetrying}},
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bans/one", r.URL.Path)
		response := record
		if r.URL.Query().Get("history-page") == "2" {
			assert.Equal(t, "8", r.URL.Query().Get("history-through-revision"))
			response.HistoryPage = 2
			response.HistoryNextPage = 3
			response.DeliveryHistory = []client.Delivery{{
				Destination: "webhook.receiver", Revision: 8,
				Status: deliverystate.StatusDelivered, AttemptsTruncated: true,
				Attempts: []client.DeliveryAttempt{
					{Number: 1, At: time.Now(), Outcome: deliverystate.OutcomeFailed, Error: "<script>unsafe()</script>"},
					{Number: 2, At: time.Now(), Outcome: deliverystate.OutcomeSucceeded},
				},
			}}
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer backend.Close()
	api, err := client.New(backend.URL, "secret")
	require.NoError(t, err)
	pages, err := NewPages(web.Assets, "/ui", appkitauth.ModeLocal, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "", "test")
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet, "/ui/bans/one?history-page=2&history-through-revision=8", nil)
	request.SetPathValue("id", "one")
	response := httptest.NewRecorder()
	BanDetails(api, pages)(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "retrying")
	assert.Contains(t, body, `aria-label="Decision summary"`)
	assert.Contains(t, body, `id="current-deliveries"`)
	assert.Contains(t, body, `data-delivery-destination="webhook.receiver"`)
	assert.Contains(t, body, `aria-label="Delivery history timeline"`)
	assert.Contains(t, body, `lg:grid-cols-2`)
	assert.Contains(t, body, "Revision 8")
	assert.Contains(t, body, "Older attempts in this group were omitted")
	assert.Contains(t, body, "/ui/bans/one?history-page=3&amp;history-through-revision=8#delivery-history")
	assert.Contains(t, body, "/ui/bans/one#delivery-history")
	assert.Contains(t, body, "&lt;script&gt;unsafe()&lt;/script&gt;")
	assert.NotContains(t, body, "<script>unsafe()")
}

func TestDeliveryHistorySnapshots(t *testing.T) {
	t.Parallel()

	t.Run("historical pages cannot replace the current decision or progress", func(t *testing.T) {
		t.Parallel()

		latest := client.Record{
			Decision: client.Decision{
				ID:       "one",
				IP:       "203.0.113.11",
				Revision: 11,
			},
			Deliveries: []client.Acknowledgement{
				{Destination: "webhook.receiver", Revision: 10},
			},
			CurrentDeliveries: []client.Delivery{
				{Destination: "webhook.receiver", Revision: 11, Status: deliverystate.StatusRetrying},
			},
		}
		historical := client.Record{
			Decision: client.Decision{
				ID:       "one",
				IP:       "203.0.113.8",
				Revision: 8,
			},
			Deliveries: []client.Acknowledgement{
				{Destination: "webhook.receiver", Revision: 8},
			},
			DeliveryHistory: []client.Delivery{
				{Destination: "webhook.receiver", Revision: 8, Status: deliverystate.StatusDelivered},
			},
		}

		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/bans/one", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")

			if r.URL.Query().Get(historyPageParam) == "2" {
				assert.Equal(t, "8", r.URL.Query().Get(historyThroughRevisionParam))
				assert.NoError(t, json.NewEncoder(w).Encode(historical))
				return
			}

			assert.NoError(t, json.NewEncoder(w).Encode(latest))
		}))
		t.Cleanup(backend.Close)

		api, err := client.New(backend.URL, "secret")
		require.NoError(t, err)

		pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "", "test")
		require.NoError(t, err)

		request := httptest.NewRequest(http.MethodGet, "/bans/one?history-page=2&history-through-revision=8", nil)
		request.SetPathValue("id", "one")
		response := httptest.NewRecorder()

		BanDetails(api, pages)(response, request)

		require.Equal(t, http.StatusOK, response.Code)
		sections := strings.SplitN(response.Body.String(), `<section id="delivery-history"`, 2)
		require.Len(t, sections, 2)
		assert.Contains(t, sections[0], "203.0.113.11")
		assert.NotContains(t, sections[0], "203.0.113.8")
		assert.Contains(t, sections[0], "retrying")
		assert.NotContains(t, sections[0], ">delivered</span>")
		assert.Contains(t, sections[1], "Revision 8")
		assert.Contains(t, sections[1], ">delivered</span>")
	})

	t.Run("an unpinned older page uses the current revision as its boundary", func(t *testing.T) {
		t.Parallel()

		latest := client.Record{
			Decision: client.Decision{
				ID:       "one",
				IP:       "203.0.113.9",
				Revision: 9,
			},
			HistoryNextPage: 2,
		}
		advanced := client.Record{
			Decision: client.Decision{
				ID:       "one",
				IP:       "203.0.113.10",
				Revision: 10,
			},
			HistoryPage:     2,
			HistoryNextPage: 3,
			DeliveryHistory: []client.Delivery{
				{Destination: "webhook.receiver", Revision: 8, Status: deliverystate.StatusDelivered},
			},
		}

		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/bans/one", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")

			if r.URL.Query().Get(historyPageParam) == "2" {
				assert.Equal(t, "9", r.URL.Query().Get(historyThroughRevisionParam))
				assert.NoError(t, json.NewEncoder(w).Encode(advanced))
				return
			}

			assert.NoError(t, json.NewEncoder(w).Encode(latest))
		}))
		t.Cleanup(backend.Close)

		api, err := client.New(backend.URL, "secret")
		require.NoError(t, err)

		pages, err := NewPages(web.Assets, "/ui", appkitauth.ModeLocal, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "", "test")
		require.NoError(t, err)

		request := httptest.NewRequest(http.MethodGet, "/ui/bans/one?history-page=2", nil)
		request.SetPathValue("id", "one")
		response := httptest.NewRecorder()

		BanDetails(api, pages)(response, request)

		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "203.0.113.9")
		assert.NotContains(t, body, "203.0.113.10")
		assert.Contains(t, body, "Revision 8")
		assert.Contains(t, body, "/ui/bans/one?history-page=3&amp;history-through-revision=9#delivery-history")
	})
}

func TestDeliveryHistoryWithoutAcknowledgements(t *testing.T) {
	t.Parallel()

	record := client.Record{
		Decision: client.Decision{
			ID:       "one",
			Revision: 3,
		},
		CurrentDeliveries: []client.Delivery{
			{Destination: "webhook.retrying", Revision: 3, Status: deliverystate.StatusRetrying},
			{Destination: "webhook.exhausted", Revision: 3, Status: deliverystate.StatusExhausted},
		},
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bans/one", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(record))
	}))
	t.Cleanup(backend.Close)

	api, err := client.New(backend.URL, "secret")
	require.NoError(t, err)

	pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "", "test")
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodGet, "/bans/one", nil)
	request.SetPathValue("id", "one")
	response := httptest.NewRecorder()

	BanDetails(api, pages)(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	sections := strings.SplitN(response.Body.String(), `<section id="delivery-history"`, 2)
	require.Len(t, sections, 2)
	assert.Contains(t, sections[0], "webhook.retrying")
	assert.Contains(t, sections[0], "webhook.exhausted")
	assert.Contains(t, sections[0], "retrying")
	assert.Contains(t, sections[0], "exhausted")
	assert.Contains(t, sections[0], "—")
	assert.NotContains(t, sections[0], "No delivery status available")
}

func TestLatestDeliveryStatus(t *testing.T) {
	t.Parallel()
	t.Run("older history cannot describe current status", func(t *testing.T) {
		t.Parallel()
		status := deliveryStatus(client.Acknowledgement{Destination: "a", Revision: 1}, 2,
			[]client.Delivery{{Destination: "a", Revision: 1, Status: deliverystate.StatusDelivered}})
		assert.False(t, status.Known)
		assert.Empty(t, status.Status)
	})
	t.Run("current acknowledgement confirms delivery", func(t *testing.T) {
		t.Parallel()
		status := deliveryStatus(client.Acknowledgement{Revision: 2}, 2, nil)
		assert.True(t, status.Known)
		assert.Equal(t, deliverystate.StatusDelivered, status.Status)
	})
	t.Run("current history preserves its enum", func(t *testing.T) {
		t.Parallel()
		status := deliveryStatus(client.Acknowledgement{Destination: "a", Revision: 1}, 2,
			[]client.Delivery{{Destination: "a", Revision: 2, Status: deliverystate.StatusRetrying}})
		assert.True(t, status.Known)
		assert.Equal(t, deliverystate.StatusRetrying, status.Status)
	})
	t.Run("unknown backend status is not treated as known", func(t *testing.T) {
		t.Parallel()
		status := deliveryStatus(client.Acknowledgement{Destination: "a"}, 2,
			[]client.Delivery{{Destination: "a", Revision: 2, Status: deliverystate.Status("future")}})
		assert.False(t, status.Known)
	})
	t.Run("missing acknowledgement does not imply delivered revision zero", func(t *testing.T) {
		t.Parallel()

		status := deliveryStatus(client.Acknowledgement{Destination: "a"}, 0, nil)

		assert.False(t, status.Known)
	})

	t.Run("reversing attempts preserves API data", func(t *testing.T) {
		t.Parallel()
		attempts := []client.DeliveryAttempt{{Number: 1}, {Number: 2}}
		assert.EqualValues(t, 2, newestAttempts(attempts)[0].Number)
		assert.EqualValues(t, 1, attempts[0].Number)
	})
}

func TestCurrentDeliveryViews(t *testing.T) {
	t.Parallel()

	t.Run("includes current progress without a prior acknowledgement", func(t *testing.T) {
		t.Parallel()

		record := client.Record{
			Decision: client.Decision{Revision: 3},
			CurrentDeliveries: []client.Delivery{
				{Destination: "webhook.retrying", Revision: 3, Status: deliverystate.StatusRetrying},
				{Destination: "webhook.exhausted", Revision: 3, Status: deliverystate.StatusExhausted},
			},
		}

		views := currentDeliveryViews(record)

		require.Len(t, views, 2)
		assert.Equal(t, "webhook.exhausted", views[0].Destination)
		assert.False(t, views[0].HasAcknowledgement)
		assert.Equal(t, deliverystate.StatusExhausted, views[0].Progress.Status)
		assert.True(t, views[0].Progress.Known)
		assert.Equal(t, "webhook.retrying", views[1].Destination)
		assert.False(t, views[1].HasAcknowledgement)
		assert.Equal(t, deliverystate.StatusRetrying, views[1].Progress.Status)
		assert.True(t, views[1].Progress.Known)
	})

	t.Run("combines acknowledged and unacknowledged destinations without duplicates", func(t *testing.T) {
		t.Parallel()

		record := client.Record{
			Decision: client.Decision{Revision: 3},
			Deliveries: []client.Acknowledgement{
				{Destination: "webhook.acknowledged", Revision: 3},
				{Destination: "webhook.retrying", Revision: 2},
			},
			CurrentDeliveries: []client.Delivery{
				{Destination: "webhook.retrying", Revision: 3, Status: deliverystate.StatusRetrying},
				{Destination: "webhook.unacknowledged", Revision: 3, Status: deliverystate.StatusExhausted},
			},
		}

		views := currentDeliveryViews(record)

		require.Len(t, views, 3)
		assert.Equal(t, "webhook.acknowledged", views[0].Destination)
		assert.EqualValues(t, 3, views[0].AcknowledgedRevision)
		assert.Equal(t, deliverystate.StatusDelivered, views[0].Progress.Status)
		assert.Equal(t, "webhook.retrying", views[1].Destination)
		assert.EqualValues(t, 2, views[1].AcknowledgedRevision)
		assert.Equal(t, deliverystate.StatusRetrying, views[1].Progress.Status)
		assert.Equal(t, "webhook.unacknowledged", views[2].Destination)
		assert.False(t, views[2].HasAcknowledgement)
	})

	t.Run("delivered release retains acknowledged revision after its queue row is removed", func(t *testing.T) {
		t.Parallel()

		record := client.Record{
			Decision: client.Decision{Revision: 4},
			CurrentDeliveries: []client.Delivery{
				{Destination: "webhook.released", Revision: 4, Status: deliverystate.StatusDelivered},
			},
		}

		views := currentDeliveryViews(record)

		require.Len(t, views, 1)
		assert.True(t, views[0].HasAcknowledgement)
		assert.Equal(t, int64(4), views[0].AcknowledgedRevision)
		assert.Equal(t, deliverystate.StatusDelivered, views[0].Progress.Status)
	})

	t.Run("older history cannot introduce destinations into current progress", func(t *testing.T) {
		t.Parallel()

		record := client.Record{
			Decision: client.Decision{Revision: 3},
			Deliveries: []client.Acknowledgement{
				{Destination: "webhook.known", Revision: 2},
			},
			CurrentDeliveries: []client.Delivery{
				{Destination: "webhook.old", Revision: 2, Status: deliverystate.StatusDelivered},
			},
		}

		views := currentDeliveryViews(record)

		require.Len(t, views, 1)
		assert.Equal(t, "webhook.known", views[0].Destination)
		assert.False(t, views[0].Progress.Known)
	})
}

// missingHistoryAPI fails a paged request after successfully returning current state.
type missingHistoryAPI struct {
	// API embeds unused operations for this focused endpoint test.
	API
}

// Get returns current state so history retrieval can be tested independently.
func (missingHistoryAPI) Get(context.Context, string) (client.Record, error) {
	return client.Record{}, nil
}

// GetPageThroughRevision simulates an unavailable history backend.
func (missingHistoryAPI) GetPageThroughRevision(context.Context, string, int, int64) (client.Record, error) {
	return client.Record{}, &client.Error{Code: http.StatusServiceUnavailable, Message: "private failure"}
}

func TestHistoryFailure(t *testing.T) {
	t.Parallel()
	pages, err := NewPages(web.Assets, "", appkitauth.ModeLocal, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "", "test")
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet, "/bans/one?history-page=2", nil)
	request.SetPathValue("id", "one")
	response := httptest.NewRecorder()
	BanDetails(missingHistoryAPI{}, pages)(response, request)
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.Contains(t, response.Body.String(), "currently unavailable")
	assert.NotContains(t, response.Body.String(), "private failure")
}

func TestCurrentDeliveryViewsIndependentOfHistoryPage(t *testing.T) {
	t.Parallel()

	t.Run("includes an unacknowledged destination outside the paginated timeline", func(t *testing.T) {
		t.Parallel()

		record := client.Record{
			Decision: client.Decision{Revision: 3},
			CurrentDeliveries: []client.Delivery{
				{Destination: "webhook.first", Revision: 3, Status: deliverystate.StatusDelivered},
				{Destination: "webhook.last", Revision: 3, Status: deliverystate.StatusExhausted},
			},
			DeliveryHistory: []client.Delivery{
				{Destination: "webhook.first", Revision: 3, Status: deliverystate.StatusDelivered},
			},
		}

		views := currentDeliveryViews(record)

		require.Len(t, views, 2)
		assert.Equal(t, "webhook.last", views[1].Destination)
		assert.Equal(t, deliverystate.StatusExhausted, views[1].Progress.Status)
		assert.False(t, views[1].HasAcknowledgement)
	})
}
