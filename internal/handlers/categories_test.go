package handlers

import (
	"net/http"
	"testing"
)

func TestCategoriesListIncludesSeededDefaults(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodGet, "/api/categories", nil, true)
	categories := decodeJSON[[]categoryDTO](t, rec)
	if len(categories) == 0 {
		t.Fatal("expected seeded default categories")
	}
}

func TestCreateAndUpdateCategory(t *testing.T) {
	env := newTestEnv(t)

	createRec := env.do(t, http.MethodPost, "/api/categories", categoryRequest{
		Name: "Hobby", Color: "#ff00ff", Type: "expense",
	}, true)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	created := decodeJSON[categoryDTO](t, createRec)

	updateRec := env.do(t, http.MethodPut, "/api/categories/"+itoa(created.ID), categoryRequest{
		Name: "Hobbys", Color: "#00ff00", Type: "expense",
	}, true)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateRec.Code, updateRec.Body.String())
	}
	updated := decodeJSON[categoryDTO](t, updateRec)
	if updated.Name != "Hobbys" || updated.Color != "#00ff00" {
		t.Errorf("updated = %+v", updated)
	}
}

func TestCreateCategoryInvalidType(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodPost, "/api/categories", categoryRequest{
		Name: "Hobby", Color: "#ff00ff", Type: "invalid",
	}, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestDeleteCategoryInUseReturnsConflict(t *testing.T) {
	env := newTestEnv(t)

	accountRec := env.do(t, http.MethodPost, "/api/accounts", accountRequest{Name: "Girokonto"}, true)
	account := decodeJSON[accountDTO](t, accountRec)

	catRec := env.do(t, http.MethodPost, "/api/categories", categoryRequest{Name: "Hobby", Color: "#ff00ff", Type: "expense"}, true)
	category := decodeJSON[categoryDTO](t, catRec)

	txRec := env.do(t, http.MethodPost, "/api/transactions", transactionRequest{
		Amount: "5.00", Date: "2026-03-01", CategoryID: category.ID, AccountID: account.ID,
	}, true)
	if txRec.Code != http.StatusCreated {
		t.Fatalf("create transaction status = %d, body = %s", txRec.Code, txRec.Body.String())
	}

	deleteRec := env.do(t, http.MethodDelete, "/api/categories/"+itoa(category.ID), nil, true)
	if deleteRec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", deleteRec.Code)
	}
}
