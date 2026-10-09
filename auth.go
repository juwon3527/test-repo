package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie = "tb_session"
	sessionTTL    = 12 * time.Hour
)

type Account struct {
	Email        string
	Name         string
	passwordHash [sha256.Size]byte
}

type Session struct {
	Email   string
	Name    string
	Expires time.Time
}

type Auth struct {
	salt     []byte
	accounts map[string]Account

	mu       sync.Mutex
	sessions map[string]Session
}

// newAuth creates the demo account store. Credentials come from
// THE_BRIDGE_EMAIL and THE_BRIDGE_PASSWORD, falling back to demo values.
func newAuth() *Auth {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	a := &Auth{salt: salt, accounts: map[string]Account{}, sessions: map[string]Session{}}
	a.addAccount(envOr("THE_BRIDGE_EMAIL", "fan@thebridge.test"), "Blue Fan", envOr("THE_BRIDGE_PASSWORD", "bluesince1905"))
	return a
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (a *Auth) hash(password string) [sha256.Size]byte {
	return sha256.Sum256(append(append([]byte{}, a.salt...), password...))
}

func (a *Auth) addAccount(email, name, password string) {
	email = strings.ToLower(strings.TrimSpace(email))
	a.accounts[email] = Account{Email: email, Name: name, passwordHash: a.hash(password)}
}

func (a *Auth) verify(email, password string) (Account, bool) {
	account, ok := a.accounts[strings.ToLower(strings.TrimSpace(email))]
	// Hash even for unknown emails so response timing doesn't reveal which accounts exist.
	got := a.hash(password)
	if !ok {
		return Account{}, false
	}
	return account, subtle.ConstantTimeCompare(got[:], account.passwordHash[:]) == 1
}

func (a *Auth) createSession(account Account) (string, Session) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	session := Session{Email: account.Email, Name: account.Name, Expires: time.Now().Add(sessionTTL)}
	a.mu.Lock()
	defer a.mu.Unlock()
	for t, s := range a.sessions {
		if time.Now().After(s.Expires) {
			delete(a.sessions, t)
		}
	}
	a.sessions[token] = session
	return token, session
}

func (a *Auth) session(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return Session{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	session, ok := a.sessions[cookie.Value]
	if !ok || time.Now().After(session.Expires) {
		delete(a.sessions, cookie.Value)
		return Session{}, false
	}
	return session, true
}

func (a *Auth) endSession(r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
	}
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type sessionResponse struct {
	Authenticated bool   `json:"authenticated"`
	Email         string `json:"email,omitempty"`
	Name          string `json:"name,omitempty"`
}

func (a *Auth) register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" || body.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Enter your email and password."})
			return
		}
		account, ok := a.verify(body.Email, body.Password)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "That email and password don't match."})
			return
		}
		token, session := a.createSession(account)
		setSessionCookie(w, r, token, int(sessionTTL.Seconds()))
		writeJSON(w, http.StatusOK, sessionResponse{Authenticated: true, Email: session.Email, Name: session.Name})
	})
	mux.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		a.endSession(r)
		setSessionCookie(w, r, "", -1)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		session, ok := a.session(r)
		if !ok {
			writeJSON(w, http.StatusOK, sessionResponse{})
			return
		}
		writeJSON(w, http.StatusOK, sessionResponse{Authenticated: true, Email: session.Email, Name: session.Name})
	})
}
