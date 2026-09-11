package handlers

import (
	"net/http"
	"testing"
)

func TestAccountsCRUD(t *testing.T) {
	env := newTestEnv(t)

	createRec := env.do(t, http.MethodPost, "/api/accounts", accountRequest{Name: "Girokonto", Icon: "🏦"}, true)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	created := decodeJSON[accountDTO](t, createRec)
	if created.Name != "Girokonto" || created.Icon != "🏦" {
		t.Errorf("created = %+v", created)
	}

	listRec := env.do(t, http.MethodGet, "/api/accounts", nil, true)
	list := decodeJSON[[]accountDTO](t, listRec)
	if len(list) != 1 {
		t.Fatalf("got %d accounts, want 1", len(list))
	}

	updateRec := env.do(t, http.MethodPut, "/api/accounts/"+itoa(created.ID), accountRequest{Name: "Hauptkonto", Icon: "💳"}, true)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateRec.Code, updateRec.Body.String())
	}
	updated := decodeJSON[accountDTO](t, updateRec)
	if updated.Name != "Hauptkonto" {
		t.Errorf("updated.Name = %q, want Hauptkonto", updated.Name)
	}

	deleteRec := env.do(t, http.MethodDelete, "/api/accounts/"+itoa(created.ID), nil, true)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleteRec.Code)
	}
}

func TestCreateAccountRequiresName(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodPost, "/api/accounts", accountRequest{Name: "", Icon: "🏦"}, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestDeleteAccountInUseReturnsConflict(t *testing.T) {
	env := newTestEnv(t)

	createRec := env.do(t, http.MethodPost, "/api/accounts", accountRequest{Name: "Girokonto"}, true)
	account := decodeJSON[accountDTO](t, createRec)

	categories := decodeJSON[[]categoryDTO](t, env.do(t, http.MethodGet, "/api/categories", nil, true))

	txRec := env.do(t, http.MethodPost, "/api/transactions", transactionRequest{
		Amount: "12.50", Date: "2026-03-01", Description: "Test",
		CategoryID: categories[0].ID, AccountID: account.ID,
	}, true)
	if txRec.Code != http.StatusCreated {
		t.Fatalf("create transaction status = %d, body = %s", txRec.Code, txRec.Body.String())
	}

	deleteRec := env.do(t, http.MethodDelete, "/api/accounts/"+itoa(account.ID), nil, true)
	if deleteRec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", deleteRec.Code)
	}
}
