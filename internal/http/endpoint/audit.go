package endpoint

import (
	"net/http"
	"time"
)

// administrativeAction identifies an operator action recorded in the Web UI audit log.
type administrativeAction string

const (
	// actionBan records a manual ban request.
	actionBan administrativeAction = "ban"
	// actionUnban records a manual release request.
	actionUnban administrativeAction = "unban"
	// actionRetry records an explicit delivery retry.
	actionRetry administrativeAction = "delivery.retry"
	// actionNotificationRetry records an explicit incident notification retry.
	actionNotificationRetry administrativeAction = "notification.retry"
	// actionPrune records manual observation cleanup.
	actionPrune administrativeAction = "maintenance.prune"
	// actionOptimize records manual database optimization.
	actionOptimize administrativeAction = "maintenance.optimize"
)

// auditAction logs an operator request and its outcome, returning the action error unchanged.
func (p *Pages) auditAction(r *http.Request, operation administrativeAction, run func() error, attributes ...any) error {
	started := time.Now()
	logger := p.logger.With(
		"actor", currentUser(r).Name,
		"operation", operation,
	).With(attributes...)

	logger.Info("administrative action requested",
		"event", "admin_action_requested",
	)

	err := run()
	elapsed := float64(time.Since(started)) / float64(time.Millisecond)
	if err != nil {
		// Failure describes the API request; a timeout cannot prove the action was rolled back.
		logger.Warn("administrative action request failed",
			"event", "admin_action_failed",
			"outcome", "failed",
			"duration_ms", elapsed,
		)
		return err
	}

	logger.Info("administrative action completed",
		"event", "admin_action_completed",
		"outcome", "completed",
		"duration_ms", elapsed,
	)

	return nil
}
