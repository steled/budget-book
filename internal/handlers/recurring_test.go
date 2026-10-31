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

func TestRecurringTemplateRemainingCount(t *testing.T) {
	env := newTestEnv(t)
	account, category := seedAccountAndCategory(t, env)
	six := 6

	rec := env.do(t, http.MethodPost, "/api/recurring", recurringTemplateRequest{
		Name: "Sofa", Amount: "50.00", CategoryID: category.ID, AccountID: account.ID,
		Interval: "monthly", NextDueDate: "2026-04-01", Active: true, RemainingCount: &six,
	}, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := decodeJSON[recurringTemplateDTO](t, rec); got.RemainingCount == nil || *got.RemainingCount != 6 {
		t.Errorf("RemainingCount = %v, want 6", got.RemainingCount)
	}

	for _, bad := range []int{0, -1, 601} {
		bad := bad
		rec := env.do(t, http.MethodPost, "/api/recurring", recurringTemplateRequest{
			Name: "Sofa", Amount: "50.00", CategoryID: category.ID, AccountID: account.ID,
			Interval: "monthly", NextDueDate: "2026-04-01", Active: true, RemainingCount: &bad,
		}, true)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("remainingCount %d: status = %d, want 400", bad, rec.Code)
		}
	}
}

func TestExtraPaymentEndpoint(t *testing.T) {
	env := newTestEnv(t)
	account, category := seedAccountAndCategory(t, env)
	count := 4
	rec := env.do(t, http.MethodPost, "/api/recurring", recurringTemplateRequest{
		Name: "PayPal", Amount: "50.00", CategoryID: category.ID, AccountID: account.ID,
		Interval: "monthly", NextDueDate: "2099-01-01", Active: true, RemainingCount: &count,
	}, true)
	created := decodeJSON[recurringTemplateDTO](t, rec)
	if created.OutstandingCents == nil || *created.OutstandingCents != 20000 {
		t.Fatalf("outstanding = %v", created.OutstandingCents)
	}

	path := "/api/recurring/" + itoa(created.ID) + "/extra"
	if r := env.do(t, http.MethodPost, path, extraPaymentRequest{Amount: "70.00", Date: "2026-11-01"}, false); r.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d", r.Code)
	}
	r := env.do(t, http.MethodPost, path, extraPaymentRequest{Amount: "70.00", Date: "2026-11-01"}, true)
	if r.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", r.Code, r.Body.String())
	}
	got := decodeJSON[recurringTemplateDTO](t, r)
	if *got.RemainingCount != 3 || *got.OutstandingCents != 13000 {
		t.Errorf("after extra = %+v", got)
	}

	for name, req := range map[string]extraPaymentRequest{
		"too high":   {Amount: "130.01", Date: "2026-11-01"},
		"zero":       {Amount: "0", Date: "2026-11-01"},
		"bad date":   {Amount: "10", Date: "x"},
		"bad amount": {Amount: "abc", Date: "2026-11-01"},
	} {
		if r := env.do(t, http.MethodPost, path, req, true); r.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, r.Code)
		}
	}
	if r := env.do(t, http.MethodPost, "/api/recurring/9999/extra", extraPaymentRequest{Amount: "1", Date: "2026-11-01"}, true); r.Code != http.StatusNotFound {
		t.Errorf("unknown id status = %d, want 404", r.Code)
	}
}
