package db

import "context"

// Theme returns the account’s saved theme or an empty name for its first visit.
func (s *Store) Theme(ctx context.Context, name string) (string, error) {
	var title string
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE((
		    SELECT theme
		    FROM user_themes
		    WHERE username = ?
		), '')
	`, name).Scan(&title)

	return title, err
}

// SaveTheme updates the appearance preference for one local account.
func (s *Store) SaveTheme(ctx context.Context, name, title string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_themes (username, theme)
		VALUES (?, ?)
		ON CONFLICT (username) DO UPDATE
		    SET theme = excluded.theme
	`, name, title)

	return err
}
