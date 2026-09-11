package database

import (
	"errors"
	"testing"
	"time"
)

func TestCreateAndUpdateCategory(t *testing.T) {
	s := newTestStore(t)

	created, err := s.CreateCategory("Hobby", "#ff0000", CategoryExpense)
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}
	if created.Name != "Hobby" || created.Color != "#ff0000" || created.Type != CategoryExpense {
		t.Errorf("created = %+v, want Name=Hobby Color=#ff0000 Type=expense", created)
	}

	updated, err := s.UpdateCategory(created.ID, "Hobbys", "#00ff00", CategoryExpense)
	if err != nil {
		t.Fatalf("UpdateCategory() error = %v", err)
	}
	if updated.Name != "Hobbys" || updated.Color != "#00ff00" {
		t.Errorf("updated = %+v, want Name=Hobbys Color=#00ff00", updated)
	}
}

func TestNewCategoryAppendedAfterSeeded(t *testing.T) {
	s := newTestStore(t)

	created, err := s.CreateCategory("Hobby", "#ff0000", CategoryExpense)
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}
	if created.Position != len(defaultCategories) {
		t.Errorf("Position = %d, want %d (appended after seeded categories)", created.Position, len(defaultCategories))
	}
}

func TestDeleteCategoryInUseFails(t *testing.T) {
	s := newTestStore(t)
	account, _ := s.CreateAccount("Girokonto", "bank")
	category, _ := s.CreateCategory("Hobby", "#ff0000", CategoryExpense)

	if _, err := s.CreateTransaction(500, time.Now(), "Kino", category.ID, account.ID, SourceManual); err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}

	err := s.DeleteCategory(category.ID)
	if !errors.Is(err, ErrInUse) {
		t.Errorf("DeleteCategory() error = %v, want ErrInUse", err)
	}
}

func TestDeleteUnusedCategorySucceeds(t *testing.T) {
	s := newTestStore(t)
	category, _ := s.CreateCategory("Hobby", "#ff0000", CategoryExpense)

	if err := s.DeleteCategory(category.ID); err != nil {
		t.Fatalf("DeleteCategory() error = %v", err)
	}

	if _, err := s.GetCategory(category.ID); err == nil {
		t.Error("GetCategory() succeeded after delete, want error")
	}
}
