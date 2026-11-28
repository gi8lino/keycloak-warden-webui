package server

import (
	"context"
	"net/http"

	"github.com/gi8lino/warden-webui/internal/http/endpoint"
)

// newManagement provides operational routes without the application prefix.
func newManagement(check func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", endpoint.Health())
	mux.Handle("GET /readyz", endpoint.Ready(check))

	return mux
}
