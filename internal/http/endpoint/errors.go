package endpoint

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/gi8lino/keycloak-warden/pkg/client"
	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// Problem renders a server-controlled error message with the normal application shell.
func (p *Pages) Problem(r *http.Request, w http.ResponseWriter, status int, message string) {
	p.render(r, w, status, pageData{
		Title: http.StatusText(status),
		Page:  "error",
		User:  currentUser(r).Name,
		Error: message,
	})
}

// fail logs backend diagnostics and shows actionable messages without disclosing response bodies.
func (p *Pages) fail(r *http.Request, w http.ResponseWriter, user auth.User, err error) {
	p.logger.Error("Warden API request failed", "event", "warden_api_failed", "error", err)
	status := upstreamErrorStatus(err)
	p.render(r, w, status, pageData{
		Title: "Request failed",
		Page:  "error",
		User:  user.Name,
		Error: upstreamErrorMessage(status),
	})
}

// upstreamErrorStatus preserves backend HTTP failures and distinguishes timeouts from connection failures.
func upstreamErrorStatus(err error) int {
	upstream, ok := errors.AsType[*client.Error](err)
	if ok && upstream.Code >= 400 && upstream.Code <= 599 {
		return upstream.Code
	}
	if backendTimedOut(err) {
		return http.StatusGatewayTimeout
	}

	return http.StatusBadGateway
}

// upstreamErrorMessage maps backend statuses to safe guidance owned by the Web UI.
func upstreamErrorMessage(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "Warden could not accept these values. Check your input and try again."
	case http.StatusUnauthorized, http.StatusForbidden:
		return "Warden denied access to the Web UI. Ask an administrator to check the configured Warden API credentials."
	case http.StatusNotFound:
		return "The requested record was not found."
	case http.StatusConflict:
		return "The request conflicts with Warden's current state. Review the latest data before trying again."
	case http.StatusTooManyRequests:
		return "Warden is handling too many requests. Wait a moment before trying again."
	case http.StatusGatewayTimeout:
		return "Warden took too long to respond. Check the current state before repeating an action; it may already have completed."
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return "Warden is currently unavailable. Try opening the page again shortly. Before repeating an action, check whether it completed."
	default:
		return "Warden could not complete the request. Check the current state before repeating an action, and contact an administrator if the problem continues."
	}
}

// backendTimedOut recognizes context deadlines and timeout-capable transport failures.
func backendTimedOut(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	network, ok := errors.AsType[net.Error](err)
	return ok && network.Timeout()
}

// failure records a local backend failure and renders only the supplied safe message.
func (p *Pages) failure(r *http.Request, w http.ResponseWriter, err error, message string) {
	p.logger.Error("Web UI request failed", "event", "webui_request_failed", "error", err)
	p.Problem(r, w, http.StatusInternalServerError, message)
}
