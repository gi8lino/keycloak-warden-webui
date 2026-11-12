package endpoint

import (
	"errors"
	"net/http"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// PasswordPage serves the password change form for a local administrator.
func PasswordPage(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)
		if !identity.CanChangeLocalPassword(user) {
			http.NotFound(w, r)
			return
		}

		pages.render(r, w, http.StatusOK, pageData{
			Title:     "Preferences",
			Page:      "password",
			User:      user.Name,
			LocalUser: true,
		})
	}
}

// ChangePassword verifies the old credential and revokes the current user's sessions after rotation.
func ChangePassword(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)

		if !identity.CanChangeLocalPassword(user) {
			http.NotFound(w, r)
			return
		}

		if err := parseForm(w, r); err != nil {
			http.Error(w, "Invalid password form.", http.StatusBadRequest)
			return
		}

		current := r.FormValue("currentPassword")
		newPassword := r.FormValue("newPassword")
		if newPassword != r.FormValue("confirmPassword") {
			passwordError(r, w, pages, user, "New passwords do not match.")
			return
		}

		if err := identity.ChangeLocalPassword(r.Context(), user, current, newPassword); err != nil {
			if errors.Is(err, appkitauth.ErrInvalidCredentials) {
				passwordError(r, w, pages, user, "Current password is incorrect.")
				return
			}
			if errors.Is(err, auth.ErrInvalidLocalInput) {
				passwordError(r, w, pages, user, "New password must contain 12–72 bytes.")
				return
			}
			http.Error(w, "Unable to update the password.", http.StatusInternalServerError)
			return
		}

		// The transaction has revoked this user's sessions; clear the current browser cookie.
		if err := identity.Logout(w, r); err != nil {
			http.Error(w, "Unable to clear browser session.", http.StatusInternalServerError)
			return
		}

		pages.redirect(w, r, "/login?changed=1")
	}
}

// passwordError renders a validation failure without disclosing credentials.
func passwordError(r *http.Request, w http.ResponseWriter, pages *Pages, user auth.User, message string) {
	pages.render(r, w, http.StatusBadRequest, pageData{
		Title:     "Preferences",
		Page:      "password",
		User:      user.Name,
		LocalUser: true,
		Error:     message,
	})
}

// Preferences displays the signed-in user's account settings for the active authentication mode.
func Preferences(pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pages.render(r, w, http.StatusOK, pageData{Title: "Preferences", Page: "password", User: currentUser(r).Name})
	}
}
