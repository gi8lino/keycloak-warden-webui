package endpoint

import (
	"errors"
	"net/http"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// LoginPage serves the local sign-in form or begins an Appkit OIDC login.
func LoginPage(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if identity.SetupRequired() {
			pages.redirect(w, r, "/setup")
			return
		}

		switch identity.Mode() {
		case appkitauth.ModeOIDC:
			if r.URL.Query().Get("continue") == "1" {
				pages.render(r, w, http.StatusOK, pageData{
					Title: "Continue to Warden",
					Page:  "continue",
				})
				return
			}
			if err := identity.BeginOIDC(w, r); err != nil {
				pages.Problem(r, w, http.StatusServiceUnavailable, "OIDC login unavailable.")
			}
		case appkitauth.ModeLocal:
			pages.render(r, w, http.StatusOK, pageData{
				Title:  "Sign in",
				Page:   "login",
				Notice: loginNotice(r),
			})
		default:
			pages.Problem(r, w, http.StatusForbidden, "Sign in through the trusted identity proxy.")
		}
	}
}

// LocalLoginPage exposes the password form only when local sign-in is enabled.
func LocalLoginPage(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !identity.LocalLoginEnabled() {
			NotFound(pages)(w, r)
			return
		}
		pages.render(r, w, http.StatusOK, pageData{
			Title:  "Local sign in",
			Page:   "login",
			Notice: loginNotice(r),
		})
	}
}

// LoginLocal authenticates a bounded browser form and creates an Appkit session.
func LoginLocal(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if identity.SetupRequired() {
			NotFound(pages)(w, r)
			return
		}
		if !identity.LocalLoginEnabled() {
			NotFound(pages)(w, r)
			return
		}
		if err := parseForm(w, r); err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid login form.")
			return
		}

		u := r.FormValue("username")
		pw := r.FormValue("password")
		if err := identity.SignInLocal(w, r, u, pw); err != nil {
			if !isRejectedLocalLogin(err) {
				pages.failure(r, w, err, "Authentication unavailable.")
				return
			}
			pages.render(r, w, http.StatusUnauthorized, pageData{
				Title: "Sign in",
				Page:  "login",
				Error: "Invalid username or password.",
			})
			return
		}

		pages.redirect(w, r, "/")
	}
}

// OIDCCallback completes the verified identity flow before issuing a browser session.
func OIDCCallback(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := identity.CompleteOIDC(w, r); err != nil {
			pages.Problem(r, w, http.StatusForbidden, "OIDC authentication failed or administrator group missing.")
			return
		}
		pages.redirect(w, r, "/")
	}
}

// Logout revokes the current browser session.
func Logout(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if identity.Mode() == appkitauth.ModeProxy && !currentUser(r).Local {
			pages.Problem(r, w, http.StatusBadRequest, "Sign out through your identity provider.")
			return
		}
		if err := identity.Logout(w, r); err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Sign out through your identity provider.")
			return
		}
		redirectAfterAuthForm(w, r, identity, pages, "/login")
	}
}

// loginNotice only displays server-controlled confirmation text.
func loginNotice(r *http.Request) string {
	if r.URL.Query().Get("changed") == "1" {
		return "Password changed. Sign in again."
	}
	return ""
}

// isRejectedLocalLogin distinguishes credential rejection from an authentication outage.
func isRejectedLocalLogin(err error) bool {
	return errors.Is(err, appkitauth.ErrInvalidCredentials) || errors.Is(err, appkitauth.ErrForbidden)
}

// redirectAfterAuthForm ends OIDC form submissions on this origin before navigating to the provider.
func redirectAfterAuthForm(w http.ResponseWriter, r *http.Request, identity *auth.Service, pages *Pages, destination string) {
	if identity.Mode() == appkitauth.ModeOIDC {
		destination = "/login?continue=1"
	}
	pages.redirect(w, r, destination)
}
