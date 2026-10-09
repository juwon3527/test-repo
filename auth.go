package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookie     = "tb_session"
	sessionTTL        = 7 * 24 * time.Hour
	minPasswordLength = 8
	maxPasswordLength = 72 // bcrypt ignores anything longer
)

// dummyHash is checked when an email isn't registered, so a failed login
// takes the same time whether or not the account exists.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.DefaultCost)

type sessionResponse struct {
	Authenticated bool   `json:"authenticated"`
	Email         string `json:"email,omitempty"`
	Name          string `json:"name,omitempty"`
	AvatarURL     string `json:"avatarUrl,omitempty"`
}

func signedIn(user User) sessionResponse {
	return sessionResponse{Authenticated: true, Email: user.Email, Name: user.Name, AvatarURL: mediaPath(user.AvatarKey)}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	return json.NewDecoder(r.Body).Decode(v) == nil
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) startSession(w http.ResponseWriter, r *http.Request, user User) error {
	buf := make([]byte, 32)
	rand.Read(buf)
	token := base64.RawURLEncoding.EncodeToString(buf)
	if err := a.store.CreateSession(r.Context(), hashToken(token), user.ID, time.Now().Add(sessionTTL)); err != nil {
		return err
	}
	setSessionCookie(w, r, token, int(sessionTTL.Seconds()))
	return nil
}

// currentUser returns the signed-in user for the request, if any.
func (a *App) currentUser(r *http.Request) (User, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return User{}, false
	}
	user, err := a.store.SessionUser(r.Context(), hashToken(cookie.Value), time.Now())
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.Printf("look up session: %v", err)
		}
		return User{}, false
	}
	return user, true
}

func validateSignup(name, email, password string) string {
	switch {
	case name == "" || utf8.RuneCountInString(name) > 80:
		return "Enter a name of up to 80 characters."
	case len(email) > 254:
		return "Enter a valid email address."
	case len(password) < minPasswordLength:
		return "Use a password of at least 8 characters."
	case len(password) > maxPasswordLength:
		return "Use a password of at most 72 characters."
	}
	if address, err := mail.ParseAddress(email); err != nil || address.Address != email {
		return "Enter a valid email address."
	}
	return ""
}

func (a *App) handleSignup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		writeError(w, http.StatusBadRequest, "Fill in your name, email and password.")
		return
	}
	name, email := strings.TrimSpace(body.Name), normalizeEmail(body.Email)
	if problem := validateSignup(name, email, body.Password); problem != "" {
		writeError(w, http.StatusBadRequest, problem)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("hash password: %v", err)
		writeError(w, http.StatusInternalServerError, "Could not create your account. Please try again.")
		return
	}
	user, err := a.store.CreateUser(r.Context(), email, name, hash)
	if errors.Is(err, ErrEmailTaken) {
		writeError(w, http.StatusConflict, "An account with that email already exists. Try signing in.")
		return
	}
	if err == nil {
		err = a.startSession(w, r, user)
	}
	if err != nil {
		log.Printf("sign up: %v", err)
		writeError(w, http.StatusInternalServerError, "Could not create your account. Please try again.")
		return
	}
	writeJSON(w, http.StatusCreated, signedIn(user))
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) || body.Email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "Enter your email and password.")
		return
	}
	user, err := a.store.UserByEmail(r.Context(), normalizeEmail(body.Email))
	if err != nil && !errors.Is(err, ErrNotFound) {
		log.Printf("log in: %v", err)
		writeError(w, http.StatusInternalServerError, "Sign in failed. Please try again.")
		return
	}
	hash := user.PasswordHash
	if err != nil {
		hash = dummyHash
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(body.Password)) != nil || err != nil {
		writeError(w, http.StatusUnauthorized, "That email and password don't match.")
		return
	}
	if err := a.startSession(w, r, user); err != nil {
		log.Printf("start session: %v", err)
		writeError(w, http.StatusInternalServerError, "Sign in failed. Please try again.")
		return
	}
	writeJSON(w, http.StatusOK, signedIn(user))
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := a.store.DeleteSession(r.Context(), hashToken(cookie.Value)); err != nil {
			log.Printf("delete session: %v", err)
		}
	}
	setSessionCookie(w, r, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(r)
	if !ok {
		writeJSON(w, http.StatusOK, sessionResponse{})
		return
	}
	writeJSON(w, http.StatusOK, signedIn(user))
}

// sameOrigin rejects state-changing requests sent from other sites. The
// SameSite cookie already blocks most of these; this is a second layer.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" {
				if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
					writeError(w, http.StatusForbidden, "Cross-site request blocked.")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
