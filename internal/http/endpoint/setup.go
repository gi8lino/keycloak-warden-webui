package endpoint

import (
	"errors"
	"net/http"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/gi8lino/warden-webui/internal/db"
	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// SetupPage presents first-run administrator provisioning only while setup is incomplete.
func SetupPage(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return whileSetupRequired(identity, pages, func(w http.ResponseWriter, r *http.Request) {
		pages.render(r, w, http.StatusOK, pageData{
			Title: "Create administrator",
			Page:  "setup",
		})
	})
}

// SetupLocal verifies the bootstrap token and atomically provisions the local administrator.
func SetupLocal(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return whileSetupRequired(identity, pages, func(w http.ResponseWriter, r *http.Request) {
		if err := parseForm(w, r); err != nil {
			pages.Problem(r, w, http.StatusBadRequest, "Invalid setup form.")
			return
		}

		password := r.FormValue("password")
		if password != r.FormValue("confirmPassword") {
			pages.render(r, w, http.StatusBadRequest, pageData{
				Title: "Create administrator",
				Page:  "setup",
				Error: "Passwords do not match.",
			})
			return
		}

		err := identity.SetupLocal(w, r, r.FormValue("setupToken"), r.FormValue("username"), password)
		if err == nil {
			redirectAfterAuthForm(w, r, identity, pages, "/")
			return
		}
		if errors.Is(err, db.ErrSetupComplete) {
			NotFound(pages)(w, r)
			return
		}
		if errors.Is(err, appkitauth.ErrForbidden) {
			pages.Problem(r, w, http.StatusForbidden, "Invalid setup authorization.")
			return
		}
		if errors.Is(err, auth.ErrInvalidLocalInput) {
			pages.render(r, w, http.StatusBadRequest, pageData{
				Title: "Create administrator",
				Page:  "setup",
				Error: "Enter a username and a password of 12–72 bytes.",
			})
			return
		}

		pages.failure(r, w, err, "Setup unavailable.")
	})
}

// whileSetupRequired exposes a setup handler only until the first administrator exists.
func whileSetupRequired(identity *auth.Service, pages *Pages, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !identity.SetupRequired() {
			NotFound(pages)(w, r)
			return
		}

		next(w, r)
	}
}
