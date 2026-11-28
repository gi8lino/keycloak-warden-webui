package endpoint

import (
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/keycloak-warden/pkg/model"
)

const (
	overviewPageParam        = "page"
	overviewIPParam          = "ip"
	overviewStateParam       = "state"
	overviewStatusParam      = "status"
	overviewDestinationParam = "destination"
	overviewThroughParam     = "through-sequence"
)

// overviewNavigation supplies bounded, prefix-aware pagination and active filters.
type overviewNavigation struct {
	// Page identifies the selected one-based listing page.
	Page int
	// Previous is the link to the next newer page.
	Previous string
	// Next is the link to the next older page.
	Next string
	// Refresh reloads the first page using the same filters.
	Refresh string
	// IP is the current exact-IP filter.
	IP string
	// State is the current desired-state filter.
	State string
	// Status is the current delivery-status filter.
	Status string
	// Destination is the current provider filter.
	Destination string
}

// parseOverviewQuery accepts only single-valued supported filters.
func parseOverviewQuery(raw string, allowed ...string) (url.Values, int, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, 0, err
	}

	page := 1
	for key, entries := range values {
		if len(entries) != 1 {
			return nil, 0, errors.New("duplicate query parameter")
		}

		if key == overviewPageParam {
			page, err = strconv.Atoi(entries[0])
			if err != nil || page < 1 {
				return nil, 0, errors.New("invalid page")
			}

			continue
		}

		if !slices.Contains(allowed, key) {
			return nil, 0, errors.New("unsupported query parameter")
		}

	}

	return values, page, nil
}

// overviewLinks preserves the selected filters across numbered pages.
func overviewLinks(prefix, path string, page, next int, filters url.Values, through int64) overviewNavigation {
	view := overviewNavigation{
		Page:        page,
		IP:          filters.Get(overviewIPParam),
		State:       filters.Get(overviewStateParam),
		Status:      filters.Get(overviewStatusParam),
		Destination: filters.Get(overviewDestinationParam),
	}

	link := func(p int, pinned bool) string {
		query := url.Values{}
		for key, values := range filters {
			if key != overviewPageParam && key != overviewThroughParam && key != "notice" {
				query[key] = values
			}
		}

		query.Set(overviewPageParam, strconv.Itoa(p))

		if pinned && through > 0 && path == "/ban-history" {
			query.Set(overviewThroughParam, strconv.FormatInt(through, 10))
		}

		return prefix + path + "?" + query.Encode()
	}

	view.Refresh = link(1, false)

	if page > 1 {
		view.Previous = link(page-1, true)
	}

	if next > 0 {
		view.Next = link(next, true)
	}

	return view
}

// BanHistory shows immutable ban/release revision events, including released decisions.
func BanHistory(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		values, page, err := parseOverviewQuery(r.URL.RawQuery, overviewIPParam, overviewStateParam, overviewThroughParam)
		if err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid ban history filters.")
			return
		}

		ip := strings.TrimSpace(values.Get(overviewIPParam))
		if ip != "" {
			address, parseErr := netip.ParseAddr(ip)
			if parseErr != nil || address.Zone() != "" || address.IsMulticast() || address.IsUnspecified() {
				pages.Problem(r, w, http.StatusBadRequest, "Invalid IP address filter.")
				return
			}

			ip = address.Unmap().String()
		}

		state := model.DecisionState(values.Get(overviewStateParam))
		if state != "" && !state.Valid() {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid decision state.")
			return
		}

		through := int64(0)
		if value := values.Get(overviewThroughParam); value != "" {
			through, err = strconv.ParseInt(value, 10, 64)
			if err != nil || through < 1 {
				pages.Problem(r, w, http.StatusBadRequest, "Invalid history sequence.")
				return
			}
		}

		list, err := api.ListDecisionHistory(r.Context(), client.RevisionFilter{
			Page:            page,
			IP:              ip,
			State:           state,
			ThroughSequence: through,
		})
		if err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}

		pages.render(r, w, http.StatusOK, pageData{
			Title:     "Ban history",
			Page:      "ban-history",
			User:      currentUser(r).Name,
			Revisions: list,
			Overview:  overviewLinks(pages.prefix, "/ban-history", page, list.NextPage, values, list.ThroughSequence),
		})
	}
}

// Deliveries shows current revision progress across all destinations and decisions.
func Deliveries(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		values, page, err := parseOverviewQuery(r.URL.RawQuery, overviewStatusParam, overviewDestinationParam, "notice")
		if err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid delivery filters.")
			return
		}

		destination := strings.TrimSpace(values.Get(overviewDestinationParam))
		if len(destination) > 128 || strings.ContainsAny(destination, "\x00\r\n\t") {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid destination filter.")
			return
		}

		status := model.DeliveryStatus(values.Get(overviewStatusParam))
		if status != "" && !status.Current() {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid delivery status.")
			return
		}

		list, err := api.ListDeliveries(r.Context(), client.DeliveryFilter{
			Page:        page,
			Status:      status,
			Destination: destination,
		})
		if err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}

		health, healthErr := api.DeliveryHealth(r.Context())
		if healthErr != nil {
			pages.logger.Warn("delivery health unavailable", "error", healthErr)
		}

		notice := ""
		if r.URL.Query().Get("notice") == "retry-queued" {
			notice = "Delivery retry queued. Refresh to check its outcome."
		}

		pages.render(r, w, http.StatusOK, pageData{
			DeliveryHealth:    health,
			HealthUnavailable: healthErr != nil,
			Notice:            notice,
			Title:             "Deliveries",
			Page:              "deliveries",
			User:              currentUser(r).Name,
			DeliveryList:      list,
			Overview:          overviewLinks(pages.prefix, "/deliveries", page, list.NextPage, values, 0),
		})
	}
}
