package database

import (
	"time"
)

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
	CreatedAt   time.Time
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
	err := row.Scan(
		&rt.ID, &rt.Name, &rt.AmountCents, &rt.CategoryID, &rt.AccountID,
		&interval, &nextDue, &active, &rt.CreatedAt,
	)
	if err != nil {
		return RecurringTemplate{}, err
	}
	rt.Interval = RecurringInterval(interval)
	rt.Active = active != 0
	rt.NextDueDate, err = time.Parse(dateLayout, nextDue)
	return rt, err
}

const recurringTemplateQuery = `
	SELECT id, name, amount_cents, category_id, account_id, interval, next_due_date, active, created_at
	FROM recurring_templates
`

// ListRecurringTemplates returns all templates, active first.
func (s *Store) ListRecurringTemplates() ([]RecurringTemplate, error) {
	rows, err := s.db.Query(recurringTemplateQuery + ` ORDER BY active DESC, next_due_date, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
	defer rows.Close()

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
func (s *Store) CreateRecurringTemplate(name string, amountCents int64, categoryID, accountID int64, interval RecurringInterval, nextDueDate time.Time, active bool) (RecurringTemplate, error) {
	res, err := s.db.Exec(
		`INSERT INTO recurring_templates (name, amount_cents, category_id, account_id, interval, next_due_date, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		name, amountCents, categoryID, accountID, string(interval), nextDueDate.Format(dateLayout), boolToInt(active),
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
func (s *Store) UpdateRecurringTemplate(id int64, name string, amountCents int64, categoryID, accountID int64, interval RecurringInterval, nextDueDate time.Time, active bool) (RecurringTemplate, error) {
	if _, err := s.db.Exec(
		`UPDATE recurring_templates
		 SET name = ?, amount_cents = ?, category_id = ?, account_id = ?, interval = ?, next_due_date = ?, active = ?
		 WHERE id = ?`,
		name, amountCents, categoryID, accountID, string(interval), nextDueDate.Format(dateLayout), boolToInt(active), id,
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
			if err := s.bookRecurringOccurrence(rt); err != nil {
				return created, err
			}
			created++
			rt.NextDueDate = rt.Advance()
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
		rt.AmountCents, rt.NextDueDate.Format(dateLayout), rt.Name, rt.CategoryID, rt.AccountID, string(SourceRecurring),
	); err != nil {
		return err
	}

	nextDue := rt.Advance()
	if _, err := tx.Exec(
		`UPDATE recurring_templates SET next_due_date = ? WHERE id = ?`,
		nextDue.Format(dateLayout), rt.ID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
