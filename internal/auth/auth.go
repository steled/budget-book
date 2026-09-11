// Package auth implements single-user authentication via bcrypt password
// verification and HMAC-signed session cookies.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	cookieName      = "bb_session"
	sessionDuration = 7 * 24 * time.Hour
)

type sessionPayload struct {
	Exp int64 `json:"exp"`
}

// Auth holds the single configured user's credentials and the secret used to
// sign session cookies.
type Auth struct {
	username      string
	passwordHash  []byte
	secret        []byte
	secureCookies bool
}

// New returns an Auth ready to validate logins and manage session cookies.
// password is either a bcrypt hash (e.g. generated with
// `htpasswd -bnBC 12 "" '<password>'`, so a plaintext password never has to
// be stored at rest in a Secret) or a plaintext password, which is hashed
// with bcrypt at startup. Either way, login attempts always perform a
// constant-time bcrypt compare.
func New(username, password, secret string, secureCookies bool) (*Auth, error) {
	hash := []byte(password)
	if _, err := bcrypt.Cost(hash); err != nil {
		h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		hash = h
	}
	return &Auth{
		username:      username,
		passwordHash:  hash,
		secret:        []byte(secret),
		secureCookies: secureCookies,
	}, nil
}

// Validate reports whether username/password match the configured user. It
// always runs bcrypt, even for an unknown username, to avoid leaking via
// timing whether the username was correct.
func (a *Auth) Validate(username, password string) bool {
	if username != a.username {
		_ = bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password)) == nil
}

func (a *Auth) sign(data string) string {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SetSessionCookie issues a signed session cookie valid for sessionDuration.
func (a *Auth) SetSessionCookie(w http.ResponseWriter, r *http.Request) {
	payload := sessionPayload{Exp: time.Now().Add(sessionDuration).Unix()}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	value := payloadB64 + "." + a.sign(payloadB64)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.secureCookies || r.TLS != nil,
	})
}

// ClearSessionCookie invalidates any existing session cookie.
func (a *Auth) ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.secureCookies || r.TLS != nil,
	})
}

// IsAuthenticated reports whether the request carries a validly signed,
// unexpired session cookie.
func (a *Auth) IsAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}

	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	payloadB64, sig := parts[0], parts[1]

	if !hmac.Equal([]byte(sig), []byte(a.sign(payloadB64))) {
		return false
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return false
	}
	var payload sessionPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return false
	}

	return time.Now().Unix() < payload.Exp
}

// RequireAuth wraps next so that unauthenticated requests are rejected
// instead of reaching the handler: page routes are redirected to /login,
// /api/ routes get a JSON 401.
func (a *Auth) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.IsAuthenticated(r) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
