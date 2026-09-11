package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func newTestAuth(t *testing.T) *Auth {
	t.Helper()
	a, err := New("alice", "s3cret-password", "0123456789abcdef0123456789abcdef", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return a
}

func TestValidate(t *testing.T) {
	a := newTestAuth(t)

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{"correct credentials", "alice", "s3cret-password", true},
		{"wrong password", "alice", "wrong", false},
		{"wrong username", "bob", "s3cret-password", false},
		{"empty credentials", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.Validate(tt.username, tt.password); got != tt.want {
				t.Errorf("Validate(%q, %q) = %v, want %v", tt.username, tt.password, got, tt.want)
			}
		})
	}
}

func TestSessionCookieRoundTrip(t *testing.T) {
	a := newTestAuth(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	a.SetSessionCookie(rec, req)

	result := rec.Result()
	cookies := result.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	authedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	authedReq.AddCookie(cookies[0])

	if !a.IsAuthenticated(authedReq) {
		t.Error("IsAuthenticated() = false, want true for freshly issued cookie")
	}
}

func TestNewAcceptsPreHashedBcryptPassword(t *testing.T) {
	preHashed, err := bcrypt.GenerateFromPassword([]byte("s3cret-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword() error = %v", err)
	}

	a, err := New("alice", string(preHashed), "0123456789abcdef0123456789abcdef", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if !a.Validate("alice", "s3cret-password") {
		t.Error("Validate() = false for correct password against pre-hashed auth.password, want true")
	}
	if a.Validate("alice", string(preHashed)) {
		t.Error("Validate() = true when logging in with the hash itself, want false")
	}
}

func TestIsAuthenticatedRejectsTamperedCookie(t *testing.T) {
	a := newTestAuth(t)

	rec := httptest.NewRecorder()
	a.SetSessionCookie(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	cookie := rec.Result().Cookies()[0]
	cookie.Value = cookie.Value + "tampered"

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)

	if a.IsAuthenticated(req) {
		t.Error("IsAuthenticated() = true for tampered cookie, want false")
	}
}

func TestIsAuthenticatedRejectsMissingCookie(t *testing.T) {
	a := newTestAuth(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if a.IsAuthenticated(req) {
		t.Error("IsAuthenticated() = true without cookie, want false")
	}
}

func TestClearSessionCookie(t *testing.T) {
	a := newTestAuth(t)

	rec := httptest.NewRecorder()
	a.ClearSessionCookie(rec, httptest.NewRequest(http.MethodGet, "/logout", nil))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative to expire cookie", cookies[0].MaxAge)
	}
}

func TestRequireAuthRedirectsPageRoutes(t *testing.T) {
	a := newTestAuth(t)
	handler := a.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		t.Error("wrapped handler should not be called when unauthenticated")
	})

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestRequireAuthReturnsJSONForAPIRoutes(t *testing.T) {
	a := newTestAuth(t)
	handler := a.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		t.Error("wrapped handler should not be called when unauthenticated")
	})

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/accounts", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuthAllowsAuthenticated(t *testing.T) {
	a := newTestAuth(t)
	called := false
	handler := a.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	rec := httptest.NewRecorder()
	a.SetSessionCookie(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)

	handler(httptest.NewRecorder(), req)

	if !called {
		t.Error("wrapped handler was not called for authenticated request")
	}
}
