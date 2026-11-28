package auth

import (
	"context"
	"errors"
	"strings"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/provisioning"

	"github.com/gi8lino/warden-webui/internal/accounts"
	"github.com/gi8lino/warden-webui/internal/db"
)

// resolveExternal binds the verified subject to an enabled local administrator.
func (s *Service) resolveExternal(ctx context.Context, issuer, subject string) (User, error) {
	accounts := provisioning.Accounts[User]{
		Resolve:         s.lookupAdministrator,
		IsAdministrator: func(u User) bool { return u.Name != "" },
	}

	return accounts.Login(
		ctx,
		provisioning.AccessPolicy{
			RequireAdministrator: true,
		},
		provisioning.Identity{
			Issuer:   issuer,
			Subject:  subject,
			Username: subject,
		})
}

// lookupAdministrator resolves explicit links from the local administrator directory.
func (s *Service) lookupAdministrator(ctx context.Context, id provisioning.Identity, _ bool) (User, error) {
	issuer, subject := id.Issuer, id.Subject
	name, err := s.repository.ResolveIdentity(ctx, issuer, subject)
	if errors.Is(err, db.ErrNotFound) {
		if err := s.repository.RecordPendingIdentity(ctx, issuer, subject); err != nil {
			return User{}, err
		}

		return User{}, appkitauth.ErrForbidden
	}
	if err != nil {
		return User{}, err
	}

	return User{Name: name, Issuer: issuer, Subject: subject}, nil
}

// IdentityLinks lists explicit mappings for the authenticated administration page.
func (s *Service) IdentityLinks(ctx context.Context) ([]accounts.IdentityLink, error) {
	if s.issuer == "" {
		return nil, nil
	}

	return s.repository.IdentityLinks(ctx, s.issuer)
}

// PendingIdentities lists verified login attempts awaiting approval for the active issuer.
func (s *Service) PendingIdentities(ctx context.Context) ([]string, error) {
	if s.issuer == "" {
		return nil, nil
	}

	return s.repository.PendingIdentities(ctx, s.issuer)
}

// UpdateIdentity assigns or removes an external login from a local administrator.
func (s *Service) UpdateIdentity(ctx context.Context, actor User, name, subject string, add bool) error {
	if !s.canUpdateIdentity(actor, subject) {
		return ErrInvalidLocalInput
	}

	link := accounts.IdentityLink{Username: name, Issuer: s.issuer, Subject: subject}
	if add {
		return accountOperationError(s.repository.ApproveIdentity(ctx, link))
	}
	if actor.Issuer == s.issuer && actor.Subject == subject {
		return ErrAccountUnavailable
	}

	return accountOperationError(s.repository.UnlinkIdentity(ctx, link))
}

// canUpdateIdentity requires an authenticated account and a provider-scoped subject.
func (s *Service) canUpdateIdentity(actor User, subject string) bool {
	return actor.Name != "" && s.issuer != "" && strings.TrimSpace(subject) != ""
}
