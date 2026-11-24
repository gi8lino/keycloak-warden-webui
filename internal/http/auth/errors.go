package auth

import (
	"errors"

	"github.com/gi8lino/warden-webui/internal/db"
)

var (
	// ErrIdentityLinked indicates a requested identity belongs to another administrator.
	ErrIdentityLinked = errors.New("identity already linked or account unavailable")
	// ErrUsernameTaken indicates a local username cannot be registered again.
	ErrUsernameTaken = errors.New("username already registered")
	// ErrAccountUnavailable indicates the target administrator cannot be modified.
	ErrAccountUnavailable = errors.New("administrator account unavailable")
)

// accountOperationError translates persistence-specific conflicts at the auth boundary.
func accountOperationError(err error) error {
	switch {
	case errors.Is(err, db.ErrIdentityLinked):
		return ErrIdentityLinked
	case errors.Is(err, db.ErrUsernameTaken):
		return ErrUsernameTaken
	case errors.Is(err, db.ErrNotFound):
		return ErrAccountUnavailable
	default:
		return err
	}
}
