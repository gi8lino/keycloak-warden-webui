package endpoint

import (
	"errors"
	"net/http"

	"github.com/gi8lino/warden-webui/internal/db"
	"github.com/gi8lino/warden-webui/internal/http/auth"
)

// Users serves the local administrator directory and its account management forms.
func Users(identity *auth.Service, pages *Pages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := currentUser(r)
		if actor.Name == "" {
			NotFound(pages)(w, r)
			return
		}
		if r.Method == http.MethodPost {
			if err := parseForm(w, r); err != nil {
				pages.Problem(r, w, http.StatusBadRequest, "Invalid user form.")
				return
			}
			err := updateUser(identity, r, actor)
			if err == nil {
				pages.redirect(w, r, "/admin/users")
				return
			}
			switch {
			case errors.Is(err, db.ErrIdentityLinked):
				pages.Problem(r, w, http.StatusConflict, "Identity already linked or account unavailable.")
			case errors.Is(err, db.ErrUsernameTaken):
				pages.Problem(r, w, http.StatusConflict, "Username already exists.")
			case errors.Is(err, db.ErrNotFound):
				pages.Problem(r, w, http.StatusConflict, "This account cannot be changed.")
			case errors.Is(err, auth.ErrInvalidLocalInput):
				pages.Problem(r, w, http.StatusBadRequest, "Check the username and password requirements.")
			default:
				pages.failure(r, w, err, "Unable to update user.")
			}
			return
		}
		users, err := identity.LocalUsers(r.Context(), actor)
		if err != nil {
			pages.failure(r, w, err, "Unable to list users.")
			return
		}
		links, err := identity.IdentityLinks(r.Context())
		if err != nil {
			pages.failure(r, w, err, "Unable to list mappings.")
			return
		}
		pending, err := identity.PendingIdentities(r.Context())
		if err != nil {
			pages.failure(r, w, err, "Unable to list pending identities.")
			return
		}
		pages.render(r, w, http.StatusOK, pageData{Title: "User administration", Page: "users", User: actor.Name, Users: users, Links: links, Pending: pending})
	}
}

// updateUser applies an explicit account operation from the submitted form.
func updateUser(identity *auth.Service, r *http.Request, actor auth.User) error {
	name := r.PostForm.Get("username")
	switch r.PostForm.Get("operation") {
	case "link", "unlink":
		return identity.UpdateIdentity(r.Context(), actor, name, r.PostForm.Get("subject"), r.PostForm.Get("operation") == "link")
	case "create":
		return identity.CreateLocalUser(r.Context(), actor, name, r.PostForm.Get("password"))
	case "enable":
		return identity.SetLocalAccess(r.Context(), actor, name, true)
	case "disable":
		return identity.SetLocalAccess(r.Context(), actor, name, false)
	default:
		return auth.ErrInvalidLocalInput
	}
}
