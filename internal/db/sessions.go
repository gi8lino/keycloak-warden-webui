package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Session is an authenticated principal bound to an expiry.
type Session struct {
	// Name is the account username in the local directory.
	Name string
	// Local differentiates local sessions from externally verified OIDC identities.
	Local bool
	// Issuer scopes the external identity used at sign-in.
	Issuer string
	// Subject binds the session to its verified external identity.
	Subject string
}

// CreateSession stores only Appkit's purpose-separated token hash.
func (s *Store) CreateSession(ctx context.Context, hash string, user Session, expires time.Time) error {
	// Prune expired records during session creation to keep the store bounded.
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE expires_at <= ?
	`, time.Now().UnixNano()); err != nil {
		return fmt.Errorf("prune expired sessions: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, username, is_local, expires_at)
		VALUES (?, ?, ?, ?)
	`, hash, user.Name, user.Local, expires.UnixNano())
	if err != nil {
		return err
	}
	if !user.Local {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO session_links (token_hash, issuer, subject)
			VALUES (?, ?, ?)
		`, hash, user.Issuer, user.Subject)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LookupSession resolves an unexpired session; local users must still exist.
func (s *Store) LookupSession(ctx context.Context, hash string) (Session, error) {
	var user Session
	var local int
	err := s.db.QueryRowContext(ctx, `
		SELECT
		    s.username,
		    s.is_local,
		    COALESCE(l.issuer, ''),
		    COALESCE(l.subject, '')
		FROM sessions s
		JOIN local_users u
		    ON u.username = s.username
		    AND u.enabled = 1
		LEFT JOIN session_links l
		    ON l.token_hash = s.token_hash
		LEFT JOIN identity_links i
		    ON i.issuer = l.issuer
		    AND i.subject = l.subject
		    AND i.username = s.username
		WHERE s.token_hash = ?
		  AND s.expires_at > ?
		  AND (s.is_local = 1 OR i.username IS NOT NULL)
	`, hash, time.Now().UnixNano()).Scan(&user.Name, &local, &user.Issuer, &user.Subject)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("lookup session: %w", err)
	}
	user.Local = local == 1
	return user, nil
}

// RevokeSession deletes the stored hash, making that bearer token invalid.
func (s *Store) RevokeSession(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE token_hash = ?
	`, hash)
	return err
}
