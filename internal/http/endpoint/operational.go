package endpoint

import (
	"context"
	"net/http"
)

// Health returns process liveness without contacting the upstream Warden API.
func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// Ready verifies that the private Web UI credential store remains responsive.
func Ready(check func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if check != nil && check(r.Context()) != nil {
			http.Error(w, "Web UI database unavailable.", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
