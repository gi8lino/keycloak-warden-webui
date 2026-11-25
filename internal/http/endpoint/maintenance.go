package endpoint

import (
	"errors"
	"net/http"

	"github.com/gi8lino/keycloak-warden/pkg/client"
)

// Maintenance displays housekeeping controls and the optional worker status.
func Maintenance(api API, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)

		status, err := api.MaintenanceStatus(r.Context())
		if err != nil {
			pages.fail(r, w, user, err)
			return
		}

		var workerView *client.Workers
		workerMessage := ""
		snapshot, workerErr := api.WorkerStatus(r.Context())
		switch {
		case workerErr == nil:
			workerView = &snapshot
		case isWorkerStatusUnsupported(workerErr):
			workerMessage = "Worker status requires a newer Warden release."
		default:
			pages.logger.Warn("worker status unavailable", "event", "worker_status_unavailable", "error", workerErr)
			workerMessage = "Worker status is temporarily unavailable."
		}

		notices := map[string]string{
			"prune":    "Observation cleanup completed.",
			"optimize": "Database optimization completed.",
		}

		pages.render(r, w, http.StatusOK, pageData{
			Title:              "Database maintenance",
			Page:               "maintenance",
			User:               user.Name,
			Maintenance:        status,
			Workers:            workerView,
			WorkersUnavailable: workerMessage,
			Notice:             notices[r.URL.Query().Get("done")],
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

// isWorkerStatusUnsupported preserves maintenance support against older Warden releases.
func isWorkerStatusUnsupported(err error) bool {
	var upstream *client.Error
	return errors.As(err, &upstream) && upstream.Code == http.StatusNotFound
}
