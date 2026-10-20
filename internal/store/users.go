package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Roles.
const (
	RoleRenter   = "renter"
	RoleProvider = "provider"
	RoleAdmin    = "admin"
)

// User is a person who can log in.
type User struct {
	ID            string
	Role          string
	Name          string
	Email         string
	Phone         string
	LicenceStatus string
	CreatedAt     time.Time
}

// FirstName is the part of Name before the first space.
func (u *User) FirstName() string {
	for i, r := range u.Name {
		if r == ' ' {
			return u.Name[:i]
		}
	}
	return u.Name
}

// IsRenter, IsProvider and IsAdmin are template helpers.
func (u *User) IsRenter() bool   { return u != nil && u.Role == RoleRenter }
func (u *User) IsProvider() bool { return u != nil && u.Role == RoleProvider }
func (u *User) IsAdmin() bool    { return u != nil && u.Role == RoleAdmin }

const userCols = `id, role, name, email, phone, licence_status, created_at`

func scanUser(row interface{ Scan(...any) error }, extra ...any) (User, error) {
	var u User
	var created string
	dest := append([]any{&u.ID, &u.Role, &u.Name, &u.Email, &u.Phone, &u.LicenceStatus, &created}, extra...)
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	var err error
	u.CreatedAt, err = parseTS(created)
	return u, err
}

// UserByID loads one user.
func (s *Store) UserByID(ctx context.Context, id string) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// UserByEmail loads a user and their bcrypt hash for login.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, string, error) {
	var hash string
	u, err := scanUser(s.DB.QueryRowContext(ctx,
		`SELECT `+userCols+`, password_hash FROM users WHERE email = ?`, email), &hash)
	return u, hash, err
}

// ErrEmailTaken is returned by CreateUser for a duplicate email.
var ErrEmailTaken = errors.New("email already registered")

// CreateUser inserts a new user. passwordHash must already be bcrypt.
func (s *Store) CreateUser(ctx context.Context, role, name, email, phone, passwordHash string) (User, error) {
	u := User{ID: NewID(), Role: role, Name: name, Email: email, Phone: phone, LicenceStatus: "pending", CreatedAt: s.Now().UTC().Truncate(time.Second)}
	var exists int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&exists); err != nil {
		return User{}, err
	}
	if exists > 0 {
		return User{}, ErrEmailTaken
	}
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO users (id, role, name, email, password_hash, phone, licence_status, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		u.ID, u.Role, u.Name, u.Email, passwordHash, u.Phone, u.LicenceStatus, ts(u.CreatedAt))
	if err != nil {
		return User{}, err
	}
	return u, nil
}
