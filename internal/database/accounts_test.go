package database

import (
	"errors"
	"testing"
	"time"
)

func TestCreateAndGetAccount(t *testing.T) {
	s := newTestStore(t)

	created, err := s.CreateAccount("Girokonto", "bank")
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if created.Name != "Girokonto" || created.Icon != "bank" {
		t.Errorf("created = %+v, want Name=Girokonto Icon=bank", created)
	}

	got, err := s.GetAccount(created.ID)
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if got != created {
		t.Errorf("GetAccount() = %+v, want %+v", got, created)
	}
}

func TestListAccountsOrderedByPosition(t *testing.T) {
	s := newTestStore(t)
	first, _ := s.CreateAccount("Girokonto", "bank")
	second, _ := s.CreateAccount("Bargeld", "cash")

	accounts, err := s.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	if len(accounts) != 2 || accounts[0].ID != first.ID || accounts[1].ID != second.ID {
		t.Fatalf("ListAccounts() = %+v, want [%d, %d] in order", accounts, first.ID, second.ID)
	}
}

func TestUpdateAccount(t *testing.T) {
	s := newTestStore(t)
	a, _ := s.CreateAccount("Girokonto", "bank")

	updated, err := s.UpdateAccount(a.ID, "Hauptkonto", "wallet")
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	if updated.Name != "Hauptkonto" || updated.Icon != "wallet" {
		t.Errorf("updated = %+v, want Name=Hauptkonto Icon=wallet", updated)
	}
}

func TestDeleteAccountInUseFails(t *testing.T) {
	s := newTestStore(t)
	account, _ := s.CreateAccount("Girokonto", "bank")
	categories, _ := s.ListCategories()

	if _, err := s.CreateTransaction(1000, time.Now(), "Test", categories[0].ID, account.ID, SourceManual); err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}

	err := s.DeleteAccount(account.ID)
	if !errors.Is(err, ErrInUse) {
		t.Errorf("DeleteAccount() error = %v, want ErrInUse", err)
	}
}

func TestDeleteUnusedAccountSucceeds(t *testing.T) {
	s := newTestStore(t)
	account, _ := s.CreateAccount("Girokonto", "bank")

	if err := s.DeleteAccount(account.ID); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}

	accounts, err := s.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("ListAccounts() = %+v, want empty after delete", accounts)
	}
}

func TestListAccountBalances(t *testing.T) {
	s := newTestStore(t)
	account, _ := s.CreateAccount("Girokonto", "bank")
	categories, _ := s.ListCategories()

	var incomeCat, expenseCat Category
	for _, c := range categories {
		if c.Type == CategoryIncome {
			incomeCat = c
		} else if c.Type == CategoryExpense {
			expenseCat = c
		}
	}

	if _, err := s.CreateTransaction(10000, time.Now(), "Gehalt", incomeCat.ID, account.ID, SourceManual); err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}
	if _, err := s.CreateTransaction(3000, time.Now(), "Einkauf", expenseCat.ID, account.ID, SourceManual); err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}

	balances, err := s.ListAccountBalances()
	if err != nil {
		t.Fatalf("ListAccountBalances() error = %v", err)
	}
	if len(balances) != 1 {
		t.Fatalf("got %d balances, want 1", len(balances))
	}
	if want := int64(7000); balances[0].BalanceCents != want {
		t.Errorf("BalanceCents = %d, want %d", balances[0].BalanceCents, want)
	}

	total, err := s.TotalBalanceCents()
	if err != nil {
		t.Fatalf("TotalBalanceCents() error = %v", err)
	}
	if total != 7000 {
		t.Errorf("TotalBalanceCents() = %d, want 7000", total)
	}
}
