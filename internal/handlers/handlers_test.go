package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/steled/budget-book/internal/auth"
	"github.com/steled/budget-book/internal/database"
)

const (
	testUsername = "alice"
	testPassword = "s3cret-password"
	testSecret   = "0123456789abcdef0123456789abcdef"
)

type testEnv struct {
	mux   *http.ServeMux
	store *database.Store
	auth  *auth.Auth
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	store, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	a, err := auth.New(testUsername, testPassword, testSecret, false)
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	templateFS := os.DirFS(filepath.Join("..", ".."))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	h, err := New(store, a, templateFS, "test", logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	mux := http.NewServeMux()
	h.Register(mux)

	return &testEnv{mux: mux, store: store, auth: a}
}

func (e *testEnv) authCookie(t *testing.T) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	e.auth.SetSessionCookie(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Result().Cookies()[0]
}

func (e *testEnv) do(t *testing.T, method, path string, body any, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if authed {
		req.AddCookie(e.authCookie(t))
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode JSON response %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestHealthzNoAuthRequired(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/healthz", nil, false)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestIndexRedirectsUnauthenticated(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/", nil, false)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestIndexRendersWhenAuthenticated(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/", nil, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Budget Book") {
		t.Error("index page body does not mention Budget Book")
	}
}

func TestAPIRouteRejectsUnauthenticatedWithJSON(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/api/accounts", nil, false)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestLoginPOSTSuccess(t *testing.T) {
	env := newTestEnv(t)
	form := "username=" + testUsername + "&password=" + testPassword
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if len(rec.Result().Cookies()) != 1 {
		t.Fatal("expected a session cookie to be set")
	}
}

func TestLoginPOSTWrongPassword(t *testing.T) {
	env := newTestEnv(t)
	form := "username=" + testUsername + "&password=wrong"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (re-rendered login form)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "falsch") {
		t.Error("expected error message in response body")
	}
}

func TestLoginRateLimiting(t *testing.T) {
	env := newTestEnv(t)
	form := "username=" + testUsername + "&password=wrong"

	for i := 0; i < maxLoginAttempts; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "203.0.113.1:12345"
		rec := httptest.NewRecorder()
		env.mux.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "203.0.113.1:12345"
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "Zu viele") {
		t.Error("expected rate-limit message after exceeding max attempts")
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodPost, "/logout", nil, true)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Error("expected logout to clear the session cookie")
	}
}
