package database

import (
	"database/sql"
	"errors"
	"sort"
	"time"
)

// ErrInvalidExtraPayment is returned by AddExtraPayment when the template is
// not an installment plan or the amount is zero or exceeds the open balance.
var ErrInvalidExtraPayment = errors.New("invalid extra payment")

// RecurringInterval is how often a recurring template repeats.
type RecurringInterval string

const (
	IntervalWeekly  RecurringInterval = "weekly"
	IntervalMonthly RecurringInterval = "monthly"
	IntervalYearly  RecurringInterval = "yearly"
)

// RecurringTemplate describes a booking that should be created
// automatically on a schedule.
type RecurringTemplate struct {
	ID          int64
	Name        string
	AmountCents int64
	CategoryID  int64
	AccountID   int64
	Interval    RecurringInterval
	NextDueDate time.Time
	Active      bool
	// RemainingCount is the number of bookings still to be made; nil means
	// unlimited. The template deactivates itself when it reaches zero.
	RemainingCount *int
	// CreditCents is the part of extra payments not yet absorbed by whole
	// installments; it reduces the final installment (always < AmountCents).
	CreditCents int64
	CreatedAt   time.Time
}

// OutstandingCents is the amount still to be paid on a limited template;
// ok is false for unlimited templates.
func (rt RecurringTemplate) OutstandingCents() (cents int64, ok bool) {
	if rt.RemainingCount == nil {
		return 0, false
	}
	return int64(*rt.RemainingCount)*rt.AmountCents - rt.CreditCents, true
}

// NextAmountCents is the amount of the next booking: the last installment of
// a limited template is reduced by any extra-payment credit.
func (rt RecurringTemplate) NextAmountCents() int64 {
	if rt.RemainingCount != nil && *rt.RemainingCount == 1 && rt.CreditCents < rt.AmountCents {
		return rt.AmountCents - rt.CreditCents
	}
	return rt.AmountCents
}

// Advance returns the next due date after the current one, per the
// template's interval.
func (rt RecurringTemplate) Advance() time.Time {
	switch rt.Interval {
	case IntervalWeekly:
		return rt.NextDueDate.AddDate(0, 0, 7)
	case IntervalYearly:
		return rt.NextDueDate.AddDate(1, 0, 0)
	default:
		return rt.NextDueDate.AddDate(0, 1, 0)
	}
}

func scanRecurringTemplate(row interface{ Scan(...any) error }) (RecurringTemplate, error) {
	var rt RecurringTemplate
	var interval, nextDue string
	var active int
	var remaining sql.NullInt64
	err := row.Scan(
		&rt.ID, &rt.Name, &rt.AmountCents, &rt.CategoryID, &rt.AccountID,
		&interval, &nextDue, &active, &remaining, &rt.CreditCents, &rt.CreatedAt,
	)
	if err != nil {
		return RecurringTemplate{}, err
	}
	rt.Interval = RecurringInterval(interval)
	rt.Active = active != 0
	if remaining.Valid {
		n := int(remaining.Int64)
		rt.RemainingCount = &n
	}
	rt.NextDueDate, err = time.Parse(dateLayout, nextDue)
	return rt, err
}

const recurringTemplateQuery = `
	SELECT id, name, amount_cents, category_id, account_id, interval, next_due_date, active, remaining_count, credit_cents, created_at
	FROM recurring_templates
`

