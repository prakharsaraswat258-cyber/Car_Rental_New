package handlers

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"saarthi/internal/store"
)

// AuthData feeds login and signup.
type AuthData struct {
	Next   string
	Email  string
	Name   string
	Phone  string
	Role   string
	Shop   string
	City   string
	Error  string
	Errors map[string]string
	Cities []store.City
}

func homeFor(u store.User) string {
	switch u.Role {
	case store.RoleProvider:
		return "/p"
	case store.RoleAdmin:
		return "/admin"
	}
	return "/trips"
}

func (h *Handlers) startSession(w http.ResponseWriter, r *http.Request, u store.User) error {
	tok, exp, err := h.Store.CreateSession(r.Context(), u.ID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: tok, Path: "/", Expires: exp,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
	return nil
}

func (h *Handlers) afterLogin(w http.ResponseWriter, r *http.Request, u store.User) {
	setFlash(w, Toast{Message: "Welcome, " + u.FirstName() + ".", Tone: "good"})
	next := safeNext(r.FormValue("next"))
	if next == "" || (u.Role != store.RoleRenter && (strings.HasPrefix(next, "/checkout") || strings.HasPrefix(next, "/trips"))) {
		next = homeFor(u)
	}
	redirect(w, r, next)
}

// LoginPage shows the login card with demo buttons.
func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	if u := UserFrom(r.Context()); u != nil {
		http.Redirect(w, r, homeFor(*u), http.StatusSeeOther)
		return
	}
	h.render(w, r, http.StatusOK, "login", "Log in · Saarthi", "account", AuthData{Next: safeNext(r.URL.Query().Get("next"))})
}

// Login checks email + password.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pass := r.FormValue("password")
	u, hash, err := h.Store.UserByEmail(r.Context(), email)
	if err == nil {
		err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass))
	} else if errors.Is(err, store.ErrNotFound) {
		bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoO5nH9a0OI6kPrxp6Z2t1aD6v1WjvTo.6"), []byte(pass)) // same timing
	}
	if err != nil {
		h.render(w, r, http.StatusUnauthorized, "login", "Log in · Saarthi", "account", AuthData{
			Next: safeNext(r.FormValue("next")), Email: email,
			Error: "That didn't match. Try again or use a demo account.",
		})
		return
	}
	if err := h.startSession(w, r, u); err != nil {
		h.serverError(w, r, err)
		return
	}
	h.afterLogin(w, r, u)
}

// DemoLogin is the one-tap "Try as Renter / Provider / Admin".
func (h *Handlers) DemoLogin(w http.ResponseWriter, r *http.Request) {
	email := map[string]string{
		store.RoleRenter: store.DemoRenterEmail, store.RoleProvider: store.DemoProviderEmail, store.RoleAdmin: store.DemoAdminEmail,
	}[r.PathValue("role")]
	if email == "" {
		h.notFound(w, r)
		return
	}
	u, _, err := h.Store.UserByEmail(r.Context(), email)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	if err := h.startSession(w, r, u); err != nil {
		h.serverError(w, r, err)
		return
	}
	h.afterLogin(w, r, u)
}

// SignupPage shows the signup form.
func (h *Handlers) SignupPage(w http.ResponseWriter, r *http.Request) {
	cities, _ := h.Store.Cities(r.Context())
	role := r.URL.Query().Get("role")
	if role != store.RoleProvider {
		role = store.RoleRenter
	}
	h.render(w, r, http.StatusOK, "signup", "Sign up · Saarthi", "account", AuthData{Next: safeNext(r.URL.Query().Get("next")), Role: role, Cities: cities})
}

// Signup creates a renter, or a provider with a pending shop.
func (h *Handlers) Signup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cities, _ := h.Store.Cities(ctx)
	d := AuthData{
		Next: safeNext(r.FormValue("next")), Email: strings.TrimSpace(strings.ToLower(r.FormValue("email"))),
		Name: strings.TrimSpace(r.FormValue("name")), Phone: strings.TrimSpace(r.FormValue("phone")),
		Role: r.FormValue("role"), Shop: strings.TrimSpace(r.FormValue("shop")), City: r.FormValue("city"),
		Errors: map[string]string{}, Cities: cities,
	}
	if d.Role != store.RoleProvider {
		d.Role = store.RoleRenter
	}
	pass := r.FormValue("password")
	if len([]rune(d.Name)) < 2 {
		d.Errors["name"] = "Add your name as it appears on your licence."
	}
	if _, err := mail.ParseAddress(d.Email); err != nil || !strings.Contains(d.Email, ".") {
		d.Errors["email"] = "That email doesn't look right."
	}
	if len(pass) < 8 {
		d.Errors["password"] = "Use at least 8 characters."
	}
	var cityID string
	if d.Role == store.RoleProvider {
		if len([]rune(d.Shop)) < 2 {
			d.Errors["shop"] = "Add your shop's name."
		}
		for _, c := range cities {
			if c.Slug == d.City {
				cityID = c.ID
			}
		}
		if cityID == "" {
			d.Errors["city"] = "Pick the city your shop is in."
		}
	}
	if len(d.Errors) > 0 {
		h.render(w, r, http.StatusUnprocessableEntity, "signup", "Sign up · Saarthi", "account", d)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	u, err := h.Store.CreateUser(ctx, d.Role, d.Name, d.Email, d.Phone, string(hash))
	if errors.Is(err, store.ErrEmailTaken) {
		d.Errors["email"] = "That email already has an account. Log in instead."
		h.render(w, r, http.StatusUnprocessableEntity, "signup", "Sign up · Saarthi", "account", d)
		return
	}
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	if d.Role == store.RoleProvider {
		if _, err := h.Store.CreateShop(ctx, u.ID, d.Shop, cityID, "", strings.TrimSpace(r.FormValue("address")), d.Phone); err != nil {
			h.serverError(w, r, err)
			return
		}
	}
	if err := h.startSession(w, r, u); err != nil {
		h.serverError(w, r, err)
		return
	}
	h.afterLogin(w, r, u)
}

// Logout ends the session.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		h.Store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	setFlash(w, Toast{Message: "Logged out. See you on the road."})
	redirect(w, r, "/")
}
