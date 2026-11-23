package db

import (
	"context"
	"database/sql"
	"errors"
)

// IdentityLink binds a verified provider subject to one local account.
type IdentityLink struct {
	// Issuer identifies the authority that verifies the subject.
	Issuer string
	// Subject is the immutable OIDC subject or trusted proxy username.
	Subject string
	// Username owns the mapping in the local directory.
	Username string
}

// ErrIdentityLinked means an administrator must unlink an identity before reassigning it.
var ErrIdentityLinked = errors.New("identity already linked")

// LinkIdentity inserts an explicit mapping without transferring existing ownership.
func (s *Store) LinkIdentity(ctx context.Context, link IdentityLink) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO identity_links (issuer, subject, username)
		SELECT ?, ?, username
		FROM local_users
		WHERE username = ?
		ON CONFLICT (issuer, subject) DO NOTHING
	`, link.Issuer, link.Subject, link.Username)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrIdentityLinked
	}

	return err
}

// UnlinkIdentity drops the mapping and invalidates sessions bound to it by foreign key.
func (s *Store) UnlinkIdentity(ctx context.Context, link IdentityLink) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM identity_links
		WHERE issuer = ?
		  AND subject = ?
		  AND username = ?
	`, link.Issuer, link.Subject, link.Username)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrNotFound
	}

	return err
}

// ResolveIdentity finds the enabled local administrator owning a verified identity.
func (s *Store) ResolveIdentity(ctx context.Context, issuer, subject string) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `
		SELECT username
		FROM identity_links
		JOIN local_users USING (username)
		WHERE issuer = ?
		  AND subject = ?
		  AND enabled = 1
	`, issuer, subject).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}

	return name, err
}

// IdentityLinks lists the explicit mappings displayed in the administration directory.
func (s *Store) IdentityLinks(ctx context.Context, issuer string) ([]IdentityLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT username, issuer, subject
		FROM identity_links
		WHERE issuer = ?
		ORDER BY username, subject
	`, issuer)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var links []IdentityLink
	for rows.Next() {
		var link IdentityLink
		if err := rows.Scan(&link.Username, &link.Issuer, &link.Subject); err != nil {
			return nil, err
		}
		links = append(links, link)
	}

	return links, rows.Err()
}

// RecordPendingIdentity records a verified subject once, excluding subjects already linked to any account.
func (s *Store) RecordPendingIdentity(ctx context.Context, issuer, subject string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pending_identities (issuer, subject)
		SELECT ?, ?
		WHERE NOT EXISTS (
		    SELECT 1
		    FROM identity_links
		    WHERE issuer = ?
		      AND subject = ?
		)
		ON CONFLICT DO NOTHING
	`, issuer, subject, issuer, subject)

	return err
}

// PendingIdentities lists unresolved sign-ins scoped to a single provider.
func (s *Store) PendingIdentities(ctx context.Context, issuer string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT subject
		FROM pending_identities
		WHERE issuer = ?
		ORDER BY subject
	`, issuer)
	if err != nil {
		return nil, err
	}
	defer rows.Close() // nolint:errcheck

	var subjects []string
	for rows.Next() {
		var subject string
		if err := rows.Scan(&subject); err != nil {
			return nil, err
		}
		subjects = append(subjects, subject)
	}

	return subjects, rows.Err()
}

// ApproveIdentity links a pending verified subject to an enabled account and consumes its request atomically.
func (s *Store) ApproveIdentity(ctx context.Context, link IdentityLink) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO identity_links (issuer, subject, username)
		SELECT p.issuer, p.subject, u.username
		FROM pending_identities p
		CROSS JOIN local_users u
		WHERE p.issuer = ?
		  AND p.subject = ?
		  AND u.username = ?
		  AND u.enabled = 1
	`, link.Issuer, link.Subject, link.Username)
	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrNotFound
	}

	return err
}