// ListRecurringTemplates returns all templates, active first.
func (s *Store) ListRecurringTemplates() ([]RecurringTemplate, error) {
	rows, err := s.db.Query(recurringTemplateQuery + ` ORDER BY active DESC, next_due_date, id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []RecurringTemplate
	for rows.Next() {
		rt, err := scanRecurringTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, rows.Err()
}

// ListActiveDueRecurringTemplates returns all active templates whose
// next_due_date is on or before asOf, used by the scheduler to find work.
func (s *Store) ListActiveDueRecurringTemplates(asOf time.Time) ([]RecurringTemplate, error) {
	rows, err := s.db.Query(
		recurringTemplateQuery+` WHERE active = 1 AND next_due_date <= ? ORDER BY next_due_date, id`,
		asOf.Format(dateLayout),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []RecurringTemplate
	for rows.Next() {
		rt, err := scanRecurringTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, rows.Err()
}

// GetRecurringTemplate fetches a single template by ID.
func (s *Store) GetRecurringTemplate(id int64) (RecurringTemplate, error) {
	row := s.db.QueryRow(recurringTemplateQuery+` WHERE id = ?`, id)
	return scanRecurringTemplate(row)
}

// CreateRecurringTemplate inserts a new recurring template.
func (s *Store) CreateRecurringTemplate(name string, amountCents int64, categoryID, accountID int64, interval RecurringInterval, nextDueDate time.Time, active bool, remainingCount *int) (RecurringTemplate, error) {
	res, err := s.db.Exec(
		`INSERT INTO recurring_templates (name, amount_cents, category_id, account_id, interval, next_due_date, active, remaining_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		name, amountCents, categoryID, accountID, string(interval), nextDueDate.Format(dateLayout), boolToInt(active), remainingCount,
	)
	if err != nil {
		return RecurringTemplate{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return RecurringTemplate{}, err
	}
	return s.GetRecurringTemplate(id)
}

// UpdateRecurringTemplate updates all editable fields of a template.
func (s *Store) UpdateRecurringTemplate(id int64, name string, amountCents int64, categoryID, accountID int64, interval RecurringInterval, nextDueDate time.Time, active bool, remainingCount *int) (RecurringTemplate, error) {
	if _, err := s.db.Exec(
		`UPDATE recurring_templates
		 SET name = ?, amount_cents = ?, category_id = ?, account_id = ?, interval = ?, next_due_date = ?, active = ?, remaining_count = ?,
		     credit_cents = CASE WHEN ? IS NULL OR credit_cents >= ? THEN 0 ELSE credit_cents END
		 WHERE id = ?`,
		name, amountCents, categoryID, accountID, string(interval), nextDueDate.Format(dateLayout), boolToInt(active), remainingCount,
		remainingCount, amountCents, id,
	); err != nil {
		return RecurringTemplate{}, err
	}
	return s.GetRecurringTemplate(id)
}

// SetRecurringTemplateNextDueDate advances a template's next_due_date, used
// by the scheduler after booking an occurrence.
func (s *Store) SetRecurringTemplateNextDueDate(id int64, nextDueDate time.Time) error {
	_, err := s.db.Exec(
		`UPDATE recurring_templates SET next_due_date = ? WHERE id = ?`,
		nextDueDate.Format(dateLayout), id,
	)
	return err
}

// DeleteRecurringTemplate removes a template. Past transactions it
// generated are unaffected (they are plain transactions once created).
func (s *Store) DeleteRecurringTemplate(id int64) error {
	_, err := s.db.Exec(`DELETE FROM recurring_templates WHERE id = ?`, id)
	return err
}

// ProcessDueRecurringTemplates books one transaction for every occurrence of
// every active template whose next_due_date is on or before asOf, advancing
// next_due_date by one interval each time. If the app was offline for
// several periods, missed occurrences are caught up one by one in a loop
// until next_due_date is after asOf. It returns the number of transactions
// created.
func (s *Store) ProcessDueRecurringTemplates(asOf time.Time) (int, error) {
	templates, err := s.ListActiveDueRecurringTemplates(asOf)
	if err != nil {
		return 0, err
	}

	created := 0
	for _, rt := range templates {
		for !rt.NextDueDate.After(asOf) {
			if rt.RemainingCount != nil && *rt.RemainingCount <= 0 {
				break
			}
			if err := s.bookRecurringOccurrence(rt); err != nil {
				return created, err
			}
			created++
			rt.NextDueDate = rt.Advance()
			if rt.RemainingCount != nil {
				n := *rt.RemainingCount - 1
				rt.RemainingCount = &n
			}
		}
	}
	return created, nil
}

func (s *Store) bookRecurringOccurrence(rt RecurringTemplate) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		`INSERT INTO transactions (amount_cents, date, description, category_id, account_id, source)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		rt.NextAmountCents(), rt.NextDueDate.Format(dateLayout), rt.Name, rt.CategoryID, rt.AccountID, string(SourceRecurring),
	); err != nil {
		return err
	}

	// A limited template deactivates once its last booking is made.
	nextDue := rt.Advance()
	if rt.RemainingCount == nil {
		if _, err := tx.Exec(
			`UPDATE recurring_templates SET next_due_date = ? WHERE id = ?`,
			nextDue.Format(dateLayout), rt.ID,
		); err != nil {
			return err
		}
	} else if _, err := tx.Exec(
		`UPDATE recurring_templates
		 SET next_due_date = ?, remaining_count = remaining_count - 1,
		     active = CASE WHEN remaining_count - 1 <= 0 THEN 0 ELSE active END
		 WHERE id = ?`,
		nextDue.Format(dateLayout), rt.ID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// AddExtraPayment books an additional payment on a limited (installment)
// template and applies it to the open balance: whole installments are dropped
// from the end of the plan, a remainder reduces the final installment. The
// plan deactivates once it is fully paid off. The transaction uses the
// template's category and account.
func (s *Store) AddExtraPayment(id int64, amountCents int64, date time.Time) (Transaction, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Transaction{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	rt, err := scanRecurringTemplate(tx.QueryRow(recurringTemplateQuery+` WHERE id = ?`, id))
	if err != nil {
		return Transaction{}, err
	}
	outstanding, ok := rt.OutstandingCents()
	if !ok || amountCents <= 0 || amountCents > outstanding || rt.AmountCents <= 0 {
		return Transaction{}, ErrInvalidExtraPayment
	}

	credit := rt.CreditCents + amountCents
	remaining := *rt.RemainingCount - int(credit/rt.AmountCents)
	credit %= rt.AmountCents

	res, err := tx.Exec(
		`INSERT INTO transactions (amount_cents, date, description, category_id, account_id, source)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		amountCents, date.Format(dateLayout), rt.Name+" (Extrazahlung)", rt.CategoryID, rt.AccountID, string(SourceManual),
	)
	if err != nil {
		return Transaction{}, err
	}
	txID, err := res.LastInsertId()
	if err != nil {
		return Transaction{}, err
	}
	if _, err := tx.Exec(
		`UPDATE recurring_templates
		 SET remaining_count = ?, credit_cents = ?, active = CASE WHEN ? <= 0 THEN 0 ELSE active END
		 WHERE id = ?`,
		remaining, credit, remaining, id,
	); err != nil {
		return Transaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return Transaction{}, err
	}
	return Transaction{ID: txID, AmountCents: amountCents, Date: date, Description: rt.Name + " (Extrazahlung)", CategoryID: rt.CategoryID, AccountID: rt.AccountID, Source: SourceManual}, nil
}

// UpcomingOccurrence is a not-yet-booked occurrence of an active template.
type UpcomingOccurrence struct {
	Template    RecurringTemplate
	Date        time.Time
	AmountCents int64
}

// ListUpcomingRecurring returns every unbooked occurrence of an active
// template with a due date on or before until, oldest first.
func (s *Store) ListUpcomingRecurring(until time.Time) ([]UpcomingOccurrence, error) {
	templates, err := s.ListRecurringTemplates()
	if err != nil {
		return nil, err
	}
	var out []UpcomingOccurrence
	for _, rt := range templates {
		if !rt.Active {
			continue
		}
		for !rt.NextDueDate.After(until) {
			if rt.RemainingCount != nil && *rt.RemainingCount <= 0 {
				break
			}
			out = append(out, UpcomingOccurrence{Template: rt, Date: rt.NextDueDate, AmountCents: rt.NextAmountCents()})
			rt.NextDueDate = rt.Advance()
			if rt.RemainingCount != nil {
				n := *rt.RemainingCount - 1
				rt.RemainingCount = &n
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
