package store

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"time"
)

// SessionTTL is how long a login lasts.
const SessionTTL = 30 * 24 * time.Hour

// Session tokens are "nonce.userID.expiry.signature". The sessions table
// stays the source of truth; the signature lets another server instance
// (Vercel runs several, each with its own database) accept a login it has
// no row for, as long as that user exists there too (D-050).
var sessionKey = loadSessionKey()

func loadSessionKey() []byte {
	if k := os.Getenv("SAARTHI_SESSION_KEY"); k != "" {
		return []byte(k)
	}
	if d := os.Getenv("VERCEL_DEPLOYMENT_ID"); d != "" {
		sum := sha256.Sum256([]byte("saarthi-session:" + d))
		return sum[:]
	}
	b := make([]byte, 32) // one process: tokens only ever live in the table
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

// SharedSessions reports whether session tokens are checkable across
// instances (the key comes from the environment, not this process).
func SharedSessions() bool {
	return os.Getenv("SAARTHI_SESSION_KEY") != "" || os.Getenv("VERCEL_DEPLOYMENT_ID") != ""
}

func sign(body string) string {
	m := hmac.New(sha256.New, sessionKey)
	m.Write([]byte(body))
	return hex.EncodeToString(m.Sum(nil))
}

// verifyToken checks a token's signature and returns its user and expiry.
func verifyToken(token string) (userID string, expires time.Time, ok bool) {
	i := strings.LastIndexByte(token, '.')
	if i < 0 || !hmac.Equal([]byte(sign(token[:i])), []byte(token[i+1:])) {
		return "", time.Time{}, false
	}
	parts := strings.Split(token[:i], ".")
	if len(parts) != 3 {
		return "", time.Time{}, false
	}
	unix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}
	return parts[1], time.Unix(unix, 0).UTC(), true
}

// CreateSession starts a login for userID and returns its token.
func (s *Store) CreateSession(ctx context.Context, userID string) (token string, expires time.Time, err error) {
	expires = s.Now().Add(SessionTTL).UTC().Truncate(time.Second)
	body := NewToken() + "." + userID + "." + strconv.FormatInt(expires.Unix(), 10)
	token = body + "." + sign(body)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO sessions (token, user_id, expires_at) VALUES (?,?,?)`,
		token, userID, ts(expires))
	return token, expires, err
}

// UserBySession returns the user behind a live session token.
func (s *Store) UserBySession(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrNotFound
	}
	u, err := scanUser(s.DB.QueryRowContext(ctx, `
		SELECT u.id, u.role, u.name, u.email, u.phone, u.licence_status, u.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND s.expires_at > ?`, token, ts(s.Now())))
	if err != ErrNotFound {
		return u, err
	}
	// A row that exists but failed above is expired or logged out.
	var known int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE token = ?`, token).Scan(&known); err != nil {
		return User{}, err
	}
	userID, expires, ok := verifyToken(token)
	if known > 0 || !ok || !expires.After(s.Now()) {
		return User{}, ErrNotFound
	}
	// A login made on another instance: adopt it here.
	if u, err = s.UserByID(ctx, userID); err != nil {
		return User{}, ErrNotFound
	}
	_, err = s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO sessions (token, user_id, expires_at) VALUES (?,?,?)`,
		token, userID, ts(expires))
	return u, err
}

// DeleteSession logs a session out. The row is kept, expired, so the
// token's signature can't bring it back on this instance.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	gone := ts(time.Unix(0, 0))
	res, err := s.DB.ExecContext(ctx, `UPDATE sessions SET expires_at = ? WHERE token = ?`, gone, token)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	if userID, _, ok := verifyToken(token); ok {
		_, err = s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO sessions (token, user_id, expires_at)
			SELECT ?, id, ? FROM users WHERE id = ?`, token, gone, userID)
	}
	return err
}
