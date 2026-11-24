package endpoint

import (
	"context"

	"github.com/gi8lino/keycloak-warden/pkg/client"
)

// API describes the Warden administration operations required by browser endpoints.
// The concrete client and bearer token remain server-side.
type API interface {
	List(context.Context) ([]client.Record, error)
	Get(context.Context, string) (client.Record, error)
	// GetPageThroughRevision retrieves a bounded delivery-history page.
	GetPageThroughRevision(context.Context, string, int, int64) (client.Record, error)
	Ban(context.Context, client.ManualBanRequest) (client.Record, error)
	Unban(context.Context, string) (client.Record, error)
	MaintenanceStatus(context.Context) (client.Maintenance, error)
	PruneObservations(context.Context) (client.Maintenance, error)
	OptimizeDatabase(context.Context) (client.Maintenance, error)
}
