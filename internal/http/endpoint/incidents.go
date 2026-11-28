package endpoint

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/gi8lino/keycloak-warden/pkg/client"
)

// Incidents displays recorded threshold hits independently of ban decisions.
func Incidents(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		values, before, err := parseCursorQuery(r.URL.RawQuery, "before-id", "rule", "kind")
		if err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid incident filters.")
			return
		}
		rule, kind := values.Get("rule"), values.Get("kind")
		if !validIncidentRuleFilter(rule) || !validIncidentKindFilter(kind) {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid incident filters.")
			return
		}

		filter := client.IncidentFilter{
			BeforeID: before,
			Rule:     rule,
			Kind:     kind,
		}

		result, err := api.ListIncidents(r.Context(), filter)
		if err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}

		pages.render(r, w, http.StatusOK, pageData{
			Title:          "Incidents",
			Page:           "incidents",
			User:           currentUser(r).Name,
			Incidents:      result,
			IncidentFilter: filter,
			Cursor:         cursorLinks(pages.prefix, "/incidents", "before-id", result.NextBeforeID, values),
		})
	}
}

// validIncidentRuleFilter accepts an optional bounded rule name without whitespace or controls.
func validIncidentRuleFilter(rule string) bool {
	if len(rule) > 64 {
		return false
	}
	return !strings.ContainsFunc(rule, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	})
}

// validIncidentKindFilter accepts the event categories exposed by the incidents API.
func validIncidentKindFilter(kind string) bool {
	switch kind {
	case "", "user", "admin":
		return true
	default:
		return false
	}
}
