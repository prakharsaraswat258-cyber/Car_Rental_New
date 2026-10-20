package store

import (
	"context"
	"time"
)

// SessionTTL is how long a login lasts.
const SessionTTL = 30 * 24 * time.Hour

// CreateSession starts a login for userID and returns its token.
func (s *Store) CreateSession(ctx context.Context, userID string) (token string, expires time.Time, err error) {
	token = NewToken()
	expires = s.Now().Add(SessionTTL).UTC().Truncate(time.Second)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO sessions (token, user_id, expires_at) VALUES (?,?,?)`,
		token, userID, ts(expires))
	return token, expires, err
}

// UserBySession returns the user behind a live session token.
func (s *Store) UserBySession(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrNotFound
	}
	return scanUser(s.DB.QueryRowContext(ctx, `
		SELECT u.id, u.role, u.name, u.email, u.phone, u.licence_status, u.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND s.expires_at > ?`, token, ts(s.Now())))
}

// DeleteSession logs a session out.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}
