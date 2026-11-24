package endpoint

import "net/http"

// Maintenance displays recent Warden observation cleanup and optimization status.
func Maintenance(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)

		status, err := api.MaintenanceStatus(r.Context())
		if err != nil {
			pages.fail(r, w, user, err)
			return
		}

		notices := map[string]string{
			"prune":    "Observation cleanup completed.",
			"optimize": "Database optimization completed.",
		}

		pages.render(r, w, http.StatusOK, pageData{
			Title:       "Database maintenance",
			Page:        "maintenance",
			User:        user.Name,
			Maintenance: status,
			Notice:      notices[r.URL.Query().Get("done")],
		})
	}
}

// RunMaintenance triggers one of Warden's two supported housekeeping operations.
func RunMaintenance(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		operation := r.PathValue("operation")
		var action administrativeAction
		var run func() error
		switch operation {
		case "prune":
			action = actionPrune
			run = func() error {
				_, err := api.PruneObservations(r.Context())
				return err
			}
		case "optimize":
			action = actionOptimize
			run = func() error {
				_, err := api.OptimizeDatabase(r.Context())
				return err
			}
		default:
			NotFound(pages)(w, r)
			return
		}

		err := pages.auditAction(r, action, run)
		if err != nil {
			pages.fail(r, w, currentUser(r), err)
			return
		}

		pages.redirect(w, r, "/maintenance?done="+operation)
	}
}
