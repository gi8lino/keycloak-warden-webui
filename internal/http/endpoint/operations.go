package endpoint

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gi8lino/keycloak-warden/pkg/model"
)

// RetryDelivery validates a browser form and sends the verified account identity to Warden.
func RetryDelivery(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := parseForm(w, r); err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid retry form.")
			return
		}

		request, err := parseRetryRequest(r)
		if err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid delivery retry.")
			return
		}

		err = pages.auditAction(
			r,
			actionRetry,
			func() error {
				return api.RetryDelivery(r.Context(), request)
			},
			"decision_id", request.DecisionID,
			"revision", request.Revision,
			"destination", request.Destination,
		)
		if err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}

		pages.redirect(w, r, "/deliveries?notice=retry-queued")
	}
}

// parseRetryRequest parses and validates a delivery retry form.
func parseRetryRequest(r *http.Request) (model.RetryRequest, error) {
	id := r.PathValue("id")
	if !validID(id) {
		return model.RetryRequest{}, errors.New("invalid decision ID")
	}

	if len(r.PostForm["revision"]) != 1 || len(r.PostForm["destination"]) != 1 {
		return model.RetryRequest{}, errors.New("expected exactly one revision and destination")
	}

	revision, err := strconv.ParseInt(r.PostForm.Get("revision"), 10, 64)
	if err != nil || revision < 1 {
		return model.RetryRequest{}, errors.New("invalid revision")
	}

	destination := r.PostForm.Get("destination")
	if !validDestination(destination) {
		return model.RetryRequest{}, errors.New("invalid destination")
	}

	return model.RetryRequest{
		DecisionID:  id,
		Revision:    revision,
		Destination: destination,
		Actor:       currentUser(r).Name,
	}, nil
}

// validDestination reports whether a delivery destination is valid.
func validDestination(destination string) bool {
	return destination != "" &&
		len(destination) <= 128 &&
		strings.TrimSpace(destination) == destination &&
		!strings.ContainsAny(destination, "\x00\r\n\t")
}

// DecisionTimeline renders chronological facts without joining unrelated pages in the browser.
func DecisionTimeline(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			NotFound(pages)(w, r)
			return
		}
		values, page, err := parseOverviewQuery(r.URL.RawQuery)
		if err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid timeline page.")
			return
		}
		result, err := api.DecisionTimeline(r.Context(), id, page)
		if err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}
		pages.render(r, w, http.StatusOK, pageData{
			Title:    "Decision timeline",
			Page:     "timeline",
			User:     currentUser(r).Name,
			Timeline: result,
			Overview: overviewLinks(pages.prefix, "/bans/"+url.PathEscape(id)+"/timeline", page, result.NextPage, values, 0),
		})
	}
}
