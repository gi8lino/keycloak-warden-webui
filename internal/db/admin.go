package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SetupRequired reports whether the local administrator has not been created.
func (s *Store) SetupRequired(ctx context.Context) (bool, error) {
	var required bool
	err := s.db.QueryRowContext(ctx, `
		SELECT NOT EXISTS (
		    SELECT 1
		    FROM local_users
		)
	`).Scan(&required)
	return required, err
}

// CreateInitialAdministrator atomically creates the first local administrator.
// A primary-key constraint prevents concurrent setup attempts from creating a second one.
func (s *Store) CreateInitialAdministrator(ctx context.Context, username, passwordHash string) error {
	return s.CreateFounder(ctx, username, passwordHash, nil)
}

// CreateFounder atomically provisions the bootstrap account and its optional external identity.
func (s *Store) CreateFounder(ctx context.Context, username, passwordHash string, link *IdentityLink) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // nolint:errcheck
	result, err := tx.ExecContext(ctx, `
		INSERT INTO local_users (id, username, password_hash)
		VALUES (1, ?, ?)
		ON CONFLICT (id) DO NOTHING
	`, username, passwordHash)
	if err != nil {
		return fmt.Errorf("create initial administrator: %w", err)
	}

	created, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check setup result: %w", err)
	}
	if created == 0 {
		return ErrSetupComplete
	}
	if link != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO identity_links (issuer, subject, username)
			VALUES (?, ?, ?)
		`, link.Issuer, link.Subject, username); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LookupCredential returns the persisted bcrypt hash for the local administrator.
func (s *Store) LookupCredential(ctx context.Context, username string) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, `
		SELECT password_hash
		FROM local_users
		WHERE username = ?
		  AND enabled = 1
	`, username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}

// ChangePassword atomically replaces the administrator's password and revokes that user's local sessions.
func (s *Store) ChangePassword(ctx context.Context, username, previousHash, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin password update: %w", err)
	}
	defer tx.Rollback() // nolint:errcheck

	result, err := tx.ExecContext(ctx, `
		UPDATE local_users
		SET password_hash = ?
		WHERE username = ?
		  AND password_hash = ?
		  AND enabled = 1
	`, passwordHash, username, previousHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrNotFound
	}

	if err := revokeUserSessions(ctx, tx, username); err != nil {
		return fmt.Errorf("revoke administrator sessions: %w", err)
	}
	return tx.Commit()
}
