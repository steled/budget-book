package database

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpenRunsMigrationsAndSeedsCategories(t *testing.T) {
	s := newTestStore(t)

	categories, err := s.ListCategories()
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}
	if len(categories) != len(defaultCategories) {
		t.Fatalf("got %d seeded categories, want %d", len(categories), len(defaultCategories))
	}
	for _, c := range categories {
		if c.Color == "" {
			t.Errorf("category %q has no color", c.Name)
		}
		if c.Type != CategoryIncome && c.Type != CategoryExpense {
			t.Errorf("category %q has invalid type %q", c.Name, c.Type)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if _, err := s1.CreateAccount("Girokonto", "bank"); err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	_ = s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer func() { _ = s2.Close() }()

	accounts, err := s2.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts after reopen, want 1", len(accounts))
	}

	categories, err := s2.ListCategories()
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}
	if len(categories) != len(defaultCategories) {
		t.Fatalf("got %d categories after reopen, want %d (no re-seeding)", len(categories), len(defaultCategories))
	}
}
