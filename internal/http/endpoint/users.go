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
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			if err := parseForm(w, r); err != nil {
				http.Error(w, "Invalid user form.", http.StatusBadRequest)
				return
			}
			err := updateUser(identity, r, actor)
			if err == nil {
				pages.redirect(w, r, "/admin/users")
				return
			}
			switch {
			case errors.Is(err, db.ErrIdentityLinked):
				http.Error(w, "Identity already linked or account unavailable.", http.StatusConflict)
			case errors.Is(err, db.ErrUsernameTaken):
				http.Error(w, "Username already exists.", http.StatusConflict)
			case errors.Is(err, db.ErrNotFound):
				http.Error(w, "This account cannot be changed.", http.StatusConflict)
			case errors.Is(err, auth.ErrInvalidLocalInput):
				http.Error(w, "Check the username and password requirements.", http.StatusBadRequest)
			default:
				http.Error(w, "Unable to update user.", http.StatusInternalServerError)
			}
			return
		}
		users, err := identity.LocalUsers(r.Context(), actor)
		if err != nil {
			http.Error(w, "Unable to list users.", http.StatusInternalServerError)
			return
		}
		links, err := identity.IdentityLinks(r.Context())
		if err != nil {
			http.Error(w, "Unable to list mappings.", http.StatusInternalServerError)
			return
		}
		pending, err := identity.PendingIdentities(r.Context())
		if err != nil {
			http.Error(w, "Unable to list pending identities.", http.StatusInternalServerError)
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
