package bootstrap

import (
	"context"
	"fmt"

	"github.com/gi8lino/warden-webui/internal/flags"
	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// Application contains the configured browser authentication service.
type Application struct {
	// identity owns local, OIDC, or trusted-proxy authentication.
	identity *auth.Service
}

// NewApplication composes authentication around the initialized private store.
func NewApplication(ctx context.Context, cfg flags.Config, infrastructure *Infrastructure) (*Application, error) {
	identity, err := auth.New(ctx, cfg, infrastructure.store)
	if err != nil {
		return nil, fmt.Errorf("configure browser authentication: %w", err)
	}
	return &Application{identity: identity}, nil
}
