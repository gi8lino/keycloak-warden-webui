package db

import (
	"context"
	"database/sql"
	"errors"
)

// LocalUser describes one account in the administration directory.
type LocalUser struct {
	// Username is the local sign-in identifier.
	Username string
	// Enabled determines whether the account can sign in.
	Enabled bool
	// Founder identifies the bootstrap administrator whose access is protected.
	Founder bool
}

// ErrUsernameTaken indicates that a local sign-in name is already registered.
var ErrUsernameTaken = errors.New("username is already registered")

// initializeUsers imports the legacy singleton credential without replacing its sessions.
func (s *Store) initializeUsers(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // nolint:errcheck
	if _, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS local_users (
		    id INTEGER PRIMARY KEY,
		    username TEXT NOT NULL UNIQUE,
		    password_hash TEXT NOT NULL,
		    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1))
		)
	`); err != nil {
		return err
	}
	var legacy bool
	if err = tx.QueryRowContext(ctx, `
		SELECT EXISTS (
		    SELECT 1
		    FROM sqlite_master
		    WHERE type = 'table'
		      AND name = 'administrators'
		)
	`).Scan(&legacy); err != nil {
		return err
	}

	if legacy {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO local_users (id, username, password_hash)
			SELECT id, username, password_hash
			FROM administrators
		`); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `
			DROP TABLE administrators
		`); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS user_themes (
		    username TEXT PRIMARY KEY REFERENCES local_users(username) ON DELETE CASCADE,
		    theme TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS identity_links (
		    issuer TEXT NOT NULL,
		    subject TEXT NOT NULL,
		    username TEXT NOT NULL REFERENCES local_users(username),
		    PRIMARY KEY (issuer, subject)
		);

		CREATE TABLE IF NOT EXISTS pending_identities (
		    issuer TEXT NOT NULL,
		    subject TEXT NOT NULL,
		    PRIMARY KEY (issuer, subject)
		);

		CREATE TRIGGER IF NOT EXISTS clear_pending_identity
		AFTER INSERT ON identity_links
		BEGIN
		    DELETE FROM pending_identities
		    WHERE issuer = NEW.issuer
		      AND subject = NEW.subject;
		END;

		CREATE TABLE IF NOT EXISTS session_links (
		    token_hash TEXT PRIMARY KEY REFERENCES sessions(token_hash) ON DELETE CASCADE,
		    issuer TEXT NOT NULL,
		    subject TEXT NOT NULL,
		    FOREIGN KEY (issuer, subject) REFERENCES identity_links(issuer, subject) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}
	return tx.Commit()
}

// LocalUsers lists accounts without exposing credential material.
func (s *Store) LocalUsers(ctx context.Context) ([]LocalUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT username, enabled, id = 1
		FROM local_users
		ORDER BY username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var users []LocalUser
	for rows.Next() {
		var u LocalUser
		if err := rows.Scan(&u.Username, &u.Enabled, &u.Founder); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

// CreateLocalUser persists an additional administrator after initial setup.
func (s *Store) CreateLocalUser(ctx context.Context, name, hash string) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO local_users (username, password_hash)
		SELECT ?, ?
		WHERE EXISTS (
		    SELECT 1
		    FROM local_users
		    WHERE id = 1
		)
		ON CONFLICT (username) DO NOTHING
	`, name, hash)
	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrUsernameTaken
	}

	return err
}

// SetLocalAccess updates an additional account and invalidates its sessions when disabled.
func (s *Store) SetLocalAccess(ctx context.Context, actor, name string, enabled bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // nolint:errcheck

	result, err := tx.ExecContext(ctx, `
		UPDATE local_users
		SET enabled = ?
		WHERE username = ?
		  AND id <> 1
		  AND username <> ?
	`, enabled, name, actor)
	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}

	if !enabled {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM sessions
			WHERE username = ?
		`, name); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// revokeUserSessions removes sessions for one local identity in the credential transaction.
func revokeUserSessions(ctx context.Context, tx *sql.Tx, name string) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE is_local = 1
		  AND username = ?
	`, name)
	return err
}
