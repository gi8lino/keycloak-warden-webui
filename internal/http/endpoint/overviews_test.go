package endpoint

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOverviewQuery(t *testing.T) {
	t.Parallel()

	t.Run("accepts one-based pages and filters", func(t *testing.T) {
		t.Parallel()
		values, page, err := parseOverviewQuery("page=2&state=unbanned&ip=203.0.113.42", "ip", "state")
		require.NoError(t, err)
		assert.Equal(t, 2, page)
		assert.Equal(t, "unbanned", values.Get("state"))
	})

	t.Run("rejects duplicate selectors", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseOverviewQuery("page=1&page=2", "state")
		require.Error(t, err)
	})

	t.Run("rejects unrelated selectors", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseOverviewQuery("admin=true", "state")
		require.Error(t, err)
	})

	t.Run("rejects nonpositive pages", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseOverviewQuery("page=0", "state")
		require.Error(t, err)
	})
}

func TestOverviewLinks(t *testing.T) {
	t.Parallel()

	t.Run("retains filters and application prefix", func(t *testing.T) {
		t.Parallel()
		view := overviewLinks("/ui", "/deliveries", 2, 3, url.Values{"status": {"exhausted"}, "destination": {"cloudflare.edge"}})
		assert.Equal(t, "/ui/deliveries?destination=cloudflare.edge&page=1&status=exhausted", view.Previous)
		assert.Equal(t, "/ui/deliveries?destination=cloudflare.edge&page=3&status=exhausted", view.Next)
	})

	t.Run("first page has no newer link", func(t *testing.T) {
		t.Parallel()
		view := overviewLinks("", "/ban-history", 1, 0, url.Values{})
		assert.Empty(t, view.Previous)
		assert.Empty(t, view.Next)
	})
}
