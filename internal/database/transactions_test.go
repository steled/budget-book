package database

import (
	"testing"
	"time"
)

func setupAccountAndCategories(t *testing.T, s *Store) (Account, Category, Category) {
	t.Helper()
	account, err := s.CreateAccount("Girokonto", "bank")
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	categories, err := s.ListCategories()
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}
	var income, expense Category
	for _, c := range categories {
		if c.Type == CategoryIncome && income.ID == 0 {
			income = c
		}
		if c.Type == CategoryExpense && expense.ID == 0 {
			expense = c
		}
	}
	return account, income, expense
}

func TestCreateAndGetTransaction(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	date := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	created, err := s.CreateTransaction(1234, date, "Einkauf", expense.ID, account.ID, SourceManual)
	if err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}
	if created.AmountCents != 1234 || created.Description != "Einkauf" || !created.Date.Equal(date) {
		t.Errorf("created = %+v", created)
	}
	if created.Source != SourceManual {
		t.Errorf("Source = %q, want manual", created.Source)
	}

	got, err := s.GetTransaction(created.ID)
	if err != nil {
		t.Fatalf("GetTransaction() error = %v", err)
	}
	if got.ID != created.ID || got.AmountCents != created.AmountCents {
		t.Errorf("GetTransaction() = %+v, want %+v", got, created)
	}
}

func TestUpdateTransactionPreservesSource(t *testing.T) {
	s := newTestStore(t)
	account, income, expense := setupAccountAndCategories(t, s)
	date := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	created, err := s.CreateTransaction(1234, date, "Miete", expense.ID, account.ID, SourceRecurring)
	if err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}

	newDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	updated, err := s.UpdateTransaction(created.ID, 5000, newDate, "Korrigiert", income.ID, account.ID)
	if err != nil {
		t.Fatalf("UpdateTransaction() error = %v", err)
	}
	if updated.AmountCents != 5000 || updated.Description != "Korrigiert" || updated.CategoryID != income.ID {
		t.Errorf("updated = %+v", updated)
	}
	if updated.Source != SourceRecurring {
		t.Errorf("Source = %q, want recurring to be preserved across edits", updated.Source)
	}
}

func TestListTransactionsOrderedNewestFirst(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)

	older := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	oldTx, _ := s.CreateTransaction(100, older, "Alt", expense.ID, account.ID, SourceManual)
	newTx, _ := s.CreateTransaction(200, newer, "Neu", expense.ID, account.ID, SourceManual)

	list, err := s.ListTransactions(0)
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
	if len(list) != 2 || list[0].ID != newTx.ID || list[1].ID != oldTx.ID {
		t.Fatalf("ListTransactions() = %+v, want [%d, %d]", list, newTx.ID, oldTx.ID)
	}
	if list[0].CategoryName != expense.Name || list[0].AccountName != "Girokonto" {
		t.Errorf("joined fields missing: %+v", list[0])
	}
}

func TestListTransactionsRespectsLimit(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	date := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		if _, err := s.CreateTransaction(100, date.AddDate(0, 0, i), "T", expense.ID, account.ID, SourceManual); err != nil {
			t.Fatalf("CreateTransaction() error = %v", err)
		}
	}

	list, err := s.ListTransactions(2)
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d transactions, want 2", len(list))
	}
}

func TestMonthSummaryAndListTransactionsInMonth(t *testing.T) {
	s := newTestStore(t)
	account, income, expense := setupAccountAndCategories(t, s)

	inMonth := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	outOfMonth := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	if _, err := s.CreateTransaction(10000, inMonth, "Gehalt", income.ID, account.ID, SourceManual); err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}
	if _, err := s.CreateTransaction(3000, inMonth, "Miete", expense.ID, account.ID, SourceManual); err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}
	if _, err := s.CreateTransaction(9999, outOfMonth, "Nicht relevant", expense.ID, account.ID, SourceManual); err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}

	incomeCents, expenseCents, err := s.MonthSummary(2026, time.March)
	if err != nil {
		t.Fatalf("MonthSummary() error = %v", err)
	}
	if incomeCents != 10000 || expenseCents != 3000 {
		t.Errorf("MonthSummary() = (%d, %d), want (10000, 3000)", incomeCents, expenseCents)
	}

	inMonthTxs, err := s.ListTransactionsInMonth(2026, time.March)
	if err != nil {
		t.Fatalf("ListTransactionsInMonth() error = %v", err)
	}
	if len(inMonthTxs) != 2 {
		t.Fatalf("got %d transactions in month, want 2", len(inMonthTxs))
	}
}

func TestDeleteTransaction(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)

	created, err := s.CreateTransaction(100, time.Now(), "Test", expense.ID, account.ID, SourceManual)
	if err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}

	if err := s.DeleteTransaction(created.ID); err != nil {
		t.Fatalf("DeleteTransaction() error = %v", err)
	}

	if _, err := s.GetTransaction(created.ID); err == nil {
		t.Error("GetTransaction() succeeded after delete, want error")
	}
}
