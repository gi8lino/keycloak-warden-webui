package server

import (
	"net/http"

	httpkitmiddleware "github.com/containeroo/httpkit/middleware"
	"github.com/gi8lino/warden-webui/internal/http/endpoint"
	"github.com/gi8lino/warden-webui/internal/http/middleware"
	"github.com/gi8lino/warden-webui/internal/http/response"
)

// newApplication composes routes and standard HTTP middleware, as in Motus.
func newApplication(config Config) http.Handler {
	applicationMux := http.NewServeMux()
	registerPublicRoutes(applicationMux, config)
	registerAdminRoutes(applicationMux, config)
	applicationMux.HandleFunc("/", endpoint.NotFound(config.Pages))

	middlewares := []httpkitmiddleware.Middleware{
		middleware.SecurityHeaders(),
	}
	if config.AccessLog {
		middlewares = append(middlewares, httpkitmiddleware.AccessLogWithErrorReference(config.Logger))
	}
	middlewares = append(middlewares,
		middleware.RecoverPanics(config.Logger),
		middleware.RejectCrossSiteWrites(config.PublicURL),
	)

	application := httpkitmiddleware.Chain(applicationMux, middlewares...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		application.ServeHTTP(response.NewRequestWriter(w, r), r)
	})
}
