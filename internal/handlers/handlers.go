// Package handlers wires HTTP routes to the database and auth layers: page
// rendering (login, main app shell), the JSON API consumed by the frontend,
// and the health check used by the container orchestrator.
package handlers

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/steled/budget-book/internal/auth"
	"github.com/steled/budget-book/internal/database"
)

// Handlers holds the dependencies shared by all HTTP handlers.
type Handlers struct {
	store     *database.Store
	auth      *auth.Auth
	templates map[string]*template.Template
	version   string
	logger    *slog.Logger

	loginLimiter *loginLimiter
}

// New parses the embedded templates and returns a ready-to-register
// Handlers. Each page is parsed as its own base.html+page combination,
// since both login.html and index.html define a "content" block under the
// same name — parsing them together would let one silently override the
// other.
func New(store *database.Store, a *auth.Auth, templateFS fs.FS, version string, logger *slog.Logger) (*Handlers, error) {
	templates := make(map[string]*template.Template)
	for _, page := range []string{"login.html", "index.html"} {
		tmpl, err := template.ParseFS(templateFS, "templates/base.html", "templates/"+page)
		if err != nil {
			return nil, err
		}
		templates[page] = tmpl
	}

	return &Handlers{
		store:        store,
		auth:         a,
		templates:    templates,
		version:      version,
		logger:       logger,
		loginLimiter: newLoginLimiter(),
	}, nil
}

// Register attaches every route to mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.Healthz)

	mux.HandleFunc("GET /login", h.LoginGET)
	mux.HandleFunc("POST /login", h.LoginPOST)
	mux.HandleFunc("POST /logout", h.auth.RequireAuth(h.Logout))

	mux.HandleFunc("GET /{$}", h.auth.RequireAuth(h.Index))

	mux.HandleFunc("GET /api/overview", h.auth.RequireAuth(h.GetOverview))
	mux.HandleFunc("GET /api/month", h.auth.RequireAuth(h.GetMonth))

	mux.HandleFunc("GET /api/accounts", h.auth.RequireAuth(h.ListAccounts))
	mux.HandleFunc("POST /api/accounts", h.auth.RequireAuth(h.CreateAccount))
	mux.HandleFunc("PUT /api/accounts/{id}", h.auth.RequireAuth(h.UpdateAccount))
	mux.HandleFunc("DELETE /api/accounts/{id}", h.auth.RequireAuth(h.DeleteAccount))

	mux.HandleFunc("GET /api/categories", h.auth.RequireAuth(h.ListCategories))
	mux.HandleFunc("POST /api/categories", h.auth.RequireAuth(h.CreateCategory))
	mux.HandleFunc("PUT /api/categories/{id}", h.auth.RequireAuth(h.UpdateCategory))
	mux.HandleFunc("DELETE /api/categories/{id}", h.auth.RequireAuth(h.DeleteCategory))

	mux.HandleFunc("GET /api/transactions", h.auth.RequireAuth(h.ListTransactions))
	mux.HandleFunc("POST /api/transactions", h.auth.RequireAuth(h.CreateTransaction))
	mux.HandleFunc("PUT /api/transactions/{id}", h.auth.RequireAuth(h.UpdateTransaction))
	mux.HandleFunc("DELETE /api/transactions/{id}", h.auth.RequireAuth(h.DeleteTransaction))

	mux.HandleFunc("GET /api/recurring", h.auth.RequireAuth(h.ListRecurringTemplates))
	mux.HandleFunc("POST /api/recurring", h.auth.RequireAuth(h.CreateRecurringTemplate))
	mux.HandleFunc("PUT /api/recurring/{id}", h.auth.RequireAuth(h.UpdateRecurringTemplate))
	mux.HandleFunc("DELETE /api/recurring/{id}", h.auth.RequireAuth(h.DeleteRecurringTemplate))
}

// Healthz reports liveness for the container HEALTHCHECK / orchestrator
// probes. It does not require authentication.
func (h *Handlers) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type pageData struct {
	Version  string
	Error    string
	LoggedIn bool
}

// Index renders the single-page app shell (Übersicht + Monat tabs); the
// frontend JS fetches its data from the JSON API.
func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, "index.html", pageData{Version: h.version, LoggedIn: true})
}

// LoginGET renders the login form, redirecting to / if already
// authenticated.
func (h *Handlers) LoginGET(w http.ResponseWriter, r *http.Request) {
	if h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.render(w, "login.html", pageData{Version: h.version})
}

// LoginPOST validates credentials (rate-limited per client IP) and, on
// success, issues a session cookie.
func (h *Handlers) LoginPOST(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !h.loginLimiter.Allow(ip) {
		h.render(w, "login.html", pageData{
			Version: h.version,
			Error:   "Zu viele Anmeldeversuche. Bitte später erneut versuchen.",
		})
		return
	}

	if err := r.ParseForm(); err != nil {
		h.render(w, "login.html", pageData{Version: h.version, Error: "Ungültige Anfrage."})
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if !h.auth.Validate(username, password) {
		h.loginLimiter.RecordFailure(ip)
		h.logger.Warn("failed login attempt", "ip", ip)
		h.render(w, "login.html", pageData{
			Version: h.version,
			Error:   "Benutzername oder Passwort ist falsch.",
		})
		return
	}

	h.loginLimiter.Reset(ip)
	h.auth.SetSessionCookie(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout clears the session cookie.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.auth.ClearSessionCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handlers) render(w http.ResponseWriter, page string, data pageData) {
	tmpl, ok := h.templates[page]
	if !ok {
		h.logger.Error("unknown template page", "page", page)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		h.logger.Error("template render failed", "page", page, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	return r.RemoteAddr
}

const (
	maxLoginAttempts   = 5
	loginAttemptWindow = 15 * time.Minute
)

type attemptRecord struct {
	count     int
	firstSeen time.Time
}

// loginLimiter is a simple in-memory per-IP rate limiter for login attempts.
// Being in-memory is acceptable for a single-instance, single-user app.
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptRecord
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: make(map[string]*attemptRecord)}
}

// Allow reports whether ip is still permitted to attempt a login.
func (l *loginLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	rec, ok := l.attempts[ip]
	if !ok {
		return true
	}
	if time.Since(rec.firstSeen) > loginAttemptWindow {
		delete(l.attempts, ip)
		return true
	}
	return rec.count < maxLoginAttempts
}

// RecordFailure counts one more failed attempt from ip.
func (l *loginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	rec, ok := l.attempts[ip]
	if !ok || time.Since(rec.firstSeen) > loginAttemptWindow {
		l.attempts[ip] = &attemptRecord{count: 1, firstSeen: time.Now()}
		return
	}
	rec.count++
}

// Reset clears any recorded failures for ip after a successful login.
func (l *loginLimiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}
