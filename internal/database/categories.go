package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CategoryType distinguishes income from expense categories.
type CategoryType string

const (
	CategoryIncome  CategoryType = "income"
	CategoryExpense CategoryType = "expense"
)

// Category is a spending/income bucket with a fixed display color.
type Category struct {
	ID        int64
	Name      string
	Color     string
	Type      CategoryType
	Position  int
	CreatedAt time.Time
}

// ErrInUse is returned when a category or account cannot be deleted because
// transactions or recurring templates still reference it.
var ErrInUse = errors.New("still referenced by other records")

// defaultCategories seeds a predefined but user-extensible set of German
// household categories on first run.
var defaultCategories = []Category{
	{Name: "Lebensmittel", Color: "#16a34a", Type: CategoryExpense},
	{Name: "Wohnen", Color: "#0ea5e9", Type: CategoryExpense},
	{Name: "Transport", Color: "#d97706", Type: CategoryExpense},
	{Name: "Freizeit", Color: "#7c3aed", Type: CategoryExpense},
	{Name: "Gesundheit", Color: "#e11d48", Type: CategoryExpense},
	{Name: "Shopping", Color: "#ec4899", Type: CategoryExpense},
	{Name: "Abos", Color: "#6366f1", Type: CategoryExpense},
	{Name: "Sonstiges", Color: "#71717a", Type: CategoryExpense},
	{Name: "Gehalt", Color: "#22c55e", Type: CategoryIncome},
	{Name: "Sonstige Einnahmen", Color: "#0d9488", Type: CategoryIncome},
}

func seedDefaultCategories(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	for i, c := range defaultCategories {
		if _, err := db.Exec(
			`INSERT INTO categories (name, color, type, position) VALUES (?, ?, ?, ?)`,
			c.Name, c.Color, string(c.Type), i,
		); err != nil {
			return err
		}
	}
	return nil
}

// ListCategories returns all categories ordered by position.
func (s *Store) ListCategories() ([]Category, error) {
	rows, err := s.db.Query(
		`SELECT id, name, color, type, position, created_at FROM categories ORDER BY position, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		var typ string
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &typ, &c.Position, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Type = CategoryType(typ)
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

// CreateCategory inserts a new category, placed after all existing ones.
func (s *Store) CreateCategory(name, color string, typ CategoryType) (Category, error) {
	var position int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(position) + 1, 0) FROM categories`).Scan(&position); err != nil {
		return Category{}, err
	}

	res, err := s.db.Exec(
		`INSERT INTO categories (name, color, type, position) VALUES (?, ?, ?, ?)`,
		name, color, string(typ), position,
	)
	if err != nil {
		return Category{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Category{}, err
	}
	return s.GetCategory(id)
}

// GetCategory fetches a single category by ID.
func (s *Store) GetCategory(id int64) (Category, error) {
	var c Category
	var typ string
	err := s.db.QueryRow(
		`SELECT id, name, color, type, position, created_at FROM categories WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Color, &typ, &c.Position, &c.CreatedAt)
	c.Type = CategoryType(typ)
	return c, err
}

// UpdateCategory updates name, color and type of an existing category.
func (s *Store) UpdateCategory(id int64, name, color string, typ CategoryType) (Category, error) {
	if _, err := s.db.Exec(
		`UPDATE categories SET name = ?, color = ?, type = ? WHERE id = ?`,
		name, color, string(typ), id,
	); err != nil {
		return Category{}, err
	}
	return s.GetCategory(id)
}

// DeleteCategory removes a category. Returns ErrInUse if transactions or
// recurring templates still reference it.
func (s *Store) DeleteCategory(id int64) error {
	_, err := s.db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	if isForeignKeyViolation(err) {
		return fmt.Errorf("delete category %d: %w", id, ErrInUse)
	}
	return err
}
