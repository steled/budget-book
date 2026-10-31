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

	created, err := s.CreateRecurringTemplate("Miete", 80000, expense.ID, account.ID, IntervalMonthly, due, true, nil)
	if err != nil {
		t.Fatalf("CreateRecurringTemplate() error = %v", err)
	}
	if created.Name != "Miete" || created.AmountCents != 80000 || !created.Active {
		t.Errorf("created = %+v", created)
	}

	updated, err := s.UpdateRecurringTemplate(created.ID, "Miete neu", 85000, expense.ID, account.ID, IntervalMonthly, due, false, nil)
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

	tmpl, err := s.CreateRecurringTemplate("Miete", 80000, expense.ID, account.ID, IntervalMonthly, due, true, nil)
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

	if _, err := s.CreateRecurringTemplate("Abo", 999, expense.ID, account.ID, IntervalMonthly, due, true, nil); err != nil {
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

	if _, err := s.CreateRecurringTemplate("Pausiert", 500, expense.ID, account.ID, IntervalMonthly, due, false, nil); err != nil {
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

	if _, err := s.CreateRecurringTemplate("Zukunft", 500, expense.ID, account.ID, IntervalMonthly, due, true, nil); err != nil {
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

	tmpl, err := s.CreateRecurringTemplate("Miete", 80000, expense.ID, account.ID, IntervalMonthly, due, true, nil)
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

func TestProcessDueRecurringTemplatesStopsAfterRemainingCount(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	due := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	three := 3

	tmpl, err := s.CreateRecurringTemplate("Waschmaschine", 5000, expense.ID, account.ID, IntervalMonthly, due, true, &three)
	if err != nil {
		t.Fatalf("CreateRecurringTemplate() error = %v", err)
	}
	if tmpl.RemainingCount == nil || *tmpl.RemainingCount != 3 {
		t.Fatalf("RemainingCount = %v, want 3", tmpl.RemainingCount)
	}

	created, err := s.ProcessDueRecurringTemplates(asOf)
	if err != nil {
		t.Fatalf("ProcessDueRecurringTemplates() error = %v", err)
	}
	if created != 3 {
		t.Fatalf("created = %d, want 3 (limited by remaining count)", created)
	}

	got, err := s.GetRecurringTemplate(tmpl.ID)
	if err != nil {
		t.Fatalf("GetRecurringTemplate() error = %v", err)
	}
	if got.Active || got.RemainingCount == nil || *got.RemainingCount != 0 {
		t.Errorf("after last booking: active = %v, remaining = %v, want inactive and 0", got.Active, got.RemainingCount)
	}

	again, err := s.ProcessDueRecurringTemplates(asOf)
	if err != nil {
		t.Fatalf("second ProcessDueRecurringTemplates() error = %v", err)
	}
	if again != 0 {
		t.Errorf("second run created %d, want 0", again)
	}
}

func TestProcessDueRecurringTemplatesSingleInstallmentBooksOnce(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	due := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	one := 1

	tmpl, err := s.CreateRecurringTemplate("PayPal später zahlen", 12000, expense.ID, account.ID, IntervalMonthly, due, true, &one)
	if err != nil {
		t.Fatalf("CreateRecurringTemplate() error = %v", err)
	}
	if n, err := s.ProcessDueRecurringTemplates(due); err != nil || n != 1 {
		t.Fatalf("first run = %d, %v, want 1, nil", n, err)
	}
	if n, err := s.ProcessDueRecurringTemplates(due.AddDate(0, 3, 0)); err != nil || n != 0 {
		t.Fatalf("later run = %d, %v, want 0, nil", n, err)
	}
	if got, _ := s.GetRecurringTemplate(tmpl.ID); got.Active {
		t.Error("template still active after its only installment")
	}
}

func TestMigrateAddsRemainingCountToExistingDatabase(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.db.Exec(`ALTER TABLE recurring_templates DROP COLUMN remaining_count`); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate() error = %v", err)
	}
	if err := migrate(s.db); err != nil {
		t.Fatalf("second migrate() error = %v", err)
	}
	if ok, err := columnExists(s.db, "recurring_templates", "remaining_count"); err != nil || !ok {
		t.Errorf("columnExists = %v, %v, want true, nil", ok, err)
	}
}

func TestAddExtraPaymentDropsWholeInstallmentsAndCreditsRemainder(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	due := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	count := 5
	rt, err := s.CreateRecurringTemplate("PayPal", 10000, expense.ID, account.ID, IntervalMonthly, due, true, &count)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.AddExtraPayment(rt.ID, 25000, due); err != nil {
		t.Fatalf("AddExtraPayment() error = %v", err)
	}
	got, _ := s.GetRecurringTemplate(rt.ID)
	if *got.RemainingCount != 3 || got.CreditCents != 5000 || !got.Active {
		t.Fatalf("after extra = count %d credit %d active %v", *got.RemainingCount, got.CreditCents, got.Active)
	}
	if out, _ := got.OutstandingCents(); out != 25000 {
		t.Errorf("outstanding = %d, want 25000", out)
	}

	// Catch up all three remaining installments: 100 + 100 + (100 - 50).
	n, err := s.ProcessDueRecurringTemplates(time.Date(2027, 1, 20, 0, 0, 0, 0, time.UTC))
	if err != nil || n != 3 {
		t.Fatalf("Process = %d, %v", n, err)
	}
	txs, _ := s.ListTransactions(0)
	var sum int64
	for _, tx := range txs {
		sum += tx.AmountCents
	}
	if sum != 50000 { // 250 extra + 100 + 100 + 50
		t.Errorf("total paid = %d, want 50000", sum)
	}
	got, _ = s.GetRecurringTemplate(rt.ID)
	if got.Active {
		t.Error("template should be inactive after the last installment")
	}
}

func TestAddExtraPaymentRejectsInvalid(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	due := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	count := 2
	limited, _ := s.CreateRecurringTemplate("Rate", 10000, expense.ID, account.ID, IntervalMonthly, due, true, &count)
	unlimited, _ := s.CreateRecurringTemplate("Miete", 10000, expense.ID, account.ID, IntervalMonthly, due, true, nil)

	if _, err := s.AddExtraPayment(unlimited.ID, 100, due); err != ErrInvalidExtraPayment {
		t.Errorf("unlimited: err = %v", err)
	}
	if _, err := s.AddExtraPayment(limited.ID, 20001, due); err != ErrInvalidExtraPayment {
		t.Errorf("too high: err = %v", err)
	}
	// Paying off everything deactivates the template.
	if _, err := s.AddExtraPayment(limited.ID, 20000, due); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetRecurringTemplate(limited.ID)
	if got.Active || *got.RemainingCount != 0 {
		t.Errorf("after payoff = %+v", got)
	}
}

func TestListUpcomingRecurring(t *testing.T) {
	s := newTestStore(t)
	account, _, expense := setupAccountAndCategories(t, s)
	count := 2
	_, _ = s.CreateRecurringTemplate("Wöchentlich", 100, expense.ID, account.ID, IntervalWeekly, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), true, nil)
	_, _ = s.CreateRecurringTemplate("Begrenzt", 200, expense.ID, account.ID, IntervalWeekly, time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), true, &count)
	_, _ = s.CreateRecurringTemplate("Inaktiv", 300, expense.ID, account.ID, IntervalMonthly, time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), false, nil)
	_, _ = s.CreateRecurringTemplate("Nächster Monat", 400, expense.ID, account.ID, IntervalMonthly, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), true, nil)

	got, err := s.ListUpcomingRecurring(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	// Begrenzt: 2 + 9; Wöchentlich: 10, 17, 24 → 5 entries.
	if len(got) != 5 {
		t.Fatalf("got %d occurrences, want 5: %+v", len(got), got)
	}
	if got[0].Template.Name != "Begrenzt" || got[1].Template.Name != "Begrenzt" || got[2].Template.Name != "Wöchentlich" {
		t.Errorf("unexpected order: %v %v %v", got[0].Template.Name, got[1].Template.Name, got[2].Template.Name)
	}
}
