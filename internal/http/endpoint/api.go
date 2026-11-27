package endpoint

import (
	"context"

	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/keycloak-warden/pkg/model"
)

// API describes the Warden administration operations required by browser endpoints.
type API interface {
	List(context.Context) ([]model.Record, error)
	ListDecisionHistory(context.Context, client.RevisionFilter) (model.RevisionPage, error)
	ListDeliveries(context.Context, client.DeliveryFilter) (model.DeliveryOverviewPage, error)
	// DeliveryHealth reports aggregate current destination progress.
	DeliveryHealth(context.Context) ([]model.DestinationHealth, error)
	// DecisionTimeline retrieves a bounded chronological audit page.
	DecisionTimeline(context.Context, string, int) (model.TimelinePage, error)
	// RetryDelivery queues one exhausted revision using the authenticated operator identity.
	RetryDelivery(context.Context, model.RetryRequest) error
	// ListIncidents reads named detection-rule threshold hits.
	ListIncidents(context.Context, client.IncidentFilter) (model.IncidentPage, error)
	Get(context.Context, string) (model.Record, error)
	// GetPageThroughRevision retrieves a bounded delivery-history page.
	GetPageThroughRevision(context.Context, string, int, int64) (model.Record, error)
	Ban(context.Context, client.ManualBanRequest) (model.Record, error)
	Unban(context.Context, string) (model.Record, error)
	// WorkerStatus reads both Warden worker snapshots and persistent delivery statistics.
	WorkerStatus(context.Context) (model.Workers, error)
	MaintenanceStatus(context.Context) (model.Maintenance, error)
	PruneObservations(context.Context) (model.Maintenance, error)
	OptimizeDatabase(context.Context) (model.Maintenance, error)
}
