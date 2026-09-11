package handlers

import (
	"net/http"
	"testing"
)

func TestRecurringTemplatesCRUD(t *testing.T) {
	env := newTestEnv(t)
	account, category := seedAccountAndCategory(t, env)

	createRec := env.do(t, http.MethodPost, "/api/recurring", recurringTemplateRequest{
		Name: "Miete", Amount: "800.00", CategoryID: category.ID, AccountID: account.ID,
		Interval: "monthly", NextDueDate: "2026-04-01", Active: true,
	}, true)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	created := decodeJSON[recurringTemplateDTO](t, createRec)
	if created.AmountCents != 80000 || !created.Active {
		t.Errorf("created = %+v", created)
	}

	updateRec := env.do(t, http.MethodPut, "/api/recurring/"+itoa(created.ID), recurringTemplateRequest{
		Name: "Miete neu", Amount: "850.00", CategoryID: category.ID, AccountID: account.ID,
		Interval: "monthly", NextDueDate: "2026-04-01", Active: false,
	}, true)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateRec.Code, updateRec.Body.String())
	}
	updated := decodeJSON[recurringTemplateDTO](t, updateRec)
	if updated.AmountCents != 85000 || updated.Active {
		t.Errorf("updated = %+v", updated)
	}

	listRec := env.do(t, http.MethodGet, "/api/recurring", nil, true)
	list := decodeJSON[[]recurringTemplateDTO](t, listRec)
	if len(list) != 1 {
		t.Fatalf("got %d templates, want 1", len(list))
	}

	deleteRec := env.do(t, http.MethodDelete, "/api/recurring/"+itoa(created.ID), nil, true)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleteRec.Code)
	}
}

func TestCreateRecurringTemplateRejectsInvalidInterval(t *testing.T) {
	env := newTestEnv(t)
	account, category := seedAccountAndCategory(t, env)

	rec := env.do(t, http.MethodPost, "/api/recurring", recurringTemplateRequest{
		Name: "Miete", Amount: "800.00", CategoryID: category.ID, AccountID: account.ID,
		Interval: "daily", NextDueDate: "2026-04-01", Active: true,
	}, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
