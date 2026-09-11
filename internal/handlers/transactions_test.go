package handlers

import (
	"net/http"
	"testing"
)

func seedAccountAndCategory(t *testing.T, env *testEnv) (accountDTO, categoryDTO) {
	t.Helper()
	accountRec := env.do(t, http.MethodPost, "/api/accounts", accountRequest{Name: "Girokonto"}, true)
	account := decodeJSON[accountDTO](t, accountRec)

	categories := decodeJSON[[]categoryDTO](t, env.do(t, http.MethodGet, "/api/categories", nil, true))
	return account, categories[0]
}

func TestTransactionsCRUD(t *testing.T) {
	env := newTestEnv(t)
	account, category := seedAccountAndCategory(t, env)

	createRec := env.do(t, http.MethodPost, "/api/transactions", transactionRequest{
		Amount: "42.50", Date: "2026-03-15", Description: "Wocheneinkauf",
		CategoryID: category.ID, AccountID: account.ID,
	}, true)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	created := decodeJSON[transactionDTO](t, createRec)
	if created.AmountCents != 4250 || created.Source != "manual" {
		t.Errorf("created = %+v", created)
	}
	if created.CategoryName != category.Name || created.AccountName != account.Name {
		t.Errorf("expected joined details, got %+v", created)
	}

	updateRec := env.do(t, http.MethodPut, "/api/transactions/"+itoa(created.ID), transactionRequest{
		Amount: "50.00", Date: "2026-03-16", Description: "Korrigiert",
		CategoryID: category.ID, AccountID: account.ID,
	}, true)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateRec.Code, updateRec.Body.String())
	}
	updated := decodeJSON[transactionDTO](t, updateRec)
	if updated.AmountCents != 5000 || updated.Description != "Korrigiert" {
		t.Errorf("updated = %+v", updated)
	}

	listRec := env.do(t, http.MethodGet, "/api/transactions", nil, true)
	list := decodeJSON[[]transactionDTO](t, listRec)
	if len(list) != 1 {
		t.Fatalf("got %d transactions, want 1", len(list))
	}

	deleteRec := env.do(t, http.MethodDelete, "/api/transactions/"+itoa(created.ID), nil, true)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleteRec.Code)
	}
}

func TestCreateTransactionRejectsNegativeAmount(t *testing.T) {
	env := newTestEnv(t)
	account, category := seedAccountAndCategory(t, env)

	rec := env.do(t, http.MethodPost, "/api/transactions", transactionRequest{
		Amount: "-5.00", Date: "2026-03-15", CategoryID: category.ID, AccountID: account.ID,
	}, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateTransactionRejectsMissingReferences(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodPost, "/api/transactions", transactionRequest{
		Amount: "5.00", Date: "2026-03-15",
	}, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateTransactionRejectsInvalidDate(t *testing.T) {
	env := newTestEnv(t)
	account, category := seedAccountAndCategory(t, env)

	rec := env.do(t, http.MethodPost, "/api/transactions", transactionRequest{
		Amount: "5.00", Date: "not-a-date", CategoryID: category.ID, AccountID: account.ID,
	}, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestOverviewReflectsAccountsAndTransactions(t *testing.T) {
	env := newTestEnv(t)
	account, category := seedAccountAndCategory(t, env)

	env.do(t, http.MethodPost, "/api/transactions", transactionRequest{
		Amount: "10.00", Date: "2026-03-15", CategoryID: category.ID, AccountID: account.ID,
	}, true)

	rec := env.do(t, http.MethodGet, "/api/overview", nil, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	overview := decodeJSON[overviewResponse](t, rec)
	if len(overview.Accounts) != 1 || len(overview.Transactions) != 1 {
		t.Errorf("overview = %+v", overview)
	}
}

func TestMonthEndpointDefaultsToCurrentMonth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/api/month", nil, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	month := decodeJSON[monthResponse](t, rec)
	if month.Year == 0 || month.Month == 0 {
		t.Errorf("month = %+v, want non-zero year/month", month)
	}
}

func TestMonthEndpointRejectsInvalidMonth(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/api/month?year=2026&month=13", nil, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
