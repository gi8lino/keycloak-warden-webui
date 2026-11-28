package response

import (
	"log/slog"
	"net/http"

	httpkitresponse "github.com/containeroo/httpkit/response"
)

// RequestWriter is HTTPkit's request diagnostics writer.
type RequestWriter = httpkitresponse.RequestWriter

// NewRequestWriter wraps the response writer with status and request metadata.
func NewRequestWriter(w http.ResponseWriter, r *http.Request) *RequestWriter {
	return httpkitresponse.NewRequestWriter(w, r)
}

// InternalServerError logs a failure and writes a safe HTTP 500 response with a reference ID.
// The application retains its structured JSON problem contract.
func InternalServerError(logger *slog.Logger, w http.ResponseWriter, err error) {
	reference := httpkitresponse.LogFailure(logger, w, err)

	if request, ok := w.(*httpkitresponse.RequestWriter); ok && request.Status != 0 {
		return
	}

	Problem(w, http.StatusInternalServerError, "The request could not be processed. Reference: "+reference)
}
