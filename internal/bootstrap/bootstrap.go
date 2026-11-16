// Package bootstrap constructs Warden Web UI's runtime dependencies and HTTP configuration.
package bootstrap

import (
	"context"
	"fmt"

	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/warden-webui/internal/db"
	"github.com/gi8lino/warden-webui/internal/flags"
)

// Infrastructure owns persistence and the server-side Warden API client.
type Infrastructure struct {
	// store persists the Web UI's administrator and sessions.
	store *db.Store
	// warden authenticates administrative requests to the upstream Warden API.
	warden *client.Client
}

// NewInfrastructure opens the private store and prepares the upstream client.
func NewInfrastructure(ctx context.Context, cfg flags.Config) (*Infrastructure, error) {
	store, err := db.Open(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("open Web UI database: %w", err)
	}

	// Use Warden's shared API client for both secure and explicitly allowed HTTP.
	options := []client.Option{}
	if cfg.AllowInsecureWardenHTTP {
		options = append(options, client.WithAllowInsecureHTTP())
	}
	warden, err := client.New(cfg.WardenURL, cfg.WardenToken, options...)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("configure Warden API client: %w", err)
	}

	return &Infrastructure{
		store:  store,
		warden: warden,
	}, nil
}

// Close releases the private store after HTTP listeners have stopped.
func (i *Infrastructure) Close() error {
	return i.store.Close()
}
