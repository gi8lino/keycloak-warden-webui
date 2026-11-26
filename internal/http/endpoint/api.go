package endpoint

import (
	"context"

	"github.com/gi8lino/keycloak-warden/pkg/client"
)

// API describes the Warden administration operations required by browser endpoints.
// The concrete client and bearer token remain server-side.
type API interface {
	List(context.Context) ([]client.Record, error)
	ListDecisionHistory(context.Context, client.RevisionFilter) (client.RevisionPage, error)
	ListDeliveries(context.Context, client.DeliveryFilter) (client.DeliveryOverviewPage, error)
	// DeliveryHealth reports aggregate current destination progress.
	DeliveryHealth(context.Context) ([]client.DestinationHealth, error)
	// DecisionTimeline retrieves a bounded chronological audit page.
	DecisionTimeline(context.Context, string, int) (client.TimelinePage, error)
	// RetryDelivery queues one exhausted revision using the authenticated operator identity.
	RetryDelivery(context.Context, client.RetryRequest) error
	// ListObservations reads hypothetical ban outcomes.
	ListObservations(context.Context, int) (client.ObservationPage, error)
	Get(context.Context, string) (client.Record, error)
	// GetPageThroughRevision retrieves a bounded delivery-history page.
	GetPageThroughRevision(context.Context, string, int, int64) (client.Record, error)
	Ban(context.Context, client.ManualBanRequest) (client.Record, error)
	Unban(context.Context, string) (client.Record, error)
	// WorkerStatus reads both Warden worker snapshots and persistent delivery statistics.
	WorkerStatus(context.Context) (client.Workers, error)
	MaintenanceStatus(context.Context) (client.Maintenance, error)
	PruneObservations(context.Context) (client.Maintenance, error)
	OptimizeDatabase(context.Context) (client.Maintenance, error)
}
