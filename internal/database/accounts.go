package database

import (
	"fmt"
	"time"
)

// Account is a place money is held (checking, cash, savings, ...).
type Account struct {
	ID        int64
	Name      string
	Icon      string
	Position  int
	CreatedAt time.Time
}

// AccountBalance is an account together with its current signed balance.
type AccountBalance struct {
	Account
	BalanceCents int64
}

// ListAccounts returns all accounts ordered by position.
func (s *Store) ListAccounts() ([]Account, error) {
	rows, err := s.db.Query(
		`SELECT id, name, icon, position, created_at FROM accounts ORDER BY position, id`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var accounts []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Icon, &a.Position, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// ListAccountBalances returns every account with its balance: the signed
// sum of all transactions booked against it (positive for income
// categories, negative for expense categories).
func (s *Store) ListAccountBalances() ([]AccountBalance, error) {
	rows, err := s.db.Query(`
		SELECT
			a.id, a.name, a.icon, a.position, a.created_at,
			COALESCE(SUM(
				CASE c.type
					WHEN 'income' THEN t.amount_cents
					ELSE -t.amount_cents
				END
			), 0) AS balance_cents
		FROM accounts a
		LEFT JOIN transactions t ON t.account_id = a.id
		LEFT JOIN categories c ON c.id = t.category_id
		GROUP BY a.id
		ORDER BY a.position, a.id
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var balances []AccountBalance
	for rows.Next() {
		var b AccountBalance
		if err := rows.Scan(&b.ID, &b.Name, &b.Icon, &b.Position, &b.CreatedAt, &b.BalanceCents); err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}
	return balances, rows.Err()
}

// TotalBalanceCents returns the sum of all account balances.
func (s *Store) TotalBalanceCents() (int64, error) {
	var total int64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(
			CASE c.type
				WHEN 'income' THEN t.amount_cents
				ELSE -t.amount_cents
			END
		), 0)
		FROM transactions t
		JOIN categories c ON c.id = t.category_id
	`).Scan(&total)
	return total, err
}

// CreateAccount inserts a new account, placed after all existing ones.
func (s *Store) CreateAccount(name, icon string) (Account, error) {
	var position int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(position) + 1, 0) FROM accounts`).Scan(&position); err != nil {
		return Account{}, err
	}

	res, err := s.db.Exec(
		`INSERT INTO accounts (name, icon, position) VALUES (?, ?, ?)`,
		name, icon, position,
	)
	if err != nil {
		return Account{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Account{}, err
	}
	return s.GetAccount(id)
}

// GetAccount fetches a single account by ID.
func (s *Store) GetAccount(id int64) (Account, error) {
	var a Account
	err := s.db.QueryRow(
		`SELECT id, name, icon, position, created_at FROM accounts WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.Icon, &a.Position, &a.CreatedAt)
	return a, err
}

// UpdateAccount updates name and icon of an existing account.
func (s *Store) UpdateAccount(id int64, name, icon string) (Account, error) {
	if _, err := s.db.Exec(
		`UPDATE accounts SET name = ?, icon = ? WHERE id = ?`,
		name, icon, id,
	); err != nil {
		return Account{}, err
	}
	return s.GetAccount(id)
}

// DeleteAccount removes an account. Returns ErrInUse if transactions or
// recurring templates still reference it.
func (s *Store) DeleteAccount(id int64) error {
	_, err := s.db.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	if isForeignKeyViolation(err) {
		return fmt.Errorf("delete account %d: %w", id, ErrInUse)
	}
	return err
}
