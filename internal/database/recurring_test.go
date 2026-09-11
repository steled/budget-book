package database

import (
	"testing"
	"time"
)

func TestRecurringTemplateAdvance(t *testing.T) {
	base := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		interval RecurringInterval
		want     time.Time
	}{
		{IntervalWeekly, base.AddDate(0, 0, 7)},
		{IntervalMonthly, base.AddDate(0, 1, 0)},
		{IntervalYearly, base.AddDate(1, 0, 0)},
	}

	for _, tt := range tests {
		rt := RecurringTemplate{NextDueDate: base, Interval: tt.interval}
		if got := rt.Advance(); !got.Equal(tt.want) {
			t.Errorf("Advance() with interval %q = %v, want %v", tt.interval, got, tt.want)
		}
	}
}

func TestCreateAndUpdateRecurringTemplate(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	due := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	created, err := s.CreateRecurringTemplate("Miete", 80000, expense.ID, account.ID, IntervalMonthly, due, true)
	if err != nil {
		t.Fatalf("CreateRecurringTemplate() error = %v", err)
	}
	if created.Name != "Miete" || created.AmountCents != 80000 || !created.Active {
		t.Errorf("created = %+v", created)
	}

	updated, err := s.UpdateRecurringTemplate(created.ID, "Miete neu", 85000, expense.ID, account.ID, IntervalMonthly, due, false)
	if err != nil {
		t.Fatalf("UpdateRecurringTemplate() error = %v", err)
	}
	if updated.Name != "Miete neu" || updated.AmountCents != 85000 || updated.Active {
		t.Errorf("updated = %+v", updated)
	}
}

func TestProcessDueRecurringTemplatesBooksSingleOccurrence(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	due := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	tmpl, err := s.CreateRecurringTemplate("Miete", 80000, expense.ID, account.ID, IntervalMonthly, due, true)
	if err != nil {
		t.Fatalf("CreateRecurringTemplate() error = %v", err)
	}

	created, err := s.ProcessDueRecurringTemplates(asOf)
	if err != nil {
		t.Fatalf("ProcessDueRecurringTemplates() error = %v", err)
	}
	if created != 1 {
		t.Fatalf("created = %d, want 1", created)
	}

	txs, err := s.ListTransactions(0)
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("got %d transactions, want 1", len(txs))
	}
	if txs[0].Source != SourceRecurring || txs[0].AmountCents != 80000 || txs[0].Description != "Miete" {
		t.Errorf("booked transaction = %+v", txs[0])
	}

	updatedTmpl, err := s.GetRecurringTemplate(tmpl.ID)
	if err != nil {
		t.Fatalf("GetRecurringTemplate() error = %v", err)
	}
	wantNextDue := due.AddDate(0, 1, 0)
	if !updatedTmpl.NextDueDate.Equal(wantNextDue) {
		t.Errorf("NextDueDate = %v, want %v", updatedTmpl.NextDueDate, wantNextDue)
	}
}

func TestProcessDueRecurringTemplatesCatchesUpMissedOccurrences(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	// Template was due monthly starting Jan 1st; app comes back online in
	// April, three occurrences behind.
	due := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)

	if _, err := s.CreateRecurringTemplate("Abo", 999, expense.ID, account.ID, IntervalMonthly, due, true); err != nil {
		t.Fatalf("CreateRecurringTemplate() error = %v", err)
	}

	created, err := s.ProcessDueRecurringTemplates(asOf)
	if err != nil {
		t.Fatalf("ProcessDueRecurringTemplates() error = %v", err)
	}
	if created != 4 {
		t.Fatalf("created = %d, want 4 (Jan, Feb, Mar, Apr)", created)
	}

	txs, err := s.ListTransactions(0)
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
	if len(txs) != 4 {
		t.Fatalf("got %d transactions, want 4", len(txs))
	}
}

func TestProcessDueRecurringTemplatesSkipsInactive(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	due := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	if _, err := s.CreateRecurringTemplate("Pausiert", 500, expense.ID, account.ID, IntervalMonthly, due, false); err != nil {
		t.Fatalf("CreateRecurringTemplate() error = %v", err)
	}

	created, err := s.ProcessDueRecurringTemplates(asOf)
	if err != nil {
		t.Fatalf("ProcessDueRecurringTemplates() error = %v", err)
	}
	if created != 0 {
		t.Errorf("created = %d, want 0 for inactive template", created)
	}
}

func TestProcessDueRecurringTemplatesSkipsNotYetDue(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	if _, err := s.CreateRecurringTemplate("Zukunft", 500, expense.ID, account.ID, IntervalMonthly, due, true); err != nil {
		t.Fatalf("CreateRecurringTemplate() error = %v", err)
	}

	created, err := s.ProcessDueRecurringTemplates(asOf)
	if err != nil {
		t.Fatalf("ProcessDueRecurringTemplates() error = %v", err)
	}
	if created != 0 {
		t.Errorf("created = %d, want 0 for not-yet-due template", created)
	}
}

func TestDeleteRecurringTemplate(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	due := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	tmpl, err := s.CreateRecurringTemplate("Miete", 80000, expense.ID, account.ID, IntervalMonthly, due, true)
	if err != nil {
		t.Fatalf("CreateRecurringTemplate() error = %v", err)
	}

	if err := s.DeleteRecurringTemplate(tmpl.ID); err != nil {
		t.Fatalf("DeleteRecurringTemplate() error = %v", err)
	}

	if _, err := s.GetRecurringTemplate(tmpl.ID); err == nil {
		t.Error("GetRecurringTemplate() succeeded after delete, want error")
	}
}
