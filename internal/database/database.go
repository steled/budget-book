// Package database provides the SQLite-backed persistence layer for
// Budget Book: accounts, categories, transactions and recurring
// templates.
//
// Schema changes must go exclusively through migrate() below, using
// CREATE TABLE IF NOT EXISTS for new tables and a columnExists guard before
// ALTER TABLE ADD COLUMN for evolving existing ones (SQLite has no ADD
// COLUMN IF NOT EXISTS). No destructive migrations.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"
)

// Store wraps the SQLite connection and exposes domain repository methods.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path, enables
// foreign key enforcement, and runs all pending migrations.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite allows only one writer at a time; serialize through a single
	// connection to avoid "database is locked" errors under concurrent
	// requests.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT    NOT NULL,
			icon       TEXT    NOT NULL DEFAULT '',
			position   INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS categories (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT    NOT NULL,
			color      TEXT    NOT NULL,
			type       TEXT    NOT NULL CHECK (type IN ('income','expense')),
			position   INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			amount_cents INTEGER NOT NULL CHECK (amount_cents >= 0),
			date         TEXT    NOT NULL,
			description  TEXT    NOT NULL DEFAULT '',
			category_id  INTEGER NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
			account_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
			source       TEXT    NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','recurring')),
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_account ON transactions(account_id)`,
		`CREATE TABLE IF NOT EXISTS recurring_templates (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			name          TEXT    NOT NULL,
			amount_cents  INTEGER NOT NULL CHECK (amount_cents >= 0),
			category_id   INTEGER NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
			account_id    INTEGER NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
			interval      TEXT    NOT NULL CHECK (interval IN ('weekly','monthly','yearly')),
			next_due_date TEXT    NOT NULL,
			active        INTEGER NOT NULL DEFAULT 1,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec migration statement: %w", err)
		}
	}

	if err := seedDefaultCategories(db); err != nil {
		return fmt.Errorf("seed default categories: %w", err)
	}

	return nil
}

// columnExists is kept for future schema evolution: guard an
// ALTER TABLE ... ADD COLUMN with it, since SQLite lacks
// "ADD COLUMN IF NOT EXISTS".
func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf(`SELECT name FROM pragma_table_info(?)`), table)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// isForeignKeyViolation reports whether err is a SQLite foreign key
// constraint violation, as opposed to some other constraint failure (e.g. a
// CHECK constraint). SQLite reports these under different extended result
// codes depending on whether the violation was caused by an insert/update
// (SQLITE_CONSTRAINT_FOREIGNKEY) or a delete blocked by a referencing row
// (SQLITE_CONSTRAINT_TRIGGER, since FK enforcement on delete is implemented
// internally via a trigger) — so the message text is the reliable check.
func isForeignKeyViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return strings.Contains(sqliteErr.Error(), "FOREIGN KEY constraint failed")
}
