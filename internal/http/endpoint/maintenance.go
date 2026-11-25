package endpoint

import (
	"context"
	"errors"
	"log/slog"
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

		workerView, workerMessage := workerStatus(r.Context(), api, pages.logger)
		notice := maintenanceNotice(r.URL.Query().Get("done"))

		pages.render(r, w, http.StatusOK, pageData{
			Title:              "Database maintenance",
			Page:               "maintenance",
			User:               user.Name,
			Maintenance:        status,
			Workers:            workerView,
			WorkersUnavailable: workerMessage,
			Notice:             notice,
		})
	}
}

// workerStatus retrieves the worker snapshot and handles unavailable endpoints.
func workerStatus(ctx context.Context, api API, logger *slog.Logger) (*client.Workers, string) {
	status, err := api.WorkerStatus(ctx)

	switch {
	case err == nil:
		return &status, ""
	case isWorkerStatusUnsupported(err):
		return nil, "Worker status requires a newer Warden release."
	default:
		logger.Warn("worker status unavailable", "event", "worker_status_unavailable", "error", err)
		return nil, "Worker status is temporarily unavailable."
	}
}

// maintenanceNotice returns the success message for a completed maintenance operation.
func maintenanceNotice(operation string) string {
	switch operation {
	case "prune":
		return "Observation cleanup completed."
	case "optimize":
		return "Database optimization completed."
	default:
		return ""
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
	upstream, ok := errors.AsType[*client.Error](err)
	return ok && upstream.Code == http.StatusNotFound
}
