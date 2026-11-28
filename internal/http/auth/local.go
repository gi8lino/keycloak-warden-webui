package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/local"
	"github.com/containeroo/appkit/auth/session"

	"github.com/gi8lino/warden-webui/internal/accounts"
	"github.com/gi8lino/warden-webui/internal/db"
)

// ErrInvalidLocalInput describes rejected initial setup or password-change input.
var ErrInvalidLocalInput = errors.New("invalid local administrator input")

const (
	// minPasswordLength is the minimum local administrator password length.
	minPasswordLength = 12
	// maxPasswordLength is bcrypt's maximum byte length.
	maxPasswordLength = 72
	// maxUsernameLength bounds a local username.
	maxUsernameLength = 64
)

// localCredentials fetches the administrator's persisted bcrypt hash.
type localCredentials struct {
	// repository reads local credentials from SQLite.
	repository *db.Store
}

// LookupCredential implements Appkit's local credential-store contract.
func (s localCredentials) LookupCredential(ctx context.Context, username string) (user User, passwordHash string, err error) {
	hash, err := s.repository.LookupCredential(ctx, username)
	if errors.Is(err, db.ErrNotFound) {
		return User{}, "", appkitauth.ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}

	return User{Name: username, Local: true}, hash, nil
}

// newLocal connects Appkit's local credential verifier to persisted credentials.
func newLocal(repository *db.Store, manager session.Manager[User]) local.Service[User] {
	return local.Service[User]{
		Credentials: localCredentials{repository: repository},
		Sessions:    manager,
	}
}

// validLocalPassword enforces the local credential policy before bcrypt hashing.
func validLocalPassword(password string) bool {
	return len(password) >= minPasswordLength &&
		len(password) <= maxPasswordLength &&
		strings.TrimSpace(password) != ""
}

// SetupLocal atomically provisions the initial administrator and signs them in.
func (s *Service) SetupLocal(w http.ResponseWriter, r *http.Request, token, username, password string) error {
	if !s.SetupRequired() {
		return db.ErrSetupComplete
	}

	username = strings.TrimSpace(username)
	var link *accounts.IdentityLink
	if s.mode != appkitauth.ModeLocal && s.setupSubject != "" {
		link = &accounts.IdentityLink{
			Issuer:   s.issuer,
			Subject:  s.setupSubject,
			Username: username,
		}
	}

	setup := local.Setup[User]{
		TokenDigest: &s.setupDigest, Validate: validateSetupCredentials,
		Create: func(ctx context.Context, name, hash string) (User, error) {
			return User{
				Name:  name,
				Local: true,
			}, s.repository.CreateFounder(ctx, name, hash, link)
		},
	}
	user, err := setup.Bootstrap(r.Context(), token, username, password)
	if errors.Is(err, db.ErrSetupComplete) {
		// Another concurrent setup already committed the initial administrator.
		s.setupRequired.Store(false)
	}

	if err != nil {
		return err
	}

	// Disable setup as soon as the database commit succeeds, even if the
	// subsequent session creation fails.
	s.setupRequired.Store(false)

	if link != nil {
		return nil
	}
	// A setup commit cannot be rolled back if issuing a session fails; normal login remains available.
	sessionToken, err := s.sessions.Start(r.Context(), user)
	if err != nil {
		return err
	}

	http.SetCookie(w, s.cookies.Cookie(sessionToken, false))

	return nil
}

// SignInLocal verifies persisted bcrypt credentials and issues a browser session.
func (s *Service) SignInLocal(w http.ResponseWriter, r *http.Request, username, password string) error {
	if !s.LocalLoginEnabled() {
		return appkitauth.ErrForbidden
	}

	_, sessionToken, err := s.local.SignIn(r.Context(), username, password)
	if err != nil {
		return err
	}

	http.SetCookie(w, s.cookies.Cookie(sessionToken, false))

	return nil
}

// CanChangeLocalPassword allows password management for locally authenticated users while password sign-in is enabled.
func (s *Service) CanChangeLocalPassword(user User) bool {
	return s.LocalLoginEnabled() && user.Local
}

// ChangeLocalPassword verifies the current password and revokes the user's local sessions.
func (s *Service) ChangeLocalPassword(ctx context.Context, user User, oldPassword, newPassword string) error {
	if !s.CanChangeLocalPassword(user) {
		return appkitauth.ErrForbidden
	}
	if !validLocalPassword(newPassword) {
		return ErrInvalidLocalInput
	}

	hash, err := s.repository.LookupCredential(ctx, user.Name)
	if errors.Is(err, db.ErrNotFound) {
		return appkitauth.ErrInvalidCredentials
	}
	if err != nil {
		return err
	}
	if err := local.VerifyPassword(hash, oldPassword); err != nil {
		return appkitauth.ErrInvalidCredentials
	}

	newHash, err := local.HashPassword(newPassword)
	if err != nil {
		return err
	}

	err = s.repository.ChangePassword(ctx, user.Name, hash, newHash)
	if errors.Is(err, db.ErrNotFound) {
		return appkitauth.ErrInvalidCredentials
	}

	return err
}

// LocalUsers returns the directory for an authenticated administrator.
func (s *Service) LocalUsers(ctx context.Context, actor User) ([]accounts.LocalUser, error) {
	if actor.Name == "" {
		return nil, appkitauth.ErrForbidden
	}

	return s.repository.LocalUsers(ctx)
}

// CreateLocalUser validates and hashes the new administrator's initial password.
func (s *Service) CreateLocalUser(ctx context.Context, actor User, name, password string) error {
	if actor.Name == "" {
		return appkitauth.ErrForbidden
	}

	name = strings.TrimSpace(name)
	if !validLocalCredentials(name, password) {
		return ErrInvalidLocalInput
	}

	hash, err := local.HashPassword(password)
	if err != nil {
		return err
	}

	return accountOperationError(s.repository.CreateLocalUser(ctx, name, hash))
}

// SetLocalAccess enables or disables another local administrator.
func (s *Service) SetLocalAccess(ctx context.Context, actor User, name string, enabled bool) error {
	if actor.Name == "" {
		return appkitauth.ErrForbidden
	}

	return accountOperationError(s.repository.SetLocalAccess(ctx, actor.Name, name, enabled))
}

// validLocalCredentials checks the username and password policy for new accounts.
func validLocalCredentials(name, password string) bool {
	return name != "" && len(name) <= maxUsernameLength && validLocalPassword(password)
}

// validateSetupCredentials applies the local administrator credential limits.
func validateSetupCredentials(name, password string) error {
	if !validLocalCredentials(name, password) {
		return ErrInvalidLocalInput
	}

	return nil
}
