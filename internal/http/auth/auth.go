// Package auth binds Appkit browser authentication to Warden Web UI administrators.
package auth

import (
	"net/http"

	appkithttpauth "github.com/containeroo/appkit/httpauth"
)

// CurrentUser returns the authenticated administrator attached to the request.
func CurrentUser(r *http.Request) (User, bool) {
	return appkithttpauth.CurrentUser[User](r)
}

// WithUser attaches the authenticated administrator to the request context.
func WithUser(r *http.Request, user User) *http.Request {
	return appkithttpauth.WithUser(r, user)
}
