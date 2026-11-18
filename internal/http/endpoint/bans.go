package endpoint

import (
	"net/http"
	"strings"
	"time"

	"github.com/gi8lino/keycloak-warden/pkg/client"
)

// Dashboard displays the currently effective bans and operator actions.
func Dashboard(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)

		bans, err := api.List(r.Context())
		if err != nil {
			pages.fail(r, w, user, err)
			return
		}

		notices := map[string]string{
			"ban-created": "Manual ban recorded.",
			"released":    "Ban sources released.",
		}

		pages.render(r, w, http.StatusOK, pageData{
			Title:  "Active bans",
			Page:   "dashboard",
			User:   user.Name,
			Bans:   bans,
			Notice: notices[r.URL.Query().Get("notice")],
		})
	}
}

// BanDetails displays the decision, its sources, and destination acknowledgements.
func BanDetails(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			NotFound(pages)(w, r)
			return
		}

		user := currentUser(r)

		record, err := api.Get(r.Context(), id)
		if err != nil {
			pages.fail(r, w, user, err)
			return
		}

		pages.render(r, w, http.StatusOK, pageData{
			Title: "Decision details",
			Page:  "detail",
			User:  user.Name,
			Ban:   record,
		})
	}
}

// CreateBan validates the browser form and creates a manual ban through Warden.
func CreateBan(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := parseForm(w, r); err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid ban form.")
			return
		}

		ip := strings.TrimSpace(r.FormValue("ip"))
		duration := strings.TrimSpace(r.FormValue("duration"))
		reason := strings.TrimSpace(r.FormValue("reason"))
		if !validBanFields(ip, duration, reason) {
			pages.Problem(r, w, http.StatusBadRequest, "IP, duration, and reason are required.")
			return
		}

		parsedDuration, err := time.ParseDuration(duration)
		if err != nil || parsedDuration <= 0 {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid ban duration.")
			return
		}

		if _, err := api.Ban(r.Context(), client.ManualBanRequest{
			IP:       ip,
			Duration: parsedDuration,
			Reason:   reason,
		}); err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}
		pages.redirect(w, r, "/?notice=ban-created")
	}
}

// ReleaseBan releases all active sources of an existing Warden decision.
func ReleaseBan(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			NotFound(pages)(w, r)
			return
		}

		if _, err := api.Unban(r.Context(), id); err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}

		pages.redirect(w, r, "/?notice=released")
	}
}

// validBanFields requires bounded, nonempty values before parsing or contacting Warden.
func validBanFields(ip, duration, reason string) bool {
	return ip != "" && len(ip) <= 64 &&
		duration != "" && len(duration) <= 32 &&
		reason != "" && len(reason) <= 512
}
